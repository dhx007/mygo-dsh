package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern, rel string
		want         bool
	}{
		{"**/*.go", "main.go", true},
		{"**/*.go", "internal/agent/agent.go", true},
		{"*.go", "main.go", true},
		{"*.go", "internal/agent/agent.go", false},
		{"src/**/*.ts", "src/a/b/c.ts", true},
		{"src/**/*.ts", "src/c.ts", true},
		{"src/**/*.ts", "other/c.ts", false},
		{"internal/*/*.go", "internal/agent/agent.go", true},
	}
	for _, c := range cases {
		if got := matchPath(c.pattern, c.rel); got != c.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", c.pattern, c.rel, got, c.want)
		}
	}
}

func TestGlobAndGrepAndRead(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc main() {}\n")
	write("internal/agent/agent.go", "package agent\n\nvar hello = \"world\"\n")
	write("notes.md", "# 标题\n\nhello deepseek\n")

	run := func(name, argsJSON string) string {
		tool := ByName(name)
		if tool == nil {
			t.Fatalf("missing tool %s", name)
		}
		out, failed := tool.Run(context.Background(), argsJSON, dir)
		if failed {
			t.Fatalf("%s failed: %s", name, out)
		}
		return out
	}

	if got := run("glob", `{"pattern":"**/*.go"}`); !strings.Contains(got, "main.go") || !strings.Contains(got, filepath.Join("internal", "agent", "agent.go")) {
		t.Errorf("glob result unexpected: %q", got)
	}
	if got := run("grep", `{"pattern":"hello","include":"*.go"}`); !strings.Contains(got, "agent.go:3:") {
		t.Errorf("grep result unexpected: %q", got)
	}
	if got := run("read_file", `{"path":"notes.md"}`); !strings.Contains(got, "标题") || !strings.Contains(got, "     1\t") {
		t.Errorf("read_file result unexpected: %q", got)
	}
	run("write_file", `{"path":"sub/dir/new.txt","content":"hi"}`)
	if b, err := os.ReadFile(filepath.Join(dir, "sub", "dir", "new.txt")); err != nil || string(b) != "hi" {
		t.Errorf("write_file did not write the file: %v", err)
	}
	if got := run("bash", `{"command":"echo ok && pwd"}`); !strings.Contains(got, "ok") {
		t.Errorf("bash result unexpected: %q", got)
	}
}

func TestBashReportsFailure(t *testing.T) {
	tool := ByName("bash")
	out, failed := tool.Run(context.Background(), `{"command":"exit 3"}`, t.TempDir())
	if !failed || !strings.Contains(out, "exit status 3") {
		t.Errorf("expected failure with exit status, got %q failed=%v", out, failed)
	}
}
