package main

import (
	"os"
	"strings"
	"testing"
)

// REQ-002 AC-1: exactly 13 built-ins (incl. 技能), no duplicate ids, every
// preset tool maps to a registered plugin, and grouping loses nothing.
func TestBuiltinPluginRegistry(t *testing.T) {
	all := builtinPlugins()
	if len(all) != 13 {
		t.Fatalf("registry has %d plugins, want 13", len(all))
	}
	seen := map[string]bool{}
	for _, p := range all {
		if seen[p.ID] {
			t.Fatalf("duplicate plugin id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Name == "" || p.Version == "" || p.Category == "" || p.Source == "" || len(p.Tools) == 0 {
			t.Errorf("plugin %q missing metadata: %+v", p.ID, p)
		}
	}
	for _, pre := range builtinPresets() {
		for _, tool := range pre.Tools {
			found := false
			for _, p := range all {
				for _, tl := range p.Tools {
					if tl == tool {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("preset %q tool %q has no plugin", pre.ID, tool)
			}
		}
	}
	total := 0
	for _, g := range pluginGroups(all) {
		total += len(g.Plugins)
	}
	if total != len(all) {
		t.Fatalf("grouping lost plugins: %d of %d", total, len(all))
	}
}

// REQ-002 AC-3: the plugins tool actions behave and persist.
func TestPluginControlActions(t *testing.T) {
	a := newTestApp(t)

	out, failed := a.pluginControl("list", "")
	if failed || !strings.Contains(out, "共 13 个内置插件") || !strings.Contains(out, "[终端]") {
		t.Fatalf("list: %q failed=%v", out, failed)
	}
	out, failed = a.pluginControl("info", "bash")
	if failed || !strings.Contains(out, "Shell 终端") || !strings.Contains(out, "已启用") {
		t.Fatalf("info: %q failed=%v", out, failed)
	}
	if _, failed := a.pluginControl("disable", "bash"); failed {
		t.Fatal("disable bash failed")
	}
	if a.enabledTool("bash") {
		t.Fatal("bash still enabled after disable")
	}
	if _, failed := a.pluginControl("enable", "bash"); failed {
		t.Fatal("enable bash failed")
	}
	if !a.enabledTool("bash") {
		t.Fatal("bash not re-enabled")
	}
	out, failed = a.pluginControl("nope", "")
	if !failed || !strings.Contains(out, "未知 action") {
		t.Fatalf("bad action: %q failed=%v", out, failed)
	}
	out, failed = a.pluginControl("info", "nope")
	if !failed || !strings.Contains(out, "未知插件") {
		t.Fatalf("bad id: %q failed=%v", out, failed)
	}
	// 持久化：disable 落盘到 config.json
	if _, failed := a.pluginControl("disable", "grep"); failed {
		t.Fatal("disable grep failed")
	}
	b, err := os.ReadFile(a.cfgPath)
	if err != nil || !strings.Contains(string(b), `"grep": false`) {
		t.Errorf("toggle not persisted: err=%v body=%s", err, b)
	}
}

// REQ-002 AC-4: creator mounts the plugins tool (unless the plugins plugin
// is off); standard never does.
func TestPluginToolPresetWiring(t *testing.T) {
	a := newTestApp(t)

	ses, err := a.store.New(a.cfg.WorkDir, "创造", "c")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses); ag.PluginTool == nil {
		t.Fatal("creator preset missing plugins tool")
	}

	a.setEnabled("plugins", false)
	ses2, err := a.store.New(a.cfg.WorkDir, "创造", "c2")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses2); ag.PluginTool != nil {
		t.Fatal("plugins plugin disabled but tool still mounted")
	}
	a.setEnabled("plugins", true)

	ses3, err := a.store.New(a.cfg.WorkDir, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses3); ag.PluginTool != nil {
		t.Fatal("standard preset should not manage plugins")
	}
}
