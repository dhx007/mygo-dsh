package main

import (
	"context"
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

	"mygo-harness/internal/tools"
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
	if err := tt.Click("⊕ 新会话"); err != nil {
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
	if err := tt.Click("↑"); err != nil {
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

	if err := tt.Click("用户菜单"); err != nil {
		t.Fatalf("open user menu: %v", err)
	}
	tt.Frame()
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("click 设置 in user menu: %v (texts=%v)", err, tt.Texts())
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

	if err := tt.Click("⊕ 新会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	id := a.ses.Meta.ID

	// 走到设置页再回来，会话仍应加载且侧栏有分组
	if err := tt.Click("轨迹"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("对话"); err != nil {
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

// REQ-001 AC-5: 极简 mounts the persistent shell, 标准 keeps one-shot bash.
func TestPersistentShellPresetWiring(t *testing.T) {
	a := newTestApp(t)

	ses, err := a.store.New(a.cfg.WorkDir, "极简", "minimal-session")
	if err != nil {
		t.Fatal(err)
	}
	ag := a.newAgentFor(ses)
	if ag.Shell == nil {
		t.Fatal("极简 preset did not mount a persistent shell")
	}
	ag.Shell.Close()

	ses2, err := a.store.New(a.cfg.WorkDir, "标准", "standard-session")
	if err != nil {
		t.Fatal(err)
	}
	if ag2 := a.newAgentFor(ses2); ag2.Shell != nil {
		t.Fatal("标准 preset should keep the one-shot bash")
	}
}

// REQ-003 AC-4: the settings 技能 section lists discovered skills with
// source badges; the empty state guides where to drop SKILL.md packs.
func TestSettingsSkillsSection(t *testing.T) {
	a := newTestApp(t)
	g, _ := skillRoots(a.dataDir, a.cfg.WorkDir)
	writeSkill(t, g, "deploy", "description: 发布流程", "步骤一")
	a.setSection = "技能"
	tt := ui.NewTester(a.settingsWindowView, 960, 700)
	tt.Frame()
	for _, want := range []string{"已发现的技能", "deploy", "发布流程", "全局"} {
		if !tt.HasText(want) {
			t.Errorf("技能 section missing %q (texts=%v)", want, tt.Texts())
		}
	}

	a2 := newTestApp(t)
	a2.setSection = "技能"
	tt2 := ui.NewTester(a2.settingsWindowView, 960, 700)
	tt2.Frame()
	if !tt2.HasText("未发现技能") {
		t.Errorf("empty state missing (texts=%v)", tt2.Texts())
	}
}

// REQ-005 AC-5: the MCP section renders servers (add form, card, two-step
// delete) and persists to mcp.json.
func TestPluginsPageMCP(t *testing.T) {
	a := newTestApp(t)
	a.selectedID = "page:plugins"
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	a.mcpNameInput = "sqlite"
	a.mcpCmdInput = "uvx mcp-server-sqlite --db test.db"
	a.addMCPServer()
	if a.errText != "" {
		t.Fatalf("add failed: %s", a.errText)
	}
	a.pluginQuery = "zzz" // 收缩内置清单，让 MCP 段进入视口
	tt.Frame()
	if !tt.HasText("MCP 服务器 · 1") || !tt.HasText("sqlite") || !tt.HasText("添加服务器") {
		t.Errorf("MCP section missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("sqlite"); err != nil {
		t.Fatalf("expand server: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("加载工具清单") || !tt.HasText("删除") {
		t.Errorf("server detail missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("再次点击确认删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(loadMCPServers(a.dataDir)) != 0 {
		t.Errorf("server still configured after delete")
	}
}

// REQ-006 AC-1/4: the market section is gated by the experimental switch;
// adding a local index and refreshing lists entries; installing one puts
// the package into the installed section.
func TestPluginsPageMarket(t *testing.T) {
	a := newTestApp(t)
	a.selectedID = "page:plugins"
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	a.pluginQuery = "zzz" // 收缩内置清单
	tt.Frame()
	if tt.HasText("插件市场 ·") {
		t.Error("market section should be hidden before the switch")
	}
	a.setExperimental("custom_plugin_markets", true)
	tt.Frame()
	if !tt.HasText("自定义插件市场") || !tt.HasText("插件市场 · 0 索引") {
		t.Errorf("market section missing after switch (texts=%v)", tt.Texts())
	}

	// 本地 git 仓库作为 origin，本地索引文件
	repo := t.TempDir()
	makeGitPluginRepo(t, repo, "mkt-pack")
	idx := filepath.Join(t.TempDir(), "market.json")
	if err := os.WriteFile(idx, []byte(`{"name":"t","plugins":[
		{"id":"mkt-pack","name":"市场包","version":"1.0.0","description":"来自市场","source":"git","origin":"`+repo+`"}
	]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.marketInput = idx
	a.addMarketIndex()
	if a.errText != "" {
		t.Fatalf("add index failed: %s", a.errText)
	}
	waitFor(t, func() bool { return len(a.marketEntries) == 1 }, "market fetch never finished")
	tt.Frame()
	if !tt.HasText("市场包") || !tt.HasText("v1.0.0") || !tt.HasText("刷新市场") {
		t.Errorf("market entries missing (texts=%v)", tt.Texts())
	}

	// 安装 → 已安装段出现，条目卡变为「已安装」
	if err := tt.Click("安装"); err != nil {
		t.Fatalf("click 安装: %v (texts=%v)", err, tt.Texts())
	}
	waitFor(t, func() bool { return len(a.pkgCache) == 1 }, "install never finished")
	tt.Frame()
	if !tt.HasText("已安装") {
		t.Errorf("installed marker missing (texts=%v)", tt.Texts())
	}
}

// REQ-007 AC-4: the Shell card detail exposes timeout/output-cap fields;
// valid changes persist to config.json and apply to the tools package,
// invalid input shows a hint.
func TestShellCardSettings(t *testing.T) {
	a := newTestApp(t)
	a.selectedID = "page:plugins"
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	a.pluginQuery = "Shell" // 只剩 Shell 终端一张卡
	tt.Frame()
	if err := tt.Click("Shell 终端"); err != nil {
		t.Fatalf("expand shell card: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("命令超时 (ms)") || !tt.HasText("输出上限 (bytes)") {
		t.Errorf("shell settings missing (texts=%v)", tt.Texts())
	}

	a.fShellTimeout = "30000"
	a.fShellCap = "1024"
	tt.Frame()
	if a.cfg.ShellTimeoutMS != 30000 || a.cfg.ShellOutputCap != 1024 {
		t.Fatalf("config not saved: %+v", a.cfg)
	}
	b, err := os.ReadFile(a.cfgPath)
	if err != nil || !strings.Contains(string(b), `"shell_timeout_ms": 30000`) {
		t.Errorf("config file not persisted: err=%v body=%s", err, b)
	}
	if tools.DefaultTimeout != 30*time.Second || tools.OutputCapBytes != 1024 {
		t.Errorf("tools vars not synced: %v %v", tools.DefaultTimeout, tools.OutputCapBytes)
	}

	a.fShellTimeout = "abc"
	tt.Frame()
	if !tt.HasText("需为整数") {
		t.Errorf("invalid hint missing (texts=%v)", tt.Texts())
	}
}

// REQ-001 AC-6: dropping the agent releases the shell process; Close is idempotent.
func TestDropAgentClosesShell(t *testing.T) {
	a := newTestApp(t)
	ses, err := a.store.New(a.cfg.WorkDir, "极简", "s")
	if err != nil {
		t.Fatal(err)
	}
	a.ag = a.newAgentFor(ses)
	sh := a.ag.Shell
	if out, failed := sh.Run(context.Background(), `{"command":"echo hi"}`); failed {
		t.Fatalf("shell did not start: %s", out)
	}
	if !sh.Running() {
		t.Fatal("shell not running after first call")
	}
	a.dropAgent()
	if sh.Running() {
		t.Fatal("shell still running after dropAgent")
	}
	sh.Close() // must not panic
}

// REQ-004 AC-1: the plugins page shows the three sections; search filters
// the built-in list; cards expand to their detail with the switch.
func TestPluginsPageSections(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	a.selectedID = "page:plugins"
	tt.Frame()
	if !tt.HasText("内置插件 · 16") || !tt.HasText("Shell 终端") {
		t.Errorf("builtin section missing (texts=%v)", tt.Texts())
	}

	// 搜索过滤到 1 项 → 其余分区块上移进入视口
	a.pluginQuery = "读文件"
	tt.Frame()
	if !tt.HasText("内置插件 · 1") || !tt.HasText("实验插件") || !tt.HasText("持久终端") || !tt.HasText("已安装插件 · 0") {
		t.Errorf("filtered sections missing (texts=%v)", tt.Texts())
	}

	// 展开详情
	if err := tt.Click("读文件"); err != nil {
		t.Fatalf("expand card: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("挂载工具：read_file") || !tt.HasText("v0.1.0") {
		t.Errorf("expanded detail missing (texts=%v)", tt.Texts())
	}
}

// REQ-004 AC-3/FR-3: an installed package renders as a card with child
// tools and uninstall; installPackage copies the source dir.
func TestPluginsPageInstalledPackage(t *testing.T) {
	a := newTestApp(t)
	a.selectedID = "page:plugins"
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Frame()

	src := t.TempDir()
	writePluginSource(t, src, "demo-pack")
	a.installInput = src
	a.installPackage()
	if a.errText != "" {
		t.Fatalf("install failed: %s", a.errText)
	}
	a.pluginQuery = "zzz" // 内置清单过滤为空，让已安装段进入视口
	tt.Frame()
	if !tt.HasText("已安装插件 · 1") || !tt.HasText("示例包") || !tt.HasText("v1.0.0") {
		t.Errorf("package card missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("示例包"); err != nil {
		t.Fatalf("expand package: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("demo_hello") || !tt.HasText("卸载") {
		t.Errorf("package detail missing (texts=%v)", tt.Texts())
	}
	// 两步卸载
	if err := tt.Click("卸载"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("再次点击确认卸载") {
		t.Errorf("uninstall confirm missing (texts=%v)", tt.Texts())
	}
	if err := tt.Click("再次点击确认卸载"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.pkgCache) != 0 {
		t.Errorf("package still installed after uninstall: %+v", a.pkgCache)
	}
}

// REQ-004 AC-2: the persistent terminals experimental switch mounts the
// persistent shell for every bash preset; 极简 always has it.
func TestPersistentTerminalsFlag(t *testing.T) {
	a := newTestApp(t)
	ses, err := a.store.New(a.cfg.WorkDir, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses); ag.Shell != nil {
		t.Error("标准 preset should not mount a shell by default")
	}
	a.setExperimental("persistent_terminals", true)
	if ag := a.newAgentFor(ses); ag.Shell == nil {
		t.Error("flag on but 标准 preset has no persistent shell")
	} else {
		ag.Shell.Close()
	}
	a.setExperimental("persistent_terminals", false)

	ses2, err := a.store.New(a.cfg.WorkDir, "极简", "s2")
	if err != nil {
		t.Fatal(err)
	}
	if ag := a.newAgentFor(ses2); ag.Shell == nil {
		t.Error("极简 preset must always mount the persistent shell")
	} else {
		ag.Shell.Close()
	}
}

// REQ-004 AC-5: installed package tools mount into the agent's toolset and
// respect package/child toggles.
func TestPackageToolAgentWiring(t *testing.T) {
	a := newTestApp(t)
	src := t.TempDir()
	writePluginSource(t, src, "demo-pack")
	if _, err := installFromPath(a.dataDir, src); err != nil {
		t.Fatal(err)
	}
	a.refreshPackages()

	ses, err := a.store.New(a.cfg.WorkDir, "标准", "s")
	if err != nil {
		t.Fatal(err)
	}
	ag := a.newAgentFor(ses)
	if len(ag.PackageSpecs) != 1 || ag.PackageSpecs[0].Function.Name != "demo_hello" {
		t.Fatalf("package spec missing: %+v", ag.PackageSpecs)
	}
	if ag.PackageTools["demo_hello"] == nil || ag.MutatingTools["demo_hello"] {
		t.Fatalf("runner/mutating wiring wrong (demo_hello 是只读工具): %+v", ag.MutatingTools)
	}
	if !strings.Contains(ag.PackageSpecs[0].Function.Description, "示例包") {
		t.Errorf("spec description should name the package: %q", ag.PackageSpecs[0].Function.Description)
	}
	// 子插件停用 → spec 消失
	a.setEnabled("demo-pack/demo_hello", false)
	if ag2 := a.newAgentFor(ses); len(ag2.PackageSpecs) != 0 {
		t.Fatalf("child toggle not respected: %+v", ag2.PackageSpecs)
	}
	// 恢复子插件、停用整包 → spec 消失
	a.setEnabled("demo-pack/demo_hello", true)
	a.setEnabled("demo-pack", false)
	if ag3 := a.newAgentFor(ses); len(ag3.PackageSpecs) != 0 {
		t.Fatalf("package toggle not respected: %+v", ag3.PackageSpecs)
	}
}

// REQ-001 AC-6: dropping the agent releases the shell process; Close is idempotent.

// REQ-004 AC-1: the settings 内置插件 section renders the same card list
// with its own search. Headless has no child window, so drive the
// production settingsWindowView directly.
func TestSettingsPluginsSectionGrouped(t *testing.T) {
	a := newTestApp(t)
	a.setSection = "内置插件"
	tt := ui.NewTester(a.settingsWindowView, 960, 700)
	tt.Frame()

	for _, want := range []string{"16 个内置插件", "Shell 终端", "读文件", "Enabled"} {
		if !tt.HasText(want) {
			t.Errorf("settings 内置插件 missing %q (texts=%v)", want, tt.Texts())
		}
	}
	a.setPluginQuery = "读文件"
	tt.Frame()
	if !tt.HasText("1 个内置插件") || tt.HasText("Shell 终端") {
		t.Errorf("settings search filter broken (texts=%v)", tt.Texts())
	}
}
