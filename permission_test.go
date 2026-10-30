package main

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"mygo-harness/internal/agent"
)

// REQ-013 AC-2: legacy auto_approve migrates to the 完全信任 preset.
func TestAccessModeMigration(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.json"
	if err := os.WriteFile(path, []byte(`{"api_key":"sk-x","auto_approve":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dataDirOverride = dir
	t.Cleanup(func() { dataDirOverride = "" })
	cfg, _ := loadConfig()
	if cfg.accessMode() != "完全信任" {
		t.Fatalf("migration failed: %q", cfg.accessMode())
	}
	// 未知值归一化为默认
	cfg.AccessMode = "nonsense"
	if cfg.accessMode() != "工作区写入" {
		t.Fatalf("normalize failed: %q", cfg.accessMode())
	}
}

// REQ-013 AC-2: decideApproval applies the three presets.
func TestDecideApproval(t *testing.T) {
	a := newTestApp(t)
	ses, err := a.store.New(a.cfg.WorkDir, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	asks := 0
	ask := func() agent.Decision { asks++; return agent.AllowAlways }

	// 完全信任：不询问
	a.setAccessMode("完全信任")
	a.errText = ""
	clearNote(a)
	if d := a.decideApproval(ses, "bash", "{}", ask); d != agent.AllowOnce || asks != 0 {
		t.Fatalf("完全信任: d=%v asks=%d", d, asks)
	}

	// 只读：直接拒绝，不询问
	a.setAccessMode("只读")
	clearNote(a)
	if d := a.decideApproval(ses, "bash", "{}", ask); d != agent.Deny || asks != 0 {
		t.Fatalf("只读: d=%v asks=%d", d, asks)
	}

	// 工作区写入：走询问流；AllowAlways 记入 alwaysAllow
	a.setAccessMode("工作区写入")
	clearNote(a)
	if d := a.decideApproval(ses, "bash", "{}", ask); d != agent.AllowOnce || asks != 1 {
		t.Fatalf("工作区写入: d=%v asks=%d", d, asks)
	}
	if !a.alwaysAllow["bash"] {
		t.Fatal("AllowAlways not remembered")
	}
	// 审批留痕
	found := false
	for _, ev := range ses.Events() {
		if ev.Kind == "approval" {
			found = true
		}
	}
	if !found {
		t.Fatal("approval events missing")
	}
}

// REQ-013 AC-3: the composer control switches and persists the preset.
func TestAccessModeUI(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	if !tt.HasText("工作区写入") {
		t.Errorf("mode control text missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("工作区写入"); err != nil {
		t.Fatalf("open mode menu: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("变更类工具一律拒绝") {
		t.Errorf("mode popover missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("只读"); err != nil {
		t.Fatalf("pick 只读: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if a.cfg.accessMode() != "只读" {
		t.Fatalf("mode = %q", a.cfg.accessMode())
	}
	b, err := os.ReadFile(a.cfgPath)
	if err != nil || !strings.Contains(string(b), `"access_mode": "只读"`) {
		t.Errorf("not persisted: err=%v body=%s", err, b)
	}
}

// clearNote drops the trailing note message that setAccessMode appends, so
// assertions stay focused on decisions.
func clearNote(a *app) {
	if n := len(a.msgs); n > 0 && a.msgs[n-1].role == "assistant" {
		a.msgs = a.msgs[:n-1]
	}
	a.errText = ""
}
