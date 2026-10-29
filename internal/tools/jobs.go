package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Background job registry: the model starts long-running commands with the
// bash tool's background flag and manages them with job_output / job_list /
// job_kill. This mirrors the original's ctx.jobs registry — host-level,
// outliving individual sessions. Jobs keep no state on disk and die with
// the host process.

const jobBufferLimit = 1024 * 1024 // 环形缓冲上限：1MB，超出丢头部

// jobBuffer accumulates a job's combined output, safe for concurrent
// writes (the child) and reads (job_output). It keeps only the tail up to
// jobBufferLimit; dropped bytes are accounted in dropped.
type jobBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	dropped int
}

func (b *jobBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if b.buf.Len() > jobBufferLimit {
		drop := b.buf.Len() - jobBufferLimit
		b.dropped += drop
		b.buf.Next(drop) // 丢头部
	}
	return len(p), nil
}

// snapshot returns the buffered bytes and how many leading bytes were
// dropped by the ring limit.
func (b *jobBuffer) snapshot() ([]byte, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out, b.dropped
}

// Job is one background command. Finished/ExitErr are written once by the
// wait goroutine under mu.
type Job struct {
	ID      string
	Command string
	Started time.Time

	mu       sync.Mutex
	finished bool
	exitErr  error

	buf *jobBuffer
	cmd *exec.Cmd
}

func (j *Job) state() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.finished {
		return "finished"
	}
	return "running"
}

func (j *Job) exit() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.finished {
		return ""
	}
	if j.exitErr != nil {
		return j.exitErr.Error()
	}
	return "exit 0"
}

func (j *Job) markFinished(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.finished = true
	j.exitErr = err
}

// JobInfo is a read-only view for job_list.
type JobInfo struct {
	ID      string
	Command string
	State   string
	Exit    string
	Started time.Time
	Bytes   int
}

// JobRegistry tracks every background job of the host.
type JobRegistry struct {
	mu   sync.Mutex
	seq  int
	jobs map[string]*Job
}

// Jobs is the host-level registry.
var Jobs = &JobRegistry{jobs: map[string]*Job{}}

// Start runs command in the background and returns its job id.
func (r *JobRegistry) Start(command, cwd string) (string, error) {
	cmd := exec.Command("bash", "-c", command) //nolint:gosec
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	buf := &jobBuffer{}
	cmd.Stdout = buf
	cmd.Stderr = buf
	r.mu.Lock()
	r.seq++
	id := fmt.Sprintf("job-%d", r.seq)
	job := &Job{ID: id, Command: command, Started: time.Now(), buf: buf, cmd: cmd}
	r.jobs[id] = job
	r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		r.mu.Lock()
		delete(r.jobs, id)
		r.mu.Unlock()
		return "", err
	}
	go func() {
		err := cmd.Wait()
		job.markFinished(err)
	}()
	return id, nil
}

// List returns every job, oldest first.
func (r *JobRegistry) List() []JobInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.jobs))
	for id := range r.jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]JobInfo, 0, len(ids))
	for _, id := range ids {
		j := r.jobs[id]
		data, dropped := j.buf.snapshot()
		out = append(out, JobInfo{
			ID: j.ID, Command: j.Command, State: j.state(), Exit: j.exit(),
			Started: j.Started, Bytes: dropped + len(data),
		})
	}
	return out
}

// Output returns the job's output from offset (up to the configured output
// cap) plus its state and the next offset to read from.
func (r *JobRegistry) Output(id string, offset int) (text, state string, next int, err error) {
	r.mu.Lock()
	j, ok := r.jobs[id]
	r.mu.Unlock()
	if !ok {
		return "", "", 0, fmt.Errorf("未知任务 %s（job_list 查看全部）", id)
	}
	data, dropped := j.buf.snapshot()
	total := dropped + len(data)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	start := offset - dropped // 落在已丢弃区间则从缓冲头开始
	if start < 0 {
		start = 0
	}
	end := start + OutputCapBytes
	if end > len(data) {
		end = len(data)
	}
	text = string(data[start:end])
	next = dropped + end
	state = j.state()
	if ex := j.exit(); ex != "" {
		state += "（" + ex + "）"
	}
	switch {
	case next < total:
		text += fmt.Sprintf("\n…[输出共 %d 字节，从 %d 处截断；用 offset=%d 继续]", total, offset, next)
	case offset >= total:
		text = "(没有更多输出)"
	}
	return text, state, next, nil
}

// Kill terminates the job's whole process group.
func (r *JobRegistry) Kill(id string) error {
	r.mu.Lock()
	j, ok := r.jobs[id]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("未知任务 %s（job_list 查看全部）", id)
	}
	if j.state() == "finished" {
		return fmt.Errorf("任务 %s 已结束", id)
	}
	return syscall.Kill(-j.cmd.Process.Pid, syscall.SIGKILL)
}
