package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygo-harness/internal/llm"
	"mygo-harness/internal/tools"
)

func bridge(t *testing.T, dir string) func(name, argsJSON string) (string, bool) {
	t.Helper()
	return func(name, argsJSON string) (string, bool) {
		bt := tools.ByName(name)
		if bt == nil {
			return "error: unknown tool " + name, true
		}
		return bt.Run(context.Background(), argsJSON, dir)
	}
}

// REQ-004 AC-5: a package tool script runs with the tool bridge and args.
func TestRunPackageToolBridge(t *testing.T) {
	dir := t.TempDir()
	script := `function run(args){ write_file(args.path, args.content); return "wrote " + args.path; }`
	out, failed := RunPackageTool(context.Background(), script,
		`{"path":"out/x.txt","content":"hi"}`, dir, bridge(t, dir))
	if failed || !strings.Contains(out, "wrote") {
		t.Fatalf("out=%q failed=%v", out, failed)
	}
	b, err := os.ReadFile(filepath.Join(dir, "out", "x.txt"))
	if err != nil || string(b) != "hi" {
		t.Fatalf("bridged write_file result: %q %v", b, err)
	}
}

// REQ-004 AC-5: error paths — missing run(), thrown errors, bad args.
func TestRunPackageToolErrors(t *testing.T) {
	dir := t.TempDir()
	br := bridge(t, dir)

	out, failed := RunPackageTool(context.Background(), `var x = 1;`, "{}", dir, br)
	if !failed || !strings.Contains(out, "未定义 function run") {
		t.Fatalf("missing run: out=%q failed=%v", out, failed)
	}
	out, failed = RunPackageTool(context.Background(), `function run(args){ throw new Error("boom"); }`, "{}", dir, br)
	if !failed || !strings.Contains(out, "boom") {
		t.Fatalf("throw: out=%q failed=%v", out, failed)
	}
	out, failed = RunPackageTool(context.Background(), `function run(args){ return args.q; }`, `{bad json`, dir, br)
	if !failed || !strings.Contains(out, "JSON") {
		t.Fatalf("bad args: out=%q failed=%v", out, failed)
	}
	out, failed = RunPackageTool(context.Background(), `function run(args){ return "got " + args.q; }`, `{"q":"zz"}`, dir, br)
	if failed || strings.TrimSpace(out) != "got zz" {
		t.Fatalf("args roundtrip: out=%q failed=%v", out, failed)
	}
}

// REQ-004 AC-5: a runaway script is interrupted at the timeout.
func TestRunPackageToolTimeout(t *testing.T) {
	old := packageToolTimeout
	packageToolTimeout = 200 * time.Millisecond
	t.Cleanup(func() { packageToolTimeout = old })

	dir := t.TempDir()
	start := time.Now()
	out, failed := RunPackageTool(context.Background(), `function run(args){ while(true){} }`, "{}", dir, bridge(t, dir))
	if !failed || !strings.Contains(out, "超时") {
		t.Fatalf("timeout: out=%q failed=%v", out, failed)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("interrupt took too long: %s", time.Since(start))
	}
}

// REQ-004 AC-5: execTool routes package tools and gates mutating ones.
func TestPackageToolDispatch(t *testing.T) {
	ran := false
	mk := func(deny bool) (*Agent, Callbacks) {
		a := &Agent{
			PackageTools: map[string]func(ctx context.Context, argsJSON string) (string, bool){
				"demo": func(ctx context.Context, argsJSON string) (string, bool) {
					ran = true
					return "ran", false
				},
			},
			MutatingTools: map[string]bool{"demo": true},
		}
		cb := Callbacks{OnApproval: func(tool, args string) Decision {
			if deny {
				return Deny
			}
			return AllowOnce
		}}
		return a, cb
	}
	var tc llm.ToolCall
	tc.Function.Name = "demo"
	tc.Function.Arguments = "{}"

	ran = false
	a, cb := mk(false)
	out, failed := a.execTool(context.Background(), tc, cb)
	if failed || out != "ran" || !ran {
		t.Fatalf("dispatch: out=%q failed=%v ran=%v", out, failed, ran)
	}

	ran = false
	denied := false
	a2, cb2 := mk(true)
	cb2.OnApproval = func(tool, args string) Decision { denied = true; return Deny }
	out, failed = a2.execTool(context.Background(), tc, cb2)
	if !failed || !denied || ran || !strings.Contains(out, "拒绝") {
		t.Fatalf("deny: out=%q failed=%v denied=%v ran=%v", out, failed, denied, ran)
	}
}
