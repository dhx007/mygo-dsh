package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mygo-harness/internal/llm"
	"mygo-harness/internal/tools"
)

// REQ-002: plugin toggles filter the toolset per Run, and the plugins spec
// mounts only when PluginTool is wired and the plugins plugin is on.
func TestToolSpecsFilterAndPlugins(t *testing.T) {
	on := map[string]bool{"bash": true, "read_file": true, "plugins": true}
	a := &Agent{
		Specs:         tools.SpecsFor([]string{"bash", "read_file"}),
		PluginTool:    func(action, plugin string) (string, bool) { return "", false },
		PluginEnabled: func(id string) bool { return on[id] },
	}
	names := specNames(a.toolSpecs())
	for _, want := range []string{"bash", "read_file", "todo", "plugins"} {
		if !contains(names, want) {
			t.Fatalf("spec set missing %q: %v", want, names)
		}
	}

	on["bash"] = false
	if names = specNames(a.toolSpecs()); contains(names, "bash") {
		t.Fatalf("bash still in specs after toggle-off: %v", names)
	}

	a2 := &Agent{Specs: tools.SpecsFor([]string{"bash"})}
	if contains(specNames(a2.toolSpecs()), "plugins") {
		t.Fatal("plugins spec mounted without PluginTool")
	}

	on["plugins"] = false
	if contains(specNames(a.toolSpecs()), "plugins") {
		t.Fatal("plugins spec mounted while plugins plugin disabled")
	}
}

// REQ-002: the plugins tool dispatches to the callback and reports errors
// for unknown actions.
func TestPluginsToolDispatch(t *testing.T) {
	var gotAction, gotID string
	a := &Agent{PluginTool: func(action, plugin string) (string, bool) {
		gotAction, gotID = action, plugin
		return "ok", false
	}}
	var tc llm.ToolCall
	tc.Function.Name = "plugins"
	tc.Function.Arguments = `{"action":"disable","plugin":"bash"}`
	out, failed := a.execTool(context.Background(), tc, Callbacks{})
	if failed || out != "ok" || gotAction != "disable" || gotID != "bash" {
		t.Fatalf("dispatch: out=%q failed=%v action=%q id=%q", out, failed, gotAction, gotID)
	}

	a2 := &Agent{}
	tc2 := llm.ToolCall{}
	tc2.Function.Name = "plugins"
	tc2.Function.Arguments = `{"action":"list"}`
	out, failed = a2.execTool(context.Background(), tc2, Callbacks{})
	if !failed || !strings.Contains(out, "未挂载") {
		t.Fatalf("unmounted: out=%q failed=%v", out, failed)
	}
}

// REQ-003: the skill spec mounts only when SkillTool is wired and the
// skill plugin is on.
func TestSkillSpecMounting(t *testing.T) {
	on := map[string]bool{"skill": true}
	a := &Agent{
		SkillTool:     func(action, name string) (string, bool) { return "", false },
		PluginEnabled: func(id string) bool { return on[id] },
	}
	if !contains(specNames(a.toolSpecs()), "skill") {
		t.Fatal("skill spec missing while mounted")
	}
	on["skill"] = false
	if contains(specNames(a.toolSpecs()), "skill") {
		t.Fatal("skill spec present while plugin disabled")
	}
	a.SkillTool = nil
	if contains(specNames(a.toolSpecs()), "skill") {
		t.Fatal("skill spec present without SkillTool")
	}
}

func specNames(specs []llm.ToolSpec) []string {
	out := make([]string, 0, len(specs))
	for _, sp := range specs {
		out = append(out, sp.Function.Name)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// REQ-001: with a Shell mounted, the bash tool routes through the
// persistent shell (state persists) and still passes the approval gate.
func TestBashRoutesThroughPersistentShell(t *testing.T) {
	s := tools.NewPersistentShell(t.TempDir())
	defer s.Close()

	approved := 0
	a := &Agent{Shell: s}
	cb := Callbacks{OnApproval: func(tool, args string) Decision {
		approved++
		return AllowOnce
	}}

	mkCall := func(id, cmd string) llm.ToolCall {
		var tc llm.ToolCall
		tc.ID = id
		tc.Function.Name = "bash"
		tc.Function.Arguments = `{"command":` + quote(cmd) + `}`
		return tc
	}
	if out, failed := a.execTool(context.Background(), mkCall("t1", "export ROUTED=yes"), cb); failed {
		t.Fatalf("export: %s", out)
	}
	out, failed := a.execTool(context.Background(), mkCall("t2", "echo $ROUTED"), cb)
	if failed || !strings.Contains(out, "yes") {
		t.Fatalf("state did not persist via execTool: out=%q failed=%v", out, failed)
	}
	if approved != 2 {
		t.Fatalf("approval gate bypassed: approved=%d", approved)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
