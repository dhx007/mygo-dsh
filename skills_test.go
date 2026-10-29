package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill drops a <name>/SKILL.md under root (body only when fm empty).
func writeSkill(t *testing.T, root, name, fm, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := body
	if fm != "" {
		content = "---\n" + fm + "\n---\n" + body
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// REQ-003 AC-1: discovery across both roots, workspace shadowing, name
// fallback, directories without SKILL.md skipped.
func TestDiscoverSkills(t *testing.T) {
	dataDir := t.TempDir()
	ws := t.TempDir()
	g, w := skillRoots(dataDir, ws)
	writeSkill(t, g, "deploy", "name: deploy\ndescription: 发布流程", "全局正文")
	writeSkill(t, g, "nofm", "", "纯正文技能")
	writeSkill(t, w, "deploy", "description: 工作区覆盖版", "工作区正文")
	empty := filepath.Join(w, "hollow")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(dataDir, ws)
	if len(skills) != 2 {
		t.Fatalf("got %d skills (%+v), want 2", len(skills), skills)
	}
	// 工作区在前
	if skills[0].ID != "deploy" || skills[0].Source != "工作区" || skills[0].Body != "工作区正文" {
		t.Fatalf("workspace shadow failed: %+v", skills[0])
	}
	if skills[1].ID != "nofm" || skills[1].Source != "全局" || skills[1].Name != "nofm" || skills[1].Body != "纯正文技能" {
		t.Fatalf("global fallback failed: %+v", skills[1])
	}
}

// REQ-003 AC-2: skill tool list/run/errors.
func TestSkillControlActions(t *testing.T) {
	a := newTestApp(t)
	g, _ := skillRoots(a.dataDir, a.cfg.WorkDir)
	writeSkill(t, g, "deploy", "description: 发布流程", "步骤一")

	out, failed := a.skillControl("list", "")
	if failed || !strings.Contains(out, "deploy") || !strings.Contains(out, "发布流程") {
		t.Fatalf("list: %q failed=%v", out, failed)
	}
	out, failed = a.skillControl("run", "deploy")
	if failed || !strings.Contains(out, "步骤一") || !strings.Contains(out, "【技能：deploy") {
		t.Fatalf("run: %q failed=%v", out, failed)
	}
	out, failed = a.skillControl("run", "nope")
	if !failed || !strings.Contains(out, "未知技能") {
		t.Fatalf("unknown skill: %q failed=%v", out, failed)
	}
	out, failed = a.skillControl("bad", "")
	if !failed || !strings.Contains(out, "未知 action") {
		t.Fatalf("bad action: %q failed=%v", out, failed)
	}
}

// REQ-003 AC-3: 标准/创造 mount the skill tool; PTC/极简 don't; disabling
// the 技能 plugin removes it.
func TestSkillToolPresetWiring(t *testing.T) {
	a := newTestApp(t)
	for _, mode := range []string{"标准", "创造"} {
		ses, err := a.store.New(a.cfg.WorkDir, mode, "s-"+mode)
		if err != nil {
			t.Fatal(err)
		}
		if ag := a.newAgentFor(ses); ag.SkillTool == nil {
			t.Errorf("%s preset missing skill tool", mode)
		}
	}
	for _, mode := range []string{"PTC", "极简"} {
		ses, err := a.store.New(a.cfg.WorkDir, mode, "s-"+mode)
		if err != nil {
			t.Fatal(err)
		}
		if ag := a.newAgentFor(ses); ag.SkillTool != nil {
			t.Errorf("%s preset should not mount skill tool", mode)
		}
	}
	a.setEnabled("skill", false)
	ses, err := a.store.New(a.cfg.WorkDir, "标准", "s-off")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses); ag.SkillTool != nil {
		t.Error("skill plugin disabled but tool still mounted")
	}
}
