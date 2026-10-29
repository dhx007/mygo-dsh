package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// REQ-009 AC-1: start returns immediately; output becomes readable and the
// job finishes with exit 0.
func TestJobLifecycle(t *testing.T) {
	id, err := Jobs.Start("echo hello; sleep 0.3; echo done", t.TempDir())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	text, state, _, err := Jobs.Output(id, 0)
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if state != "running" {
		t.Fatalf("state = %q right after start", state)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		text, state, _, _ = Jobs.Output(id, 0)
		if state != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state == "running" {
		t.Fatal("job never finished")
	}
	if !strings.Contains(state, "exit 0") || !strings.Contains(text, "hello") || !strings.Contains(text, "done") {
		t.Fatalf("final: state=%q text=%q", state, text)
	}
	// job_list 可见
	found := false
	for _, j := range Jobs.List() {
		if j.ID == id && j.State == "finished" {
			found = true
		}
	}
	if !found {
		t.Fatal("job not listed as finished")
	}
}

// REQ-009 AC-2: output paging with a small cap reads the full stream.
func TestJobOutputPaging(t *testing.T) {
	old := OutputCapBytes
	OutputCapBytes = 1024
	t.Cleanup(func() { OutputCapBytes = old })

	id, err := Jobs.Start("head -c 4096 /dev/zero | tr '\\0' 'z'", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// 等任务完成再分页（输出才完整）
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, state, _, _ := Jobs.Output(id, 0); state != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var (
		text  string
		next  int
		total int
	)
	for i := 0; i < 10; i++ {
		var more bool
		text, _, next, _ = Jobs.Output(id, next)
		if j := strings.Index(text, "\n…[输出共"); j >= 0 {
			more = true
			text = text[:j]
		}
		total += len(text)
		if !more {
			break // 已到末尾
		}
	}
	if total != 4096 {
		t.Fatalf("paged %d bytes, want 4096", total)
	}
}

// REQ-009 AC-3: job_kill takes the whole tree down.
func TestJobKill(t *testing.T) {
	id, err := Jobs.Start("sleep 30", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := Jobs.Kill(id); err != nil {
		t.Fatalf("kill: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, state, _, _ := Jobs.Output(id, 0); state != "running" {
			if !strings.Contains(state, "killed") {
				t.Fatalf("state after kill: %q", state)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job still running after kill")
	// 重复杀已结束任务报错
	if err := Jobs.Kill(id); err == nil {
		t.Fatal("killing a finished job should fail")
	}
}

// REQ-009 AC-1: the bash tool's background flag returns a job id at once.
func TestBashBackgroundParam(t *testing.T) {
	b := ByName("bash")
	start := time.Now()
	out, failed := b.Run(context.Background(), `{"command":"sleep 5","background":true}`, t.TempDir())
	if failed {
		t.Fatalf("out=%s", out)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("background start blocked %s", time.Since(start))
	}
	i := strings.Index(out, "job-")
	if i < 0 {
		t.Fatalf("no job id in %q", out)
	}
	id := out[i:]
	if j := strings.IndexAny(id, " \n"); j >= 0 {
		id = id[:j]
	}
	// job_list 可见且运行中
	var listed *JobInfo
	for _, j := range Jobs.List() {
		if j.ID == id {
			listed = &j
		}
	}
	if listed == nil || listed.State != "running" {
		t.Fatalf("job not listed running: %+v", listed)
	}
	// 收尾：杀掉
	if err := Jobs.Kill(id); err != nil {
		t.Fatal(err)
	}
	// 未知任务输出报错
	if _, failed = ByName("job_output").Run(context.Background(), `{"job_id":"job-999999"}`, t.TempDir()); !failed {
		t.Fatal("unknown job should fail")
	}
}
