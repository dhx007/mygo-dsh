package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	dataDirOverride = dir
	t.Cleanup(func() { dataDirOverride = "" })
	a := newApp()
	a.cfg.WorkDir = t.TempDir() // sessions group under a stable fake workspace
	a.fWorkDir = a.cfg.WorkDir
	return a
}

// mockDeepSeek returns an SSE server that answers every chat completion
// with a two-chunk reply carrying the given content.
func mockDeepSeek(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		chunks := []string{content[:len(content)/2], content[len(content)/2:]}
		for _, c := range chunks {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]any{"content": c}}},
			})
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// The full first-run flow: welcome → new session → type → send → reply.
func TestNewSessionAndSend(t *testing.T) {
	a := newTestApp(t)
	srv := mockDeepSeek(t, "你好，我是测试回复。")
	a.cfg.BaseURL = srv.URL
	a.cfg.APIKey = "sk-test"
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	if !tt.HasText("DeepSeek Harness") {
		t.Fatalf("welcome missing: %v", tt.Texts())
	}

	// 新建会话
	if err := tt.Click("＋ 新建会话"); err != nil {
		t.Fatalf("click 新建会话: %v", err)
	}
	tt.Frame()
	if a.ses == nil {
		t.Fatal("session was not created")
	}
	if a.page != "chat" {
		t.Errorf("page = %q, want chat", a.page)
	}

	// 输入（composer 有 AutoFocus，直接打字）
	tt.Type("你好")
	if a.input != "你好" {
		t.Fatalf("composer input = %q, typing did not land", a.input)
	}

	// 发送
	if err := tt.Click("发送"); err != nil {
		t.Fatalf("click 发送: %v", err)
	}
	waitFor(t, func() bool { return !a.busy }, "send never finished")

	var reply string
	for _, m := range a.msgs {
		if m.role == "assistant" {
			reply = m.text
		}
	}
	if !strings.Contains(reply, "测试回复") {
		t.Errorf("assistant reply = %q", reply)
	}
	evs := a.ses.Events()
	if len(evs) < 3 { // session + user + assistant(+tool…)
		t.Errorf("trajectory events = %d, want >= 3", len(evs))
	}
}

// Settings: type an API key and save; the file must contain it.
func TestSettingsSaveKey(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open settings: %v", err)
	}
	tt.Frame()
	if a.page != "settings" {
		t.Fatalf("page = %q, want settings", a.page)
	}
	if err := tt.Click("API Key 输入框"); err != nil {
		t.Fatalf("focus key input: %v (texts=%v)", err, tt.Texts())
	}
	tt.Type("sk-test-abc")
	if a.fKey != "sk-test-abc" {
		t.Fatalf("fKey = %q after typing", a.fKey)
	}
	if err := tt.Click("保存设置"); err != nil {
		t.Fatalf("click 保存设置: %v", err)
	}
	if a.cfg.APIKey != "sk-test-abc" {
		t.Errorf("cfg.APIKey = %q", a.cfg.APIKey)
	}
	b, err := os.ReadFile(a.cfgPath)
	if err != nil || !strings.Contains(string(b), "sk-test-abc") {
		t.Errorf("config file not saved: err=%v body=%s", err, b)
	}
}

// The sidebar groups sessions under their workspace and reloads them.
func TestSidebarReloadSession(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	if err := tt.Click("＋ 新建会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	id := a.ses.Meta.ID

	// 走到设置页再回来，会话仍应加载且侧栏有分组
	if err := tt.Click("轨迹"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != "chat" || a.ses == nil || a.ses.Meta.ID != id {
		t.Fatalf("back to chat: page=%q ses=%v", a.page, a.ses)
	}
	if !tt.HasText("新会话") {
		t.Errorf("session title not visible: %v", tt.Texts())
	}
	// meta 落盘
	b, err := os.ReadFile(filepath.Join(a.store.Root, "s", id, "meta.json"))
	if err != nil || !strings.Contains(string(b), id) {
		t.Errorf("meta.json missing: %v", err)
	}
}
