package agent

import (
	"context"
	"strings"
	"testing"

	"mygo-harness/internal/tools"
)

func TestRunPTCCode(t *testing.T) {
	dir := t.TempDir()
	out, failed := runPTCCode(context.Background(), `
		const list = bash("echo one two three");
		const files = glob("**/*");
		write_file("notes.txt", "written from PTC");
		const content = read_file("notes.txt");
		return list.trim() + "|" + content.trim() + "|" + files.length;
	`, dir, func(rawName, argsJSON string) (string, bool) {
		tl := tools.ByName(strings.TrimSuffix(rawName, "_raw"))
		if tl == nil {
			return "unknown tool", true
		}
		return tl.Run(context.Background(), argsJSON, dir)
	})
	if failed {
		t.Fatalf("runPTCCode failed: %s", out)
	}
	if !strings.Contains(out, "one two three") || !strings.Contains(out, "written from PTC") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestRunPTCTimeoutGuardCompiles(t *testing.T) {
	// Just verify the interpreter initializes with the wrappers; a real
	// timeout test would burn 120s, so only a quick loop runs.
	out, failed := runPTCCode(context.Background(), `
		var s = 0;
		for (var i = 0; i < 1000; i++) { s += i; }
		return String(s);
	`, t.TempDir(), func(string, string) (string, bool) { return "", false })
	if failed || out != "499500" {
		t.Errorf("loop result = %q failed=%v", out, failed)
	}
}
