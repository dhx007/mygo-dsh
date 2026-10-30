// Package agent runs the tool-calling loop: stream a reply, execute the
// tools the model asked for, feed results back, until a final answer.
// Mutating tools pass an approval gate first, as the original harness
// asks the user before shell and file writes.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mygo-harness/internal/llm"
	"mygo-harness/internal/tools"
)

const defaultMaxTurns = 30

// Decision is the user's answer to a tool approval request.
type Decision int

const (
	Deny Decision = iota
	AllowOnce
	AllowAlways
)

// TodoItem is one checklist entry of the plan tool.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // pending | in_progress | completed
}

// Callbacks deliver progress. All run on the goroutine calling Run; the
// UI wraps them with Window.Update. Nil hooks are skipped.
type Callbacks struct {
	OnContent   func(delta string)
	OnReasoning func(delta string)
	OnToolStart func(id, name, args string)
	OnToolEnd   func(id, result string, failed bool)
	OnStatus    func(text string)
	// OnApproval gates a mutating tool; nil denies every mutating tool.
	OnApproval func(tool, args string) Decision
	OnTodo     func(items []TodoItem)
	// OnSubagentStart/End bracket a task-tool delegation.
	OnSubagentStart func(description string)
	OnSubagentEnd   func(description, output string)
	// OnAssistantMessage reports each assembled assistant message (one
	// per turn, tool calls included) — the trajectory's source of truth.
	OnAssistantMessage func(m llm.Message)
	OnDone             func(err error)
}

// Agent holds one conversation's history and settings.
type Agent struct {
	Client       *llm.Client
	Model        string
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
	WorkDir      string
	MaxTurns     int
	// Depth is 0 for the top-level loop; task delegates one level deeper.
	Depth int
	// Specs is the preset tool set; todo and task are added dynamically.
	Specs []llm.ToolSpec
	// Shell, when set, runs the bash tool in a persistent shell whose
	// env/cwd survive across calls (the original's minimal preset).
	// Close it when the agent is discarded.
	Shell *tools.PersistentShell
	// PluginTool, when set, mounts the plugins management tool (list /
	// info / enable / disable) — the original only enables it for the
	// creator preset with an editable profile.
	PluginTool func(action, plugin string) (string, bool)
	// SkillTool, when set, mounts the skills tool (list / run) over the
	// discovered SKILL.md packs.
	SkillTool func(action, name string) (string, bool)
	// PackageTools routes installed plugin-package tools (name → runner);
	// MutatingTools marks which of them need the approval gate.
	PackageTools  map[string]func(ctx context.Context, argsJSON string) (string, bool)
	MutatingTools map[string]bool
	// PackageSpecs are the installed packages' tool schemas; mounted at
	// depth 0 and gated per name by PluginEnabled.
	PackageSpecs []llm.ToolSpec
	// PluginEnabled, when set, is asked for every tool spec before each
	// Run so plugins toggled off (by the user or by the plugins tool)
	// disappear from the next message's toolset.
	PluginEnabled func(id string) bool
	// Subagent builds the child agent for the task tool (depth 0 only).
	Subagent func() *Agent

	History []llm.Message
}

var todoSpec = llm.ToolSpec{
	Type: "function",
	Function: llm.ToolDef{
		Name: "todo",
		Description: "Maintain the session's task checklist: call this whenever the plan " +
			"changes or a step is done. One in_progress at a time; mark completed as soon as done.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content": map[string]any{"type": "string"},
							"status":  map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
						},
						"required": []string{"content", "status"},
					},
				},
			},
			"required": []string{"todos"},
		},
	},
}

var taskSpec = llm.ToolSpec{
	Type: "function",
	Function: llm.ToolDef{
		Name: "task",
		Description: "Delegate a self-contained subtask to a subagent with the same tools " +
			"(except task) and report its final answer. Use for focused research or scoping work.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"description": map[string]any{"type": "string", "description": "Short label, shown in the timeline."},
				"prompt":      map[string]any{"type": "string", "description": "Complete, self-contained instruction for the subagent."},
			},
			"required": []string{"description", "prompt"},
		},
	},
}

var skillSpec = llm.ToolSpec{
	Type: "function",
	Function: llm.ToolDef{
		Name: "skill",
		Description: "Use the installed skills (SKILL.md instruction packs): list them " +
			"with their descriptions, or run one to load its full instructions and follow them. " +
			"Run a skill when the task matches its description.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "run"}, "description": "list all skills, or run one by name."},
				"name":   map[string]any{"type": "string", "description": "Skill name; required for run."},
			},
			"required": []string{"action"},
		},
	},
}

var pluginSpec = llm.ToolSpec{
	Type: "function",
	Function: llm.ToolDef{
		Name: "plugins",
		Description: "Manage the harness's plugins: list them with their on/off state, " +
			"inspect one, enable or disable it. Disabling a tool plugin removes its tool " +
			"from your toolset in the next turn — be careful, especially with bash; the " +
			"user can always re-enable it in the plugins page.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "info", "enable", "disable"}, "description": "What to do."},
				"plugin": map[string]any{"type": "string", "description": "Plugin id; required for info/enable/disable."},
			},
			"required": []string{"action"},
		},
	},
}

// toolSpecs assembles the toolset for one Run: the preset specs (minus
// plugins toggled off), the todo checklist, and — depth 0 only — the task
// delegator and the plugins manager when wired.
func (a *Agent) toolSpecs() []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(a.Specs)+3)
	for _, sp := range a.Specs {
		if a.PluginEnabled != nil && !a.PluginEnabled(sp.Function.Name) {
			continue
		}
		specs = append(specs, sp)
	}
	specs = append(specs, todoSpec)
	if a.Depth == 0 && a.Subagent != nil {
		specs = append(specs, taskSpec)
	}
	if a.Depth == 0 && a.PluginTool != nil && (a.PluginEnabled == nil || a.PluginEnabled("plugins")) {
		specs = append(specs, pluginSpec)
	}
	if a.Depth == 0 && a.SkillTool != nil && (a.PluginEnabled == nil || a.PluginEnabled("skill")) {
		specs = append(specs, skillSpec)
	}
	if a.Depth == 0 {
		for _, sp := range a.PackageSpecs {
			if a.PluginEnabled != nil && !a.PluginEnabled(sp.Function.Name) {
				continue
			}
			specs = append(specs, sp)
		}
	}
	return specs
}

// Run processes one user message to completion. The returned error is
// also passed to OnDone.
func (a *Agent) Run(ctx context.Context, userText string, cb Callbacks) error {
	return a.RunMessage(ctx, llm.Message{Role: "user", Content: userText}, cb)
}

// RunMessage is Run with a prepared user message (attachments arrive as
// multimodal Parts, REQ-011).
func (a *Agent) RunMessage(ctx context.Context, user llm.Message, cb Callbacks) error {
	a.History = append(a.History, user)
	maxTurns := a.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	specs := a.toolSpecs()
	if a.Shell != nil {
		for i, sp := range specs {
			if sp.Function.Name == "bash" {
				sp.Function.Description += " This shell is persistent: the working " +
					"directory, exported variables and shell options persist across calls."
				specs[i] = sp
			}
		}
	}

	var runErr error
	answered := false
	for turn := 0; turn < maxTurns && !answered; turn++ {
		if err := ctx.Err(); err != nil {
			runErr = errors.New("已停止")
			break
		}
		if cb.OnStatus != nil {
			cb.OnStatus("思考中…")
		}
		req := llm.Request{
			Model:       a.Model,
			Messages:    a.requestMessages(),
			Tools:       a.toolSpecs(),
			Temperature: a.Temperature,
			MaxTokens:   a.MaxTokens,
		}
		asst, err := a.Client.Chat(ctx, req, llm.StreamHandler{
			OnContent:   cb.OnContent,
			OnReasoning: cb.OnReasoning,
		})
		if err != nil {
			runErr = err
			break
		}
		if asst.Content == "" && asst.Reasoning == "" && len(asst.ToolCalls) == 0 {
			asst.Content = "(空回复)"
		}
		a.History = append(a.History, asst)
		if cb.OnAssistantMessage != nil {
			cb.OnAssistantMessage(asst)
		}
		if len(asst.ToolCalls) == 0 {
			answered = true
			break
		}
		for _, tc := range asst.ToolCalls {
			if err := ctx.Err(); err != nil {
				runErr = errors.New("已停止")
				break
			}
			if cb.OnStatus != nil {
				cb.OnStatus("执行 " + tc.Function.Name + "…")
			}
			if cb.OnToolStart != nil {
				cb.OnToolStart(tc.ID, tc.Function.Name, tc.Function.Arguments)
			}
			result, failed := a.execTool(ctx, tc, cb)
			if cb.OnToolEnd != nil {
				cb.OnToolEnd(tc.ID, result, failed)
			}
			a.History = append(a.History, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}
	if !answered && runErr == nil {
		runErr = fmt.Errorf("已达最大工具调用轮数（%d），任务未完成", maxTurns)
	}
	if runErr == nil && ctx.Err() != nil {
		runErr = errors.New("已停止")
	}
	if cb.OnDone != nil {
		cb.OnDone(runErr)
	}
	return runErr
}

// Compact replaces the history with a summary message.
func (a *Agent) Compact(summary string) {
	a.History = []llm.Message{
		{Role: "user", Content: "[历史已压缩] 之前对话的摘要：\n" + summary},
	}
}

func (a *Agent) requestMessages() []llm.Message {
	msgs := make([]llm.Message, 0, len(a.History)+1)
	if strings.TrimSpace(a.SystemPrompt) != "" {
		msgs = append(msgs, llm.Message{Role: "system", Content: a.SystemPrompt})
	}
	return append(msgs, a.History...)
}

func (a *Agent) execTool(ctx context.Context, tc llm.ToolCall, cb Callbacks) (result string, failed bool) {
	defer func() {
		if r := recover(); r != nil {
			result, failed = fmt.Sprintf("error: tool panic: %v", r), true
		}
	}()
	if ctx.Err() != nil {
		return "[canceled]", true
	}

	switch tc.Function.Name {
	case "todo":
		var a2 struct {
			Todos []TodoItem `json:"todos"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &a2); err != nil {
			return "error: 参数不是合法 JSON: " + err.Error(), true
		}
		if cb.OnTodo != nil {
			cb.OnTodo(a2.Todos)
		}
		return fmt.Sprintf("ok，清单已更新（%d 项）", len(a2.Todos)), false

	case "task":
		return a.runSubagent(ctx, tc, cb)

	case "plugins":
		if a.PluginTool == nil {
			return "error: plugins 工具未挂载（当前预设不提供插件管理）", true
		}
		var a2 struct {
			Action string `json:"action"`
			Plugin string `json:"plugin"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &a2); err != nil {
			return "error: 参数不是合法 JSON: " + err.Error(), true
		}
		return a.PluginTool(a2.Action, a2.Plugin)

	case "skill":
		if a.SkillTool == nil {
			return "error: skill 工具未挂载（当前预设不提供技能）", true
		}
		var a2 struct {
			Action string `json:"action"`
			Name   string `json:"name"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &a2); err != nil {
			return "error: 参数不是合法 JSON: " + err.Error(), true
		}
		return a.SkillTool(a2.Action, a2.Name)

	case "run_code":
		if d := a.approve(cb, tc, "run_code"); d == Deny {
			return deniedResult(), true
		}
		return a.runCode(ctx, tc)

	default:
		if runner, ok := a.PackageTools[tc.Function.Name]; ok {
			if a.MutatingTools[tc.Function.Name] {
				if d := a.approve(cb, tc, tc.Function.Name); d == Deny {
					return deniedResult(), true
				}
			}
			return runner(ctx, tc.Function.Arguments)
		}
		t := tools.ByName(tc.Function.Name)
		if t == nil {
			return "error: unknown tool " + tc.Function.Name, true
		}
		if t.Mutating {
			if d := a.approve(cb, tc, t.Name); d == Deny {
				return deniedResult(), true
			}
		}
		if tc.Function.Name == "bash" && a.Shell != nil {
			return a.Shell.Run(ctx, tc.Function.Arguments)
		}
		return t.Run(ctx, tc.Function.Arguments, a.WorkDir)
	}
}

func (a *Agent) approve(cb Callbacks, tc llm.ToolCall, name string) Decision {
	if cb.OnApproval == nil {
		return Deny
	}
	brief := tc.Function.Arguments
	if len(brief) > 400 {
		brief = brief[:400] + "…"
	}
	return cb.OnApproval(name, brief)
}

func deniedResult() string {
	return "[用户拒绝了本次工具调用] 请尊重用户决定，不要原样重试；如确有必要，换一种方式并先说明理由。"
}

func (a *Agent) runSubagent(ctx context.Context, tc llm.ToolCall, cb Callbacks) (string, bool) {
	if a.Depth > 0 || a.Subagent == nil {
		return "error: 子代理不可用（已达最大深度）", true
	}
	var a2 struct {
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a2); err != nil {
		return "error: 参数不是合法 JSON: " + err.Error(), true
	}
	if strings.TrimSpace(a2.Prompt) == "" {
		return "error: prompt 为空", true
	}
	if cb.OnSubagentStart != nil {
		cb.OnSubagentStart(a2.Description)
	}
	child := a.Subagent()
	if child.Depth != a.Depth+1 {
		// The factory must build a depth+1 agent; enforce here.
		child.Depth = a.Depth + 1
	}
	child.Specs = a.Specs
	var out strings.Builder
	err := child.Run(ctx, a2.Prompt, Callbacks{
		OnContent: func(d string) { out.WriteString(d) },
	})
	result := strings.TrimSpace(out.String())
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		result += "\n[子代理未正常结束]"
	}
	if cb.OnSubagentEnd != nil {
		cb.OnSubagentEnd(a2.Description, result)
	}
	if err != nil {
		return result, true
	}
	return result, false
}

// runCode executes PTC mode: the model's JavaScript with the tools bound
// as functions (implemented with a pure-Go interpreter).
func (a *Agent) runCode(ctx context.Context, tc llm.ToolCall) (string, bool) {
	var a2 struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &a2); err != nil {
		return "error: 参数不是合法 JSON: " + err.Error(), true
	}
	if strings.TrimSpace(a2.Code) == "" {
		return "error: code 为空", true
	}
	return runPTCCode(ctx, a2.Code, a.WorkDir, func(rawName, argsJSON string) (string, bool) {
		name := strings.TrimSuffix(rawName, "_raw")
		if name == "bash" && a.Shell != nil {
			return a.Shell.Run(ctx, argsJSON)
		}
		t := tools.ByName(name)
		if t == nil {
			return "error: unknown tool " + name, true
		}
		return t.Run(ctx, argsJSON, a.WorkDir)
	})
}
