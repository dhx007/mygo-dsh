package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-008 AC-1/2: discovery order, fallback, empty-file skip, truncation.
func TestLoadWorkspaceInstructions(t *testing.T) {
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// AGENTS.md 优先
	ws := t.TempDir()
	write(ws, "AGENTS.md", "用 go build 构建")
	write(ws, "CLAUDE.md", "不该被用")
	out := loadWorkspaceInstructions(ws)
	if !strings.Contains(out, "[工作区指令：AGENTS.md]") || !strings.Contains(out, "go build") {
		t.Fatalf("agents.md not used: %q", out)
	}

	// 仅 CLAUDE.md → 回退
	ws2 := t.TempDir()
	write(ws2, "CLAUDE.md", "Claude 约定")
	if out = loadWorkspaceInstructions(ws2); !strings.Contains(out, "[工作区指令：CLAUDE.md]") || !strings.Contains(out, "Claude 约定") {
		t.Fatalf("fallback failed: %q", out)
	}

	// AGENTS.md 为空 → 回退 CLAUDE.md
	ws3 := t.TempDir()
	write(ws3, "AGENTS.md", "   \n")
	write(ws3, "CLAUDE.md", "兜底")
	if out = loadWorkspaceInstructions(ws3); !strings.Contains(out, "兜底") {
		t.Fatalf("empty-file skip failed: %q", out)
	}

	// 都没有 → 空
	if out := loadWorkspaceInstructions(t.TempDir()); out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
	if out := loadWorkspaceInstructions(""); out != "" {
		t.Fatalf("empty workspace: %q", out)
	}

	// 超长截断
	ws4 := t.TempDir()
	write(ws4, "AGENTS.md", strings.Repeat("x", 40*1024))
	if out = loadWorkspaceInstructions(ws4); !strings.Contains(out, "已截断") || len(out) > 34*1024 {
		t.Fatalf("truncation failed: len=%d", len(out))
	}
}

// REQ-008 AC-3: instructions inject into the agent system prompt at build
// time and the 工作区指令 plugin gates them.
func TestAgentInstructionsWiring(t *testing.T) {
	a := newTestApp(t)
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "AGENTS.md"), []byte("永远使用中文回复"), 0o644); err != nil {
		t.Fatal(err)
	}
	ses, err := a.store.New(ws, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	prompt := a.newAgentFor(ses).SystemPrompt
	if !strings.Contains(prompt, "[工作区指令：AGENTS.md]") || !strings.Contains(prompt, "永远使用中文回复") {
		t.Fatalf("instructions not injected: %q", prompt)
	}

	// 停用插件 → 不注入
	a.setEnabled("instructions", false)
	if prompt = a.newAgentFor(ses).SystemPrompt; strings.Contains(prompt, "[工作区指令") {
		t.Fatalf("disabled plugin still injects: %q", prompt)
	}
	// 无文件 → 不注入
	a.setEnabled("instructions", true)
	ses2, err := a.store.New(t.TempDir(), "标准", "s2")
	if err != nil {
		t.Fatal(err)
	}
	if prompt = a.newAgentFor(ses2).SystemPrompt; strings.Contains(prompt, "[工作区指令") {
		t.Fatalf("unexpected injection: %q", prompt)
	}
}
