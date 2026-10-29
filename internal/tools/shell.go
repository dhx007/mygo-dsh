package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// PersistentShell keeps one long-running bash process so the working
// directory, exported variables and shell options survive across calls —
// the persistent shell the original harness ships for its minimal preset
// (dsh-tool-bash-persistent). Commands go in over stdin; the result is
// collected until a per-call nonce marker line, so partial lines and
// model output can never be mistaken for the terminator. A dead or
// timed-out shell is torn down (whole process group) and transparently
// restarted on the next call.
type PersistentShell struct {
	mu    sync.Mutex
	dir   string
	env   []string
	n     int // nonce counter
	cmd   *exec.Cmd
	stdin io.WriteCloser
	outR  *os.File
	buf   *bufio.Reader
}

// NewPersistentShell returns an idle shell that will start bash in dir on
// first use. Call Close to release the process when the owning agent is
// discarded.
func NewPersistentShell(dir string) *PersistentShell {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return &PersistentShell{dir: dir, env: os.Environ()}
}

func (s *PersistentShell) start() error {
	cmd := exec.Command("bash", "--noprofile", "--norc")
	cmd.Dir = s.dir
	cmd.Env = s.env
	// Own process group: a timeout must take down command trees (make,
	// npm run …), not just the bash that spawned them.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	r, w, err := os.Pipe()
	if err != nil {
		stdin.Close()
		return err
	}
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		stdin.Close()
		r.Close()
		w.Close()
		return err
	}
	// The child holds the write end now; ours must not block EOF.
	w.Close()
	s.cmd = cmd
	s.stdin = stdin
	s.outR = r
	s.buf = bufio.NewReaderSize(r, 32*1024)
	return nil
}

// killLocked tears the current process down; next Run starts fresh.
func (s *PersistentShell) killLocked() {
	if s.cmd != nil {
		if s.cmd.Process != nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
			_, _ = s.cmd.Process.Wait()
		}
		_ = s.cmd.Wait()
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.outR != nil {
		_ = s.outR.Close()
	}
	s.cmd, s.stdin, s.outR, s.buf = nil, nil, nil, nil
}

// Close releases the shell process. Safe to call repeatedly.
func (s *PersistentShell) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.killLocked()
}

// Running reports whether a bash process is alive right now (used by tests
// and for status display; lazily started shells report false before use).
func (s *PersistentShell) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil
}

// Run executes one command in the persistent shell. argsJSON matches the
// one-shot bash tool: {"command": string, "timeout_seconds"?: int}.
func (s *PersistentShell) Run(ctx context.Context, argsJSON string) (string, bool) {
	var a struct {
		Command   string `json:"command"`
		TimeoutOK int    `json:"timeout_seconds"`
	}
	if err := args(context.Background(), argsJSON, &a); err != nil {
		return err.Error(), true
	}
	if strings.TrimSpace(a.Command) == "" {
		return "error: command is empty", true
	}
	timeout := DefaultTimeout
	if a.TimeoutOK > 0 {
		timeout = time.Duration(min(a.TimeoutOK, 600)) * time.Second
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for attempt := 0; attempt < 2; attempt++ {
		if s.cmd == nil {
			if err := s.start(); err != nil {
				return "error: 启动持久 bash: " + err.Error(), true
			}
		}
		s.n++
		end := fmt.Sprintf("__DSH_END_%d_%x:__", s.n, time.Now().UnixNano()&0xffffffff)
		payload := a.Command + "\n" +
			"__dsh_rc=$?\n" +
			"printf '\\n%s%d\\n' '" + end + "' \"$__dsh_rc\"\n"
		if _, err := io.WriteString(s.stdin, payload); err != nil {
			// Write failed before the command could run: restart and try
			// once in a fresh shell.
			s.killLocked()
			continue
		}
		out, rc, dead, timedOut := s.collectLocked(ctx, end, timeout)
		switch {
		case timedOut:
			s.killLocked()
			if ctx.Err() != nil {
				return "[canceled]", true
			}
			return truncateWithSpill(out, OutputCapBytes) + fmt.Sprintf("\n[exit] 命令超时（%s），持久 shell 已重置，下次调用将在新 shell 中执行", timeout), true
		case dead:
			// The shell itself is gone (exit, kill $$, crash). Never
			// re-run the command — it may have had side effects.
			s.killLocked()
			return truncateWithSpill(out, OutputCapBytes) + "\n[exit] bash 会话已退出（执行了 exit 或进程终止）；状态已清零，下次调用将自动重启新 shell", true
		default:
			if rc != 0 {
				out += fmt.Sprintf("\n[exit] rc=%d", rc)
			}
			if out == "" {
				out = "(no output)"
			}
			return truncateWithSpill(out, OutputCapBytes), rc != 0
		}
	}
	return "error: 持久 bash 无法接收命令，已重置", true
}

// collectLocked reads output lines until the marker, the deadline or EOF.
// A single mutex spans write+read, so calls never interleave.
func (s *PersistentShell) collectLocked(ctx context.Context, end string, timeout time.Duration) (out string, rc int, dead, timedOut bool) {
	// Canceling the context must break the blocking read immediately.
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = s.outR.SetReadDeadline(time.Now())
		case <-watchDone:
		}
	}()
	defer close(watchDone)

	_ = s.outR.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = s.outR.SetReadDeadline(time.Time{}) }()

	var lines []string
	for {
		line, err := s.buf.ReadString('\n')
		if err != nil {
			if line != "" {
				lines = append(lines, line)
			}
			if errors.Is(err, os.ErrDeadlineExceeded) && ctx.Err() == nil {
				timedOut = true
			} else {
				dead = true
			}
			break
		}
		if strings.HasPrefix(line, end) {
			if v, perr := strconv.Atoi(strings.TrimSpace(line[len(end):])); perr == nil {
				rc = v
			} else {
				dead = true
			}
			break
		}
		lines = append(lines, line)
	}
	// The marker's leading newline leaves one empty line; drop it.
	if n := len(lines); n > 0 && strings.TrimRight(lines[n-1], "\n") == "" && !timedOut && !dead {
		lines = lines[:n-1]
	}
	for _, l := range lines {
		out += l
	}
	return strings.TrimRight(out, "\n"), rc, dead, timedOut
}
