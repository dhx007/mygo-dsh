package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// REQ-001 AC-1: env and cwd survive across Run calls.
func TestPersistentShellStatePersists(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewPersistentShell(root)
	defer s.Close()

	if out, failed := s.Run(context.Background(), `{"command":"export MYGO_PERSIST=hello"}`); failed {
		t.Fatalf("export failed: %s", out)
	}
	out, failed := s.Run(context.Background(), `{"command":"echo $MYGO_PERSIST"}`)
	if failed || strings.TrimSpace(out) != "hello" {
		t.Fatalf("env not persisted: out=%q failed=%v", out, failed)
	}

	if out, failed := s.Run(context.Background(), `{"command":"cd sub"}`); failed {
		t.Fatalf("cd failed: %s", out)
	}
	out, _ = s.Run(context.Background(), `{"command":"pwd"}`)
	if !strings.HasSuffix(strings.TrimSpace(out), "/sub") {
		t.Fatalf("cwd not persisted: pwd=%q want */sub", out)
	}
}

// REQ-001 AC-2: exit codes mark failures and stderr is merged in.
func TestPersistentShellExitCodeAndStderr(t *testing.T) {
	s := NewPersistentShell(t.TempDir())
	defer s.Close()

	out, failed := s.Run(context.Background(), `{"command":"false"}`)
	if !failed || !strings.Contains(out, "rc=1") {
		t.Fatalf("false: out=%q failed=%v, want failure with rc=1", out, failed)
	}
	out, failed = s.Run(context.Background(), `{"command":"echo boom 1>&2"}`)
	if failed || !strings.Contains(out, "boom") {
		t.Fatalf("stderr not merged: out=%q failed=%v", out, failed)
	}
}

// REQ-001 AC-3: a timed-out command is killed (whole tree) and the shell
// resets; the next call runs in a fresh shell.
func TestPersistentShellTimeout(t *testing.T) {
	s := NewPersistentShell(t.TempDir())
	defer s.Close()

	start := time.Now()
	out, failed := s.Run(context.Background(), `{"command":"sleep 5","timeout_seconds":1}`)
	if !failed || !strings.Contains(out, "超时") {
		t.Fatalf("timeout: out=%q failed=%v", out, failed)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("timeout took %s, kill did not fire", elapsed)
	}
	out, failed = s.Run(context.Background(), `{"command":"echo ok"}`)
	if failed || strings.TrimSpace(out) != "ok" {
		t.Fatalf("next call after timeout: out=%q failed=%v", out, failed)
	}
}

// REQ-001 AC-4: `exit` ends the shell; the call reports it honestly and
// the next call starts a fresh shell with cleared state.
func TestPersistentShellRestartAfterExit(t *testing.T) {
	s := NewPersistentShell(t.TempDir())
	defer s.Close()

	if _, failed := s.Run(context.Background(), `{"command":"export GONE=1"}`); failed {
		t.Fatal("export failed")
	}
	out, failed := s.Run(context.Background(), `{"command":"exit 7"}`)
	if !failed || !strings.Contains(out, "已退出") {
		t.Fatalf("exit 7: out=%q failed=%v, want session-exited report", out, failed)
	}
	out, failed = s.Run(context.Background(), `{"command":"echo ${GONE-unset} alive"}`)
	if failed || strings.TrimSpace(out) != "unset alive" {
		t.Fatalf("fresh shell state: out=%q failed=%v", out, failed)
	}
}

// Multiline commands and heredocs ride stdin as-is.
func TestPersistentShellHeredocAndMultiline(t *testing.T) {
	s := NewPersistentShell(t.TempDir())
	defer s.Close()

	args := `{"command":"cat <<'EOF'\nline1\nline2\nEOF"}`
	out, failed := s.Run(context.Background(), args)
	if failed || !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Fatalf("heredoc: out=%q failed=%v", out, failed)
	}
}
