package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Preset is an agent mode: the tool set and persona the session runs
// with. The original ships four: standard, ptc, minimal and creator.
type Preset struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Tools        []string `json:"tools"`
	SystemPrompt string   `json:"system_prompt"`
	// PersistentShell runs the bash tool in one long-lived shell whose
	// env/cwd survive across calls (the original's minimal preset ships
	// this; customs can opt in via presets.json).
	PersistentShell bool `json:"persistent_shell"`
	// CanManagePlugins mounts the model-side plugins tool (the original
	// gates it to creator with an editable profile).
	CanManagePlugins bool `json:"can_manage_plugins"`
	Custom           bool `json:"custom"`
}

const standardPrompt = "You are DeepSeek Harness, a coding agent running on the user's machine (macOS), working inside the workspace directory. " +
	"You can run shell commands and read, write and edit files; prefer tools over guessing and inspect real files before claiming anything about them. " +
	"Plan multi-step work with the todo tool, and delegate self-contained subtasks with the task tool. " +
	"Check the skill list when a task might match an installed skill, and run the skill to follow its instructions. " +
	"Keep commands safe and reversible; never destroy user data without asking. " +
	"Answer concisely, in the user's language (Chinese when they write Chinese). " +
	"Current working directory: {cwd}"

const minimalPrompt = "You are DeepSeek Harness in minimal mode: a plain coding agent with exactly two tools, a shell and a file editor. " +
	"Solve the task with bash and file writes alone; no other tools exist. " +
	"Your bash session is persistent: working directory, exported variables and shell options survive across calls, " +
	"so set up the environment once instead of repeating it in every command. " +
	"Answer concisely, in the user's language (Chinese when they write Chinese). " +
	"Current working directory: {cwd}"

const ptcPrompt = "You are DeepSeek Harness in PTC (programmatic tool calling) mode. " +
	"Instead of calling tools one by one, you write a JavaScript program and run it with the run_code tool; " +
	"inside the program, the tools are plain functions you can call in loops: bash(cmd), read_file(path), write_file(path, content), glob(pattern), grep(pattern), web_fetch(url). " +
	"Prefer one well-structured program over many round trips; return the final answer string from the program. " +
	"Answer concisely, in the user's language (Chinese when they write Chinese). " +
	"Current working directory: {cwd}"

const creatorPrompt = "You are DeepSeek Harness in creator mode: the mode that builds modes. " +
	"You have the full standard tool set and can read and write the app's preset files (presets.json in the app data directory). " +
	"You also have the plugins tool to list, inspect, enable and disable the app's plugins — use it when the user asks about plugins or wants one toggled. " +
	"When the user asks for a new mode, create or edit a preset: an object with id, label, description, tools (from: bash, read_file, write_file, edit_file, glob, grep, web_fetch, web_search, todo, task, run_code) and system_prompt. " +
	"Keep the JSON valid; the app reloads presets from disk on restart. " +
	"Answer concisely, in the user's language (Chinese when they write Chinese). " +
	"Current working directory: {cwd}. App data directory: {data}"

var standardTools = []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep", "web_fetch", "web_search", "todo", "task", "skill"}
var minimalTools = []string{"bash", "write_file", "edit_file"}
var ptcTools = []string{"run_code"}
var creatorTools = []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep", "web_fetch", "web_search", "todo", "task", "skill"}

func builtinPresets() []Preset {
	return []Preset{
		{ID: "标准", Label: "标准", Description: "全套工具：文件编辑、shell、文件与网页读取、规划、子代理", Tools: standardTools, SystemPrompt: standardPrompt},
		{ID: "PTC", Label: "PTC", Description: "程序化工具调用：模型写一段脚本，把多次工具操作合并成一次执行", Tools: ptcTools, SystemPrompt: ptcPrompt},
		{ID: "极简", Label: "极简", Description: "只留一个持久 shell 和一个文件编辑器，跨命令保留 env/cwd，考模型裸实力", Tools: minimalTools, SystemPrompt: minimalPrompt, PersistentShell: true},
		{ID: "创造", Label: "创造", Description: "用全套工具去创造新的模式，并可管理插件（plugins 工具）", Tools: creatorTools, SystemPrompt: creatorPrompt, CanManagePlugins: true},
	}
}

// presetsFile is the on-disk list: built-ins plus user customs.
type presetsFile struct {
	Customs []Preset `json:"customs"`
}

func loadPresets(dir string) []Preset {
	all := builtinPresets()
	b, err := os.ReadFile(filepath.Join(dir, "presets.json"))
	if err == nil {
		var pf presetsFile
		if json.Unmarshal(b, &pf) == nil {
			for _, p := range pf.Customs {
				p.Custom = true
				p.ID = p.Label
				if len(p.Tools) == 0 {
					p.Tools = append([]string(nil), standardTools...)
				}
				all = append(all, p)
			}
		}
	}
	return all
}

func saveCustomPresets(dir string, customs []Preset) error {
	pf := presetsFile{Customs: customs}
	b, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "presets.json"), b, 0o644)
}

func presetByID(all []Preset, id string) *Preset {
	for i := range all {
		if all[i].ID == id {
			return &all[i]
		}
	}
	return &all[0]
}

func applyCwdTemplate(prompt, cwd, dataDir string) string {
	p := strings.ReplaceAll(prompt, "{cwd}", cwd)
	return strings.ReplaceAll(p, "{data}", dataDir)
}
