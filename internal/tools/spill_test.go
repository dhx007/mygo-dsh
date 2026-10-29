package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// spillPath extracts the spilled temp-file path from a result text.
func spillPath(out string) string {
	i := strings.Index(out, "已落盘：")
	if i < 0 {
		return ""
	}
	rest := out[i+len("已落盘："):]
	if j := strings.Index(rest, "（"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// REQ-007 AC-1/2: one-shot bash output beyond the cap spills to a temp
// file with the full content; within-cap output never spills.
func TestBashOutputSpill(t *testing.T) {
	old := OutputCapBytes
	OutputCapBytes = 1024
	t.Cleanup(func() { OutputCapBytes = old })

	b := ByName("bash")
	out, failed := b.Run(context.Background(), `{"command":"head -c 4096 /dev/zero | tr '\\0' 'x'"}`, t.TempDir())
	if failed {
		t.Fatalf("out=%s", out)
	}
	if !strings.Contains(out, "已落盘") {
		t.Fatalf("spill note missing: %q", out)
	}
	path := spillPath(out)
	if path == "" {
		t.Fatalf("no spill path in %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spill file: %v", err)
	}
	if len(data) != 4096 {
		t.Fatalf("spilled %d bytes, want 4096", len(data))
	}

	OutputCapBytes = 64000
	out2, _ := b.Run(context.Background(), `{"command":"echo hi"}`, t.TempDir())
	if strings.Contains(out2, "已落盘") {
		t.Fatalf("unexpected spill: %q", out2)
	}
}

// REQ-007 AC-1: the persistent shell spills the same way.
func TestPersistentShellSpill(t *testing.T) {
	old := OutputCapBytes
	OutputCapBytes = 1024
	t.Cleanup(func() { OutputCapBytes = old })

	s := NewPersistentShell(t.TempDir())
	defer s.Close()
	out, failed := s.Run(context.Background(), `{"command":"head -c 4096 /dev/zero | tr '\\0' 'y'"}`)
	if failed {
		t.Fatalf("out=%s", out)
	}
	path := spillPath(out)
	if path == "" {
		t.Fatalf("spill note missing: %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spill file: %v", err)
	}
	if len(data) < 4096 {
		t.Fatalf("spilled %d bytes, want >= 4096", len(data))
	}
}

// REQ-007 AC-3: DefaultTimeout applies when the model passes no
// timeout_seconds; an explicit argument overrides it.
func TestShellTimeoutConfig(t *testing.T) {
	old := DefaultTimeout
	DefaultTimeout = 1 * time.Second
	t.Cleanup(func() { DefaultTimeout = old })

	b := ByName("bash")
	start := time.Now()
	out, failed := b.Run(context.Background(), `{"command":"sleep 3"}`, t.TempDir())
	if !failed {
		t.Fatalf("sleep 3 should fail at 1s default: out=%s", out)
	}
	if time.Since(start) > 2500*time.Millisecond {
		t.Fatalf("default timeout not applied: %s", time.Since(start))
	}
	// 显式参数覆盖默认值（sleep 1 < 5s，能正常完成）
	if out, failed = b.Run(context.Background(), `{"command":"sleep 1","timeout_seconds":5}`, t.TempDir()); failed {
		t.Fatalf("explicit timeout_seconds should win: out=%s", out)
	}

	// 持久 shell 同样受默认值约束
	s := NewPersistentShell(t.TempDir())
	defer s.Close()
	start = time.Now()
	out, failed = s.Run(context.Background(), `{"command":"sleep 3"}`)
	if !failed || !strings.Contains(out, "超时") {
		t.Fatalf("persistent timeout: out=%q failed=%v", out, failed)
	}
	if time.Since(start) > 2500*time.Millisecond {
		t.Fatalf("persistent default timeout not applied: %s", time.Since(start))
	}
}
