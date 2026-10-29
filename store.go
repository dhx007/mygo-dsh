package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mygo-harness/internal/llm"
)

// Session is one persisted conversation: a directory under the profiles
// root with meta.json, events.jsonl (the append-only Trajectory) and
// history.json (the resumable message list).
type Session struct {
	Meta SessionMeta `json:"-"`

	dir string
}

type SessionMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Mode      string `json:"mode"`
	// Model is the session-level model override; empty follows the global
	// default (config.json), the original's per-session model selection.
	Model     string    `json:"model,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages the profiles root directory.
type Store struct {
	Root string
}

func newStore(root string) *Store { return &Store{Root: root} }

func newSessionID() string {
	return time.Now().Format("20060102-150405") + "-" + fmt.Sprintf("%04x", time.Now().UnixNano()&0xffff)
}

// New creates a session directory and writes its meta.
func (s *Store) New(workspace, mode, title string) (*Session, error) {
	ses := &Session{
		Meta: SessionMeta{
			ID:        newSessionID(),
			Title:     title,
			Workspace: workspace,
			Mode:      mode,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		dir: filepath.Join(s.Root, "s"),
	}
	ses.dir = filepath.Join(s.Root, "s", ses.Meta.ID)
	if err := os.MkdirAll(ses.dir, 0o755); err != nil {
		return nil, err
	}
	if err := ses.saveMeta(); err != nil {
		return nil, err
	}
	return ses, nil
}

// List loads every session meta, newest first.
func (s *Store) List() []*SessionMeta {
	entries, err := os.ReadDir(filepath.Join(s.Root, "s"))
	if err != nil {
		return nil
	}
	var out []*SessionMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Root, "s", e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var m SessionMeta
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		out = append(out, &m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

// Search filters sessions by query against title, workspace and trajectory
// content (files are capped so the scan stays quick).
func (s *Store) ListFiltered(query string) []*SessionMeta {
	all := s.List()
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all
	}
	var out []*SessionMeta
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Title), q) || strings.Contains(strings.ToLower(m.Workspace), q) || strings.Contains(strings.ToLower(m.Mode), q) {
			out = append(out, m)
			continue
		}
		if f, err := os.Open(filepath.Join(s.Root, "s", m.ID, "events.jsonl")); err == nil { //nolint:gosec
			sc := bufio.NewScanner(f) //nolint:gosec
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			hits := false
			for i := 0; i < 4000 && sc.Scan(); i++ {
				if strings.Contains(strings.ToLower(sc.Text()), q) {
					hits = true
					break
				}
			}
			f.Close()
			if hits {
				out = append(out, m)
			}
		}
	}
	return out
}

// Open loads a session directory.
func (s *Store) Open(id string) (*Session, error) {
	dir := filepath.Join(s.Root, "s", filepath.Clean("/"+id))
	meta := filepath.Join(dir, "meta.json")
	b, err := os.ReadFile(meta) //nolint:gosec
	if err != nil {
		return nil, err
	}
	ses := &Session{dir: dir}
	if err := json.Unmarshal(b, &ses.Meta); err != nil {
		return nil, err
	}
	return ses, nil
}

func (s *Session) saveMeta() error {
	s.Meta.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(&s.Meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "meta.json"), b, 0o644)
}

// Retitle renames the session and bumps its update time.
func (s *Session) Retitle(title string) {
	s.Meta.Title = title
	_ = s.saveMeta()
}

// Touch marks the session as recently active.
func (s *Session) Touch() { _ = s.saveMeta() }

// Delete removes the session directory.
func (s *Session) Delete() error { return os.RemoveAll(s.dir) }

// Fork copies the whole session into a new one titled "(fork)".
func (s *Store) Fork(src *Session) (*Session, error) {
	dst, err := s.New(src.Meta.Workspace, src.Meta.Mode, src.Meta.Title+" (fork)")
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"events.jsonl", "history.json"} {
		if b, err := os.ReadFile(filepath.Join(src.dir, name)); err == nil { //nolint:gosec
			_ = os.WriteFile(filepath.Join(dst.dir, name), b, 0o644)
		}
	}
	return dst, nil
}

// ---- Trajectory (events.jsonl, append-only) ----

type Event struct {
	Time time.Time      `json:"t"`
	Kind string         `json:"kind"`
	Data map[string]any `json:"data"`
}

// AppendEvent adds one line to the Trajectory log.
func (s *Session) AppendEvent(kind string, data map[string]any) {
	if s.dir == "" {
		return
	}
	ev := Event{Time: time.Now(), Kind: kind, Data: data}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(s.dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// Events reads the whole Trajectory, oldest first.
func (s *Session) Events() []Event {
	f, err := os.Open(filepath.Join(s.dir, "events.jsonl")) //nolint:gosec
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f) //nolint:gosec
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var ev Event
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			out = append(out, ev)
		}
	}
	return out
}

// ---- History (history.json) ----

// SaveHistory persists the resumable message list.
func (s *Session) SaveHistory(msgs []llm.Message) {
	b, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(s.dir, "history.json"), b, 0o644)
}

// LoadHistory reads back the message list.
func (s *Session) LoadHistory() []llm.Message {
	b, err := os.ReadFile(filepath.Join(s.dir, "history.json")) //nolint:gosec
	if err != nil {
		return nil
	}
	var msgs []llm.Message
	if json.Unmarshal(b, &msgs) != nil {
		return nil
	}
	return msgs
}
