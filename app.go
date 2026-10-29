package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"mygo-harness/internal/agent"
	"mygo-harness/internal/llm"
	"mygo-harness/internal/tools"
)

// ---- icons (Lucide-style strokes, as the original's icon set) ----

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	icoChat   = icon(`<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`)
	icoTraj   = icon(`<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>`)
	icoPlugin = icon(`<path d="M12 22a2 2 0 0 1-2-2v-1a1 1 0 0 0-1-1H8a2 2 0 0 1-2-2v-1a1 1 0 0 0-1-1H4a2 2 0 0 1-2-2 2 2 0 0 1 2-2h1a1 1 0 0 0 1-1V7a2 2 0 0 1 2-2h1a1 1 0 0 0 1-1 2 2 0 0 1 2-2 2 2 0 0 1 2 2 1 1 0 0 0 1 1h1a2 2 0 0 1 2 2v1a1 1 0 0 0 1 1h1a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-1a1 1 0 0 0-1 1v1a2 2 0 0 1-2 2h-1a1 1 0 0 0-1 1v1a2 2 0 0 1-2 2z"/>`)
	icoGear   = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>`)
	icoWrench = icon(`<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>`)
	icoBot    = icon(`<rect x="4" y="8" width="16" height="12" rx="2"/><path d="M12 8V4M8 4h8M9 14h.01M15 14h.01"/>`)
	icoList   = icon(`<path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>`)
	icoFolder = icon(`<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>`)
	icoPlus   = icon(`<path d="M5 12h14M12 5v14"/>`)
	icoUp     = icon(`<path d="M18 15l-6-6-6 6"/>`)
	icoDown   = icon(`<path d="M6 9l6 6 6-6"/>`)
	icoUser   = icon(`<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>`)
	icoGlobe  = icon(`<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>`)
	icoPal    = icon(`<circle cx="13.5" cy="6.5" r=".5"/><circle cx="17.5" cy="10.5" r=".5"/><circle cx="8.5" cy="7.5" r=".5"/><circle cx="6.5" cy="12.5" r=".5"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.926 0 1.648-.746 1.648-1.688 0-.437-.18-.835-.437-1.125-.29-.289-.438-.652-.438-1.125a1.64 1.64 0 0 1 1.668-1.668h1.996c3.051 0 5.555-2.503 5.555-5.554C21.965 6.012 17.461 2 12 2z"/>`)
	icoArc    = icon(`<rect x="2" y="3" width="20" height="5" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/>`)
	icoHeart  = icon(`<path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/>`)
)

// ---- chat list items ----

type toolView struct {
	id      string
	name    string
	args    string
	result  string
	running bool
	failed  bool
	open    bool
}

type uiMsg struct {
	id            int
	role          string // user | assistant | tool | todo | subagent
	text          string
	reasoning     string
	reasoningOpen bool
	tool          *toolView
	todos         []agent.TodoItem
	desc          string
	done          bool
}

type pendingApproval struct {
	tool string
	args string
	ch   chan agent.Decision
}

// pump batches streaming deltas into at most one repaint per tick.
type pump struct {
	win *mygo.Window
	mu  sync.Mutex
	fns []func()
	t   *time.Timer
}

func newPump(w *mygo.Window) *pump { return &pump{win: w} }

func (p *pump) add(fn func()) {
	p.mu.Lock()
	p.fns = append(p.fns, fn)
	if p.t == nil {
		p.t = time.AfterFunc(60*time.Millisecond, p.flush)
	}
	p.mu.Unlock()
}

func (p *pump) flush() {
	p.mu.Lock()
	fns := p.fns
	p.fns = nil
	p.t = nil
	p.mu.Unlock()
	if len(fns) == 0 {
		return
	}
	if p.win == nil { // headless (tests): run inline
		for _, f := range fns {
			f()
		}
		return
	}
	p.win.Update(func() {
		for _, f := range fns {
			f()
		}
	})
}

func (p *pump) stop() { p.flush() }

// app is the whole UI state; mutations happen on the main thread, either
// inside the view (clicks) or through Window.Update from goroutines.
type app struct {
	win     *mygo.Window
	cfg     *Config
	cfgPath string
	theme   *ui.Theme

	store   *Store
	dataDir string
	presets []Preset

	// current session
	ses  *Session
	msgs []*uiMsg

	list   ui.ListState
	md     markdown.Options
	nextID int

	input   string
	busy    bool
	status  string
	errText string

	ag     *agent.Agent
	cancel context.CancelFunc
	pump   *pump

	toolNames     map[string]string
	focusComposer int          // frames left to hand keyboard focus back to the composer
	savedAt       time.Time    // last successful settings save
	userMenu      bool         // user-row popup menu open
	settingsWin   *mygo.Window // the settings child window, if open
	setSection    string       // settings window: selected section label

	// 账号与余额（DeepSeek /user/balance）
	balLoading              bool
	balFetched              bool
	balErr                  string
	balTotal, balTopped     string
	balGranted, balCurrency string

	// 使用统计
	statsLoading, statsLoaded  bool
	statsSessions, statsEvents int

	setList ui.ListState // settings window content scroll

	pending     *pendingApproval
	alwaysAllow map[string]bool

	// navigation & sidebar
	page       string // chat | trajectory | plugins | settings
	sessions   []*SessionMeta
	query      string
	selectedID string
	loadedID   string
	wsOpen     map[string]*bool
	navOpen    bool
	confirmDel bool

	// trajectory page
	traj     []Event
	trajList ui.ListState
	trajOpen map[int]*bool

	// plugins page
	pluginList ui.ListState
	// REQ-004：内置清单搜索（页/设置各记一份）、详情展开、安装输入、卸载确认
	pluginQuery      string
	setPluginQuery   string
	pluginOpen       map[string]*bool
	pkgOpen          map[string]*bool
	installInput     string
	confirmUninstall string
	pkgCache         []PluginPackage

	// settings page
	settingsList                 ui.ListState
	fKey, fURL, fModel, fWorkDir string
	autoApprove                  bool

	// preset editor
	epOpen   bool
	epLabel  string
	epPrompt string
}

// dataDirOverride lets tests redirect all storage away from the real
// user directories.
var dataDirOverride string

func newApp() *app {
	cfg, path := loadConfig()
	if dataDirOverride != "" {
		path = filepath.Join(dataDirOverride, "config.json")
	}
	a := &app{
		cfg:           cfg,
		cfgPath:       path,
		theme:         dshTheme(cfg.Theme),
		page:          "chat",
		alwaysAllow:   map[string]bool{},
		toolNames:     map[string]string{},
		wsOpen:        map[string]*bool{},
		navOpen:       true,
		trajOpen:      map[int]*bool{},
		pluginOpen:    map[string]*bool{},
		pkgOpen:       map[string]*bool{},
		focusComposer: 1,
		fKey:          cfg.APIKey,
		fURL:          cfg.BaseURL,
		fModel:        cfg.Model,
		fWorkDir:      cfg.WorkDir,
		autoApprove:   cfg.AutoApprove,
	}
	dir := dataDirOverride
	if dir == "" {
		dir = userDataDir()
	}
	if dir == "" {
		if base, err := os.UserConfigDir(); err == nil {
			dir = filepath.Join(base, "mygo-harness")
		} else {
			dir = ".mygo-harness"
		}
	}
	a.dataDir = dir
	a.store = newStore(filepath.Join(dir, "profiles"))
	a.presets = loadPresets(dir)
	a.refreshSessions()
	a.refreshPackages()
	return a
}

func (a *app) refreshSessions() { a.sessions = a.store.List() }

// refreshPackages rescans installed plugin packages (startup + install/uninstall).
func (a *app) refreshPackages() { a.pkgCache = installedPackages(a.dataDir) }

// boolRef lazily allocates a toggle for detail-expand state.
func (a *app) boolRef(m map[string]*bool, key string) *bool {
	if b, ok := m[key]; ok {
		return b
	}
	v := false
	m[key] = &v
	return &v
}

// setExperimental persists an experimental switch (REQ-004 FR-2).
func (a *app) setExperimental(key string, on bool) {
	if a.cfg.Experimental == nil {
		a.cfg.Experimental = map[string]bool{}
	}
	a.cfg.Experimental[key] = on
	_ = saveConfig(a.cfg, a.cfgPath)
}

// onMain runs fn on the UI thread; headless tests (no window) run inline.
func (a *app) onMain(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

func (a *app) enabledTool(name string) bool { return a.cfg.enabledTool(name) }

func (a *app) experimentalOn(key string) bool { return a.cfg.experimentalOn(key) }

// currentPreset resolves the session's (or default) preset.
func (a *app) currentPreset() *Preset {
	mode := a.cfg.Mode
	if a.ses != nil && a.ses.Meta.Mode != "" {
		mode = a.ses.Meta.Mode
	}
	return presetByID(a.presets, mode)
}

// ---- view ----

func (a *app) view(c *ui.Context) {
	c.SetTheme(a.theme)
	a.md.Reveal = &markdown.Reveal{Unit: markdown.ByWord}
	if strings.HasPrefix(a.selectedID, "page:") {
		a.page = strings.TrimPrefix(a.selectedID, "page:")
	} else if a.selectedID != "" && a.loadedID != a.selectedID {
		a.loadSession(a.selectedID)
	}
	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).FillHeight().Background(c.Theme().Background).Children(func() {
			switch a.page {
			case "plugins":
				a.pluginsView(c)
			case "settings":
				a.settingsView(c)
			default:
				a.chatShell(c)
			}
		})
	})
}

// ---- sidebar ----

type wsGroup struct {
	workspace string
	items     []*SessionMeta
}

func (a *app) visibleSessions() []*SessionMeta { return a.store.ListFiltered(a.query) }

func (a *app) wsOpenFor(ws string) *bool {
	if p, ok := a.wsOpen[ws]; ok {
		return p
	}
	v := true
	a.wsOpen[ws] = &v
	return &v
}

func shortWS(ws string) string {
	ws = strings.TrimRight(ws, "/")
	if i := strings.LastIndex(ws, "/"); i >= 0 {
		return ws[i+1:]
	}
	return ws
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(264).FillHeight().Background(t.Surface).Shrink(0).Children(func() {
		// 品牌行：logo + DeepSeek + HARNESS 徽标
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(14, 16, 10).Children(func() {
			ui.Icon(c, icoBot).FontSize(17)
			ui.Text(c, "DeepSeek").Bold().FontSize(14)
			ui.Text(c, "HARNESS").FontSize(9).Padding(2, 5).Radius(4).
				BorderWidth(1, 1, 1, 1).BorderColor(t.Border).TextColor(t.TextMuted)
			ui.Spacer(c)
		})
		// 新会话：大按钮
		ui.Row(c).Padding(0, 12, 8).Children(func() {
			if ui.Button(c, "⊕ 新会话").Grow(1).Clicked() {
				a.newSession()
			}
		})
		// 搜索
		ui.Row(c).Padding(0, 12, 8).Children(func() {
			ui.TextInput(c, &a.query).Placeholder("搜索会话与轨迹…")
		})
		// 导航 + 工作区
		if ui.Sidebar(c, &a.selectedID, func() {
			ui.SidebarItem(c, "page:plugins", icoPlugin, "插件")
			ui.Spacer(c).Height(6)
			groups := groupByWorkspace(a.visibleSessions())
			for _, g := range groups {
				open := a.wsOpenFor(g.workspace)
				ui.SidebarSection(c, shortWS(g.workspace), open, func() {
					for _, m := range g.items {
						ui.SidebarItem(c, m.ID, icoFolder, m.Title)
					}
				})
			}
			if len(groups) == 0 {
				ui.Text(c, "没有匹配的会话").TextColor(t.TextMuted).FontSize(12).Padding(4, 20)
			}
		}).Grow(1).Changed() {
			if strings.HasPrefix(a.selectedID, "page:") {
				a.page = strings.TrimPrefix(a.selectedID, "page:")
			} else if a.selectedID != "" {
				a.page = "chat"
				a.focusComposer = 2
			}
		}
		// 底部：状态行 + 用户行（点击弹出菜单）
		ui.Column(c).Padding(8, 16, 10).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icoChat).FontSize(11)
				ui.Text(c, "本地模式").TextColor(t.TextMuted).FontSize(11)
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("会话 %d", len(a.sessions))).TextColor(t.TextMuted).FontSize(11)
			})
			userRow := ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 8).Radius(8).Label("用户菜单")
			if userRow.Hovered() || a.userMenu {
				userRow.Background(t.SurfaceHover)
			}
			userRow.Children(func() {
				ui.Text(c, "D").FontSize(11).Bold().Width(20).Height(20).Radius(10).
					Background(t.Accent).TextColor(t.AccentText).AlignItems(ui.Center)
				ui.Text(c, "DeepSeek Harness").FontSize(12)
				ui.Spacer(c)
				if a.userMenu {
					ui.Icon(c, icoUp).FontSize(12)
				} else {
					ui.Icon(c, icoDown).FontSize(12)
				}
			})
			if userRow.Clicked() {
				a.userMenu = !a.userMenu
			}
			ui.Popover(c, userRow, &a.userMenu, func() {
				for _, it := range []struct {
					icon  *ui.SVG
					label string
					page  string
				}{
					{icoGear, "设置", "settings"},
					{icoPlugin, "插件", "plugins"},
				} {
					entry := ui.Row(c.Key(it.label)).Gap(10).AlignItems(ui.Center).Padding(8, 12).Radius(8).Width(200)
					if entry.Hovered() {
						entry.Background(t.SurfaceHover)
					}
					entry.Children(func() {
						ui.Icon(c, it.icon).FontSize(14)
						ui.Text(c, it.label).FontSize(13)
					})
					if entry.Clicked() {
						a.userMenu = false
						if it.page == "settings" {
							a.openSettings()
							return
						}
						if strings.HasPrefix(a.selectedID, "page:") {
							a.selectedID = "" // 解除 page: 前缀对 page 的逐帧覆盖
						}
						a.page = it.page
					}
				}
				ui.Row(c).Padding(6, 12).Children(func() {
					ui.Text(c, "DeepSeek Harness v0.2").TextColor(t.TextMuted).FontSize(11)
				})
			})
		})
	})
}

func groupByWorkspace(all []*SessionMeta) []wsGroup {
	var order []string
	byWS := map[string][]*SessionMeta{}
	for _, m := range all {
		if _, ok := byWS[m.Workspace]; !ok {
			order = append(order, m.Workspace)
		}
		byWS[m.Workspace] = append(byWS[m.Workspace], m)
	}
	var out []wsGroup
	for _, ws := range order {
		out = append(out, wsGroup{workspace: ws, items: byWS[ws]})
	}
	return out
}

// ---- session lifecycle ----

func (a *app) newSession() {
	if a.busy {
		a.errText = "当前会话正在运行，请先停止"
		return
	}
	ses, err := a.store.New(a.cfg.WorkDir, a.cfg.Mode, "新会话")
	if err != nil {
		a.errText = "创建会话失败: " + err.Error()
		return
	}
	ses.AppendEvent("session", map[string]any{"mode": ses.Meta.Mode, "workspace": ses.Meta.Workspace})
	a.refreshSessions()
	a.selectedID = ses.Meta.ID
	a.page = "chat"
	a.loadSession(ses.Meta.ID)
}

func (a *app) loadSession(id string) {
	if a.busy {
		a.selectedID = a.loadedID
		a.errText = "会话正在运行，请先停止再切换"
		return
	}
	ses, err := a.store.Open(id)
	if err != nil {
		a.loadedID = a.selectedID
		return
	}
	a.ses = ses
	a.msgs = nil
	a.nextID = 0
	a.errText = ""
	a.status = ""
	a.input = ""
	a.dropAgent()
	a.cancel = nil
	a.pending = nil
	a.traj = ses.Events()
	a.trajOpen = map[int]*bool{}
	a.toolNames = map[string]string{}
	a.focusComposer = 2
	for _, ev := range a.traj {
		a.applyEvent(&a.msgs, ev)
	}
	a.loadedID = id
}

// applyEvent folds one trajectory event into the chat list (used both for
// replay and, via pump, for live updates).
func (a *app) applyEvent(msgs *[]*uiMsg, ev Event) {
	a.nextID++
	id := a.nextID
	switch ev.Kind {
	case "user":
		text, _ := ev.Data["text"].(string)
		*msgs = append(*msgs, &uiMsg{id: id, role: "user", text: text, done: true})
	case "assistant":
		b, err := json.Marshal(ev.Data["message"])
		if err != nil {
			return
		}
		var m llm.Message
		if json.Unmarshal(b, &m) != nil {
			return
		}
		*msgs = append(*msgs, &uiMsg{id: id, role: "assistant", text: m.Content, reasoning: m.Reasoning, done: true})
	case "tool_start":
		name, _ := ev.Data["name"].(string)
		args, _ := ev.Data["args"].(string)
		tid, _ := ev.Data["id"].(string)
		*msgs = append(*msgs, &uiMsg{id: id, role: "tool", done: true,
			tool: &toolView{id: tid, name: name, args: args, running: true}})
	case "tool":
		tid, _ := ev.Data["id"].(string)
		name, _ := ev.Data["name"].(string)
		result, _ := ev.Data["result"].(string)
		failed, _ := ev.Data["failed"].(bool)
		merged := false
		for _, m := range *msgs {
			if m.tool != nil && m.tool.id == tid {
				m.tool.result = result
				m.tool.failed = failed
				m.tool.running = false
				if name != "" {
					m.tool.name = name
				}
				merged = true
				break
			}
		}
		if !merged {
			*msgs = append(*msgs, &uiMsg{id: id, role: "tool", done: true,
				tool: &toolView{id: tid, name: name, result: result, failed: failed}})
		}
	case "todo":
		var items []agent.TodoItem
		if b, err := json.Marshal(ev.Data["items"]); err == nil && json.Unmarshal(b, &items) == nil {
			*msgs = append(*msgs, &uiMsg{id: id, role: "todo", todos: items, done: true})
		}
	case "subagent":
		desc, _ := ev.Data["description"].(string)
		out, _ := ev.Data["output"].(string)
		*msgs = append(*msgs, &uiMsg{id: id, role: "subagent", desc: desc, text: out, done: true})
	case "error":
		text, _ := ev.Data["text"].(string)
		a.errText = text
	case "note":
		text, _ := ev.Data["text"].(string)
		*msgs = append(*msgs, &uiMsg{id: id, role: "assistant", text: text, done: true})
	}
}

func (a *app) deleteSession() {
	if a.ses == nil {
		return
	}
	if a.busy {
		a.stop()
	}
	_ = a.ses.Delete()
	a.refreshSessions()
	a.ses = nil
	a.msgs = nil
	a.selectedID = ""
	a.loadedID = ""
	a.page = "chat"
}

func (a *app) forkSession() {
	if a.ses == nil {
		return
	}
	dst, err := a.store.Fork(a.ses)
	if err != nil {
		a.errText = "分叉失败: " + err.Error()
		return
	}
	a.refreshSessions()
	a.selectedID = dst.Meta.ID
	a.loadSession(dst.Meta.ID)
	a.page = "chat"
}

// ---- chat view (对话 / 轨迹 tabs over one shell) ----

func (a *app) chatShell(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		// 头部：标题 + 模式 + 会话操作
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(12, 20, 6).Children(func() {
			if a.ses != nil {
				ui.Text(c, a.ses.Meta.Title).Bold().FontSize(14).MaxWidth(320).Ellipsis("…")
				if ui.Select(c, &a.ses.Meta.Mode, presetLabels(a.presets)).Changed() {
					a.modeChanged()
				}
				if ui.Button(c, "分叉").Clicked() {
					a.forkSession()
				}
				if ui.Button(c, "删除").Clicked() {
					a.confirmDel = true
				}
			} else {
				ui.Text(c, "DeepSeek Harness").Bold().FontSize(14)
			}
			ui.Spacer(c)
			if a.status != "" {
				ui.Text(c, a.status).TextColor(t.TextMuted).FontSize(12)
			}
		})
		// 对话 / 轨迹 标签页
		ui.Row(c).Gap(24).Padding(0, 20).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			a.tab(c, "对话", "chat")
			a.tab(c, "轨迹", "trajectory")
		})
		if a.page == "trajectory" {
			a.trajectoryView(c)
		} else {
			a.chatTab(c)
		}
	})
	if ui.AlertDialog(c, &a.confirmDel, "删除会话？", "会话的轨迹与历史将一并删除，无法恢复。", "取消", "删除") == 1 {
		a.deleteSession()
	}
}

// chatTab is the 对话 tab: timeline, approval card and the inline composer.
func (a *app) chatTab(c *ui.Context) {
	t := c.Theme()
	a.list.Key = func(i int) any { return a.msgs[i].id }
	a.list.FollowEnd = true
	a.list.FollowAnimation = &ui.Smooth
	ui.Column(c).Fill().Children(func() {
		ui.List(c, &a.list, len(a.msgs), func(i int) {
			a.message(c, a.msgs[i])
		}).Grow(1).Padding(18, 20).Gap(14).Justify(ui.End).Children(func() {
			if len(a.msgs) == 0 {
				if a.ses == nil {
					a.welcome(c)
				} else {
					a.sessionReady(c)
				}
			}
		})
		if a.errText != "" {
			ui.Row(c).Padding(4, 24).Children(func() {
				ui.Text(c, a.errText).TextColor(t.Danger).FontSize(12).Selectable()
			})
		}
		if a.pending != nil {
			a.approvalCard(c)
		}
		// 内嵌式输入框：圆角容器 = 输入行 + 操作行
		ui.Column(c).Padding(8, 20, 4).Children(func() {
			ui.Column(c).Radius(14).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Background(t.Surface).Padding(12, 14).Gap(10).Children(func() {
				in := ui.TextInput(c, &a.input).Placeholder("发消息，/ 调用指令").BorderWidth(0, 0, 0, 0)
				if a.focusComposer > 0 {
					// Focus() re-asserts every frame for a two-frame window:
					// the click that set this flag (新会话 / 发送) hands focus
					// to the button AFTER the build, so the next frame must
					// take it back for typing.
					in = in.Focus()
					a.focusComposer--
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if ui.Button(c, "＋").Width(28).Height(28).Clicked() {
						a.input = "/"
						a.focusComposer = 2
					}
					if ui.Text(c, "/help 命令").TextColor(t.TextMuted).FontSize(12).Clicked() {
						a.note("可用命令：/new 新会话 · /compact 压缩历史 · /fork 分叉会话 · /mode <名称> 切换模式 · /cwd <目录> 切换工作区 · /trajectory 轨迹 · /help 本帮助")
					}
					ui.Spacer(c)
					ui.Text(c, a.cfg.Model).TextColor(t.TextMuted).FontSize(12)
					if a.busy {
						if ui.Button(c, "停止").Clicked() {
							a.stop()
						}
					} else if (ui.PrimaryButton(c, "↑").Width(30).Height(30).Radius(15).Disabled(strings.TrimSpace(a.input) == "").Clicked() || in.Submitted()) && strings.TrimSpace(a.input) != "" {
						a.send()
					}
				})
			})
		})
		// 底部状态栏
		ui.Row(c).Gap(14).AlignItems(ui.Center).Padding(2, 24, 8).Children(func() {
			ready := a.status
			if ready == "" {
				ready = "就绪"
			}
			ui.Text(c, ready+"  ·  轨迹 "+fmt.Sprint(len(a.traj))+" 条").TextColor(t.TextMuted).FontSize(11)
			ui.Spacer(c)
			ui.Text(c, "工具会先请求批准").TextColor(t.TextMuted).FontSize(11)
		})
	})
}

// tab renders one 对话/轨迹 tab with an orange underline when active.
func (a *app) tab(c *ui.Context, label, target string) {
	t := c.Theme()
	active := a.page == target
	txt := ui.Text(c, label).FontSize(13).Padding(6, 2)
	if active {
		txt = txt.TextColor(t.Text).Bold().BorderWidth(0, 0, 2, 0).BorderColor(t.Accent)
	} else {
		txt = txt.TextColor(t.TextMuted)
	}
	if txt.Clicked() && !active {
		a.page = target
		if target == "chat" {
			a.focusComposer = 2
		}
	}
}

func presetLabels(all []Preset) []string {
	var out []string
	for _, p := range all {
		out = append(out, p.Label)
	}
	return out
}

func labelToPreset(all []Preset, label string) *Preset {
	for i := range all {
		if all[i].Label == label {
			return &all[i]
		}
	}
	return &all[0]
}

func (a *app) sessionReady(c *ui.Context) {
	t := c.Theme()
	p := a.currentPreset()
	ui.Column(c).Margin(ui.Auto).MaxWidth(560).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icoChat).FontSize(16)
			ui.Text(c, "会话已就绪").Bold()
		})
		ui.Text(c, "工作区  "+a.ses.Meta.Workspace).TextColor(t.TextMuted).FontSize(12).MaxWidth(520).Ellipsis("…")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "模式").TextColor(t.TextMuted).FontSize(12)
			ui.Text(c, p.Label).FontSize(12)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "工具").TextColor(t.TextMuted).FontSize(12)
			ui.Text(c, strings.Join(p.Tools, " · ")).FontSize(12)
		})
		ui.Text(c, "在下方输入第一条消息开始。").TextColor(t.TextMuted).FontSize(12)
	})
}

func (a *app) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Margin(ui.Auto).MaxWidth(560).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icoBot).FontSize(28)
			ui.Text(c, "DeepSeek Harness").FontSize(24).Bold()
		})
		ui.Text(c, "Go 原生复刻 · MyGo 自绘 UI · 会话按工作区组织 · 工具调用摊开在时间线上").TextColor(t.TextMuted)
		ui.Column(c).Gap(6).Padding(12, 14).Radius(10).Background(t.Surface).Children(func() {
			for _, p := range a.presets {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Label).Bold().FontSize(12)
					ui.Text(c, p.Description).TextColor(t.TextMuted).FontSize(12)
				})
			}
		})
		if ui.PrimaryButton(c, "＋ 新建会话").Clicked() {
			a.newSession()
		}
		if strings.TrimSpace(a.cfg.APIKey) == "" {
			ui.Text(c, "尚未配置 API Key — 到「设置」填写，或设 DEEPSEEK_API_KEY 环境变量").TextColor(t.Warning)
		}
	})
}

func (a *app) message(c *ui.Context, m *uiMsg) {
	t := c.Theme()
	switch m.role {
	case "user":
		ui.Row(c).Children(func() {
			ui.Spacer(c)
			ui.Text(c, m.text).Selectable().Padding(8, 12).Radius(14).Background(t.Surface).MaxWidth(560)
		})
	case "tool":
		label := m.tool.name
		icon := icoWrench
		statusColor := t.TextMuted
		status := ""
		switch {
		case m.tool.running:
			status = " · 运行中…"
		case m.tool.failed:
			status = " · 失败"
			statusColor = t.Danger
		default:
			status = " · 完成"
			statusColor = t.Success
		}
		_ = statusColor
		ui.Row(c).MaxWidth(760).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).FontSize(13)
			ui.Collapsible(c, label+status, &m.tool.open, func() {
				ui.Column(c).Gap(6).Children(func() {
					if m.tool.args != "" {
						ui.Text(c, "参数").TextColor(t.TextMuted).FontSize(11)
						ui.Text(c, m.tool.args).FontSize(12).Selectable().Padding(8, 10).Radius(8).Background(t.Surface)
					}
					if m.tool.result != "" {
						ui.Text(c, "结果").TextColor(t.TextMuted).FontSize(11)
						ui.Text(c, m.tool.result).FontSize(12).Selectable().Padding(8, 10).Radius(8).Background(t.Surface)
					}
				})
			})
		})
	case "todo":
		done := 0
		for _, it := range m.todos {
			if it.Status == "completed" {
				done++
			}
		}
		ui.Column(c).MaxWidth(560).Padding(10, 14).Radius(10).Background(t.Surface).Gap(6).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icoList).FontSize(13)
				ui.Text(c, fmt.Sprintf("任务清单 · %d/%d", done, len(m.todos))).Bold().FontSize(13)
			})
			for _, it := range m.todos {
				mark := "○"
				col := t.TextMuted
				switch it.Status {
				case "completed":
					mark = "●"
					col = t.Success
				case "in_progress":
					mark = "◐"
					col = t.Accent
				}
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, mark).TextColor(col).FontSize(12)
					ui.Text(c, it.Content).FontSize(13).TextColor(t.Text)
				})
			}
		})
	case "subagent":
		ui.Row(c).MaxWidth(760).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icoBot).FontSize(13)
			ui.Collapsible(c, "子代理 · "+m.desc, &m.reasoningOpen, func() {
				ui.Text(c, m.text).FontSize(12).Selectable().Padding(8, 10).Radius(8).Background(t.Surface)
			})
		})
	default: // assistant
		ui.Column(c).MaxWidth(760).Gap(8).Children(func() {
			if m.reasoning != "" {
				ui.Collapsible(c, "思考过程", &m.reasoningOpen, func() {
					ui.Text(c, m.reasoning).TextColor(t.TextMuted).FontSize(12).Selectable()
				})
			}
			switch {
			case m.text != "" || m.done:
				if m.done {
					markdown.View(c, m.text, &a.md)
				} else {
					markdown.Stream(c, m.text, &a.md)
				}
			case m.reasoning == "":
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "思考中…").TextColor(t.TextMuted)
				})
			}
		})
	}
}

func (a *app) approvalCard(c *ui.Context) {
	t := c.Theme()
	p := a.pending
	ui.Column(c).Padding(10, 20).Children(func() {
		ui.Column(c).Gap(8).Padding(12, 14).Radius(10).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Warning).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icoWrench).FontSize(14)
				ui.Text(c, "工具请求批准 · "+p.tool).Bold()
			})
			ui.Text(c, p.args).FontSize(12).Selectable().MaxHeight(120)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "拒绝").Clicked() {
					p.ch <- agent.Deny
				}
				if ui.Button(c, "总是允许").Clicked() {
					p.ch <- agent.AllowAlways
				}
				if ui.PrimaryButton(c, "允许一次").Clicked() {
					p.ch <- agent.AllowOnce
				}
			})
		})
	})
}

// modeChanged persists a mode switch made from the header Select.
func (a *app) modeChanged() {
	if a.ses == nil {
		return
	}
	a.cfg.Mode = a.ses.Meta.Mode
	_ = saveConfig(a.cfg, a.cfgPath)
	a.dropAgent()
	a.refreshSessions()
}

func (a *app) kindColor(t *ui.Theme, kind string) ui.Color {
	switch kind {
	case "user":
		return t.Accent
	case "tool":
		return t.Success
	case "error":
		return t.Danger
	case "approval":
		return t.Warning
	case "todo":
		return t.Accent
	case "subagent":
		return ui.Hex("#a78bfa")
	default:
		return t.TextMuted
	}
}

func eventSummary(ev Event) string {
	switch ev.Kind {
	case "user":
		s, _ := ev.Data["text"].(string)
		return ellipsis(s, 90)
	case "assistant":
		if mm, ok := ev.Data["message"].(map[string]any); ok {
			s, _ := mm["Content"].(string)
			if s == "" {
				s, _ = mm["content"].(string)
			}
			return ellipsis(s, 90)
		}
		return "(模型回复)"
	case "tool":
		name, _ := ev.Data["name"].(string)
		r, _ := ev.Data["result"].(string)
		return name + " → " + ellipsis(r, 70)
	case "todo":
		return "任务清单更新"
	case "subagent":
		d, _ := ev.Data["description"].(string)
		return "子代理：" + d
	case "approval":
		tl, _ := ev.Data["tool"].(string)
		d, _ := ev.Data["decision"].(float64)
		dec := "拒绝"
		if d == 1 {
			dec = "允许"
		}
		if d == 2 {
			dec = "总是允许"
		}
		return tl + " · " + dec
	case "error":
		s, _ := ev.Data["text"].(string)
		return ellipsis(s, 90)
	case "compact":
		return "历史已压缩"
	case "session":
		m, _ := ev.Data["mode"].(string)
		return "会话 · 模式 " + m
	case "note":
		s, _ := ev.Data["text"].(string)
		return ellipsis(s, 90)
	}
	return ""
}

func ellipsis(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > n {
		r := []rune(s)
		return string(r[:n]) + "…"
	}
	return s
}

func (a *app) trajOpenFor(i int) *bool {
	if p, ok := a.trajOpen[i]; ok {
		return p
	}
	v := false
	a.trajOpen[i] = &v
	return &v
}

func (a *app) trajectoryView(c *ui.Context) {
	t := c.Theme()
	title := "（无会话）"
	if a.ses != nil {
		title = a.ses.Meta.Title
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "轨迹 · Trajectory").Bold()
			ui.Text(c, title).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				if a.ses != nil {
					a.traj = a.ses.Events()
					a.trajOpen = map[int]*bool{}
				}
			}
		})
		ui.List(c, &a.trajList, len(a.traj), func(i int) {
			ev := a.traj[i]
			raw, _ := json.MarshalIndent(ev.Data, "", "  ")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ev.Kind).FontSize(11).Padding(2, 8).Radius(10).
					Background(a.kindColor(t, ev.Kind)).TextColor(t.Background)
				ui.Text(c, ev.Time.Format("15:04:05")).TextColor(t.TextMuted).FontSize(11)
				ui.Text(c, eventSummary(ev)).FontSize(12).Grow(1).Ellipsis("…")
			})
			ui.Collapsible(c, "详情", a.trajOpenFor(i), func() {
				ui.Text(c, string(raw)).FontSize(11).Selectable().Padding(8, 10).Radius(8).Background(t.Surface)
			}).Padding(0, 0, 0, 24)
		}).Grow(1).Padding(12, 16).Gap(8).Children(func() {
			if len(a.traj) == 0 {
				ui.Text(c, "轨迹为空：一切模型看到的东西都会记录在这里（只增不改）").TextColor(t.TextMuted).Margin(ui.Auto)
			}
		})
	})
}

// ---- plugins view ----

var pluginToggles = map[string]*bool{}

func (a *app) pluginOn(name string) *bool {
	if b, ok := pluginToggles[name]; ok {
		return b
	}
	v := a.enabledTool(name)
	pluginToggles[name] = &v
	return &v
}

// ---- plugins view（REQ-004 三段式：内置清单卡 / 实验插件 / 已安装包）----

type renderFn func(c *ui.Context)

func (a *app) pluginsView(c *ui.Context) {
	t := c.Theme()
	var rows []renderFn

	// 段 1：内置插件清单卡（可搜索 + 展开详情）
	var builtin []Plugin
	q := strings.ToLower(strings.TrimSpace(a.pluginQuery))
	for _, p := range builtinPlugins() {
		if q == "" || strings.Contains(strings.ToLower(p.Name+p.ID+p.Description), q) {
			builtin = append(builtin, p)
		}
	}
	rows = append(rows, a.sectionHeader(fmt.Sprintf("内置插件 · %d", len(builtin))))
	rows = append(rows, func(c *ui.Context) {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 16).Children(func() {
			ui.TextInput(c, &a.pluginQuery).Placeholder("搜索插件…")
		})
	})
	for _, p := range builtin {
		plugin := p
		rows = append(rows, func(c *ui.Context) { a.builtinPluginCard(c, plugin) })
	}

	// 段 2：实验插件（REQ-004 FR-2）
	rows = append(rows, a.sectionHeader("实验插件"))
	rows = append(rows, func(c *ui.Context) {
		a.experimentalCard(c, "持久终端",
			"Keep shell processes alive across tool calls——所有含 bash 的模式都用持久 shell（原版同名实验插件）",
			"persistent_terminals")
	})

	// 段 3：已安装插件包
	rows = append(rows, a.sectionHeader(fmt.Sprintf("已安装插件 · %d", len(a.pkgCache))))
	rows = append(rows, a.installRow)
	for _, pkg := range a.pkgCache {
		p := pkg
		rows = append(rows, func(c *ui.Context) { a.packageCard(c, p) })
	}
	rows = append(rows, a.pluginsFooter)

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "插件 · 一切皆插件").Bold()
			ui.Spacer(c)
		})
		ui.List(c, &a.pluginList, len(rows), func(i int) {
			rows[i](c)
		}).Grow(1).Padding(8, 8)
	})
}

func (a *app) sectionHeader(text string) renderFn {
	return func(c *ui.Context) {
		t := c.Theme()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(12, 16).Children(func() {
			ui.Text(c, text).Bold().FontSize(12).TextColor(t.TextMuted)
		})
	}
}

func (a *app) pluginsFooter(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(4).Padding(14).Children(func() {
		ui.Text(c, "与初代一样，模型、工具、技能、会话、沙箱都是配置项；开关决定模式挂载哪些内置插件。").TextColor(t.TextMuted).FontSize(12)
		ui.Text(c, "创造模式的模型也能用 plugins 工具列出与管理插件；本地包放 <数据目录>/plugins/<id>/，或用上方输入框安装。").TextColor(t.TextMuted).FontSize(12)
	})
}

// badge renders the Enabled/Disabled pill used by the plugin list cards.
func pluginBadge(c *ui.Context, on bool) {
	t := c.Theme()
	text, color := "Enabled", t.Success
	if !on {
		text, color = "Disabled", t.TextMuted
	}
	ui.Text(c, text).FontSize(10).Padding(1, 7).Radius(8).Background(t.Background).TextColor(color)
}

// builtinPluginCard is one built-in plugin list card: name + state badge,
// description, expandable detail with version/source/tools and the switch.
func (a *app) builtinPluginCard(c *ui.Context, p Plugin) {
	t := c.Theme()
	open := a.boolRef(a.pluginOpen, p.ID)
	on := a.enabledTool(p.ID)
	ui.Column(c).Grow(1).Gap(0).Padding(2, 8).Children(func() {
		row := ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(10).BorderWidth(1).BorderColor(t.Border)
		row.Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, p.Name).Bold().FontSize(13)
				ui.Text(c, ellipsis(p.Description, 100)).TextColor(t.TextMuted).FontSize(11)
			})
			pluginBadge(c, on)
			if *open {
				ui.Text(c, "▾").TextColor(t.TextMuted)
			} else {
				ui.Text(c, "▸").TextColor(t.TextMuted)
			}
		})
		if row.Clicked() {
			*open = !*open
		}
		if *open {
			ui.Column(c).Gap(4).Padding(10, 14).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "v"+p.Version).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					ui.Text(c, p.Source).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					ui.Text(c, p.Category).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					ui.Text(c, p.ID).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
				})
				ui.Text(c, "挂载工具："+strings.Join(p.Tools, ", ")).TextColor(t.TextMuted).FontSize(11)
				sw := a.enabledTool(p.ID)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "启用").FontSize(12)
					ui.Switch(c, &sw)
				})
				if sw != a.enabledTool(p.ID) {
					a.setEnabled(p.ID, sw)
				}
			})
		}
	})
}

// experimentalCard is one experimental-plugin toggle card (original style:
// title + description + Enable switch).
func (a *app) experimentalCard(c *ui.Context, name, desc, key string) {
	t := c.Theme()
	on := a.experimentalOn(key)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(10).BorderWidth(1).BorderColor(t.Border).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, name).Bold().FontSize(13)
				ui.Text(c, "Experimental").FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
			})
			ui.Text(c, ellipsis(desc, 120)).TextColor(t.TextMuted).FontSize(11)
		})
		ui.Switch(c, &on)
	})
	if on != a.experimentalOn(key) {
		a.setExperimental(key, on)
	}
}

// installRow is the Add-plugin entry of the installed section.
func (a *app) installRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 12).Children(func() {
		ui.TextInput(c, &a.installInput).Placeholder("本地插件目录路径，或以 .git / git@ 开头的 git URL…").Grow(1)
		if ui.Button(c, "安装插件").Clicked() {
			a.installPackage()
		}
		ui.Text(c, "").FontSize(1).TextColor(t.TextMuted)
	})
}

// packageCard is one installed package: name/version/source + state, with
// expandable per-tool (child plugin) switches and a two-step uninstall.
func (a *app) packageCard(c *ui.Context, p PluginPackage) {
	t := c.Theme()
	open := a.boolRef(a.pkgOpen, p.ID)
	ui.Column(c).Grow(1).Gap(0).Padding(2, 8).Children(func() {
		row := ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(10).BorderWidth(1).BorderColor(t.Border)
		row.Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Name).Bold().FontSize(13)
					ui.Text(c, "v"+p.Version).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					ui.Text(c, p.Source).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					ui.Text(c, a.cfg.packageState(p)).FontSize(10).TextColor(t.TextMuted)
				})
				if p.Description != "" {
					ui.Text(c, ellipsis(p.Description, 100)).TextColor(t.TextMuted).FontSize(11)
				}
			})
			if *open {
				ui.Text(c, "▾").TextColor(t.TextMuted)
			} else {
				ui.Text(c, "▸").TextColor(t.TextMuted)
			}
		})
		if row.Clicked() {
			*open = !*open
		}
		if *open {
			ui.Column(c).Gap(6).Padding(10, 14).Children(func() {
				if p.Origin != "" {
					ui.Text(c, "来源："+p.Origin).TextColor(t.TextMuted).FontSize(10)
				}
				for _, tl := range p.Tools {
					tool := tl
					on := a.cfg.packageChildOn(p.ID, tool.Name)
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Gap(1).Children(func() {
							ui.Text(c, tool.Name).FontSize(12)
							ui.Text(c, ellipsis(tool.Description, 90)).TextColor(t.TextMuted).FontSize(10)
						})
						ui.Switch(c, &on)
					})
					if on != a.cfg.packageChildOn(p.ID, tool.Name) {
						a.setEnabled(p.ID+"/"+tool.Name, on)
					}
				}
				label := "卸载"
				if a.confirmUninstall == p.ID {
					label = "再次点击确认卸载"
				}
				if ui.Button(c, label).Clicked() {
					a.uninstallPackageUI(p)
				}
			})
		}
	})
}

// installPackage installs from the composer input: an existing directory is
// imported locally; a git-looking URL is cloned (REQ-004 FR-6).
func (a *app) installPackage() {
	src := strings.TrimSpace(a.installInput)
	a.installInput = ""
	a.errText = ""
	if src == "" {
		a.errText = "请输入本地插件目录路径或 git URL"
		return
	}
	var (
		p   PluginPackage
		err error
	)
	if st, serr := os.Stat(src); serr == nil && st.IsDir() {
		p, err = installFromPath(a.dataDir, src)
	} else if strings.HasPrefix(src, "git@") || strings.HasSuffix(src, ".git") ||
		strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "http://") {
		p, err = installFromGit(a.dataDir, src)
	} else {
		err = fmt.Errorf("目录不存在，也无法识别为 git URL（git 地址请以 .git 结尾或 git@ 开头）")
	}
	if err != nil {
		a.errText = "安装失败：" + err.Error()
		return
	}
	a.refreshPackages()
	a.dropAgent()
	a.note(fmt.Sprintf("插件包已安装：%s v%s（%d 个工具，从下一条消息开始可用）", p.Name, p.Version, len(p.Tools)))
}

// uninstallPackageUI runs the two-step uninstall (REQ-004 FR-7).
func (a *app) uninstallPackageUI(p PluginPackage) {
	if a.confirmUninstall != p.ID {
		a.confirmUninstall = p.ID
		return
	}
	a.confirmUninstall = ""
	if err := uninstallPackage(a.dataDir, p.ID); err != nil {
		a.errText = "卸载失败：" + err.Error()
		return
	}
	a.refreshPackages()
	a.dropAgent()
	a.note("插件包已卸载：" + p.Name)
}

func (a *app) setEnabled(name string, on bool) {
	if a.cfg.EnabledTools == nil {
		a.cfg.EnabledTools = map[string]bool{}
	}
	a.cfg.EnabledTools[name] = on
	_ = saveConfig(a.cfg, a.cfgPath)
}

// pluginControl backs the agent's plugins tool (REQ-002): the model can
// list, inspect, enable and disable plugins. Changes persist via
// setEnabled and take effect from the next user message.
func (a *app) pluginControl(action, id string) (string, bool) {
	all := builtinPlugins()
	switch action {
	case "list":
		var b strings.Builder
		fmt.Fprintf(&b, "共 %d 个内置插件：", len(all))
		for _, g := range pluginGroups(all) {
			fmt.Fprintf(&b, "\n\n[%s]", g.Category)
			for _, p := range g.Plugins {
				state := "关"
				if a.enabledTool(p.ID) {
					state = "开"
				}
				fmt.Fprintf(&b, "\n- %s（%s）%s — %s", p.ID, state, p.Name, p.Description)
			}
		}
		return b.String(), false
	case "info":
		p, ok := pluginByID(all, id)
		if !ok {
			return "error: 未知插件 " + id + "；用 action=list 查看全部", true
		}
		state := "已停用"
		if a.enabledTool(p.ID) {
			state = "已启用"
		}
		return fmt.Sprintf("id=%s\n名称=%s\n版本=v%s\n分类=%s\n来源=%s\n挂载工具=%s\n状态=%s\n描述=%s",
			p.ID, p.Name, p.Version, p.Category, p.Source, strings.Join(p.Tools, ","), state, p.Description), false
	case "enable", "disable":
		p, ok := pluginByID(all, id)
		if !ok {
			return "error: 未知插件 " + id + "；用 action=list 查看全部", true
		}
		on := action == "enable"
		a.setEnabled(p.ID, on)
		verb := "已启用"
		if !on {
			verb = "已停用"
		}
		tail := ""
		if p.Kind == "control" && !on {
			tail = "（注意：这会移除你自己的 plugins 工具，用户仍可在插件页恢复）"
		}
		return fmt.Sprintf("插件 %s（%s）%s，从下一条用户消息开始生效。%s", p.ID, p.Name, verb, tail), false
	default:
		return "error: 未知 action " + action + "（可选 list / info / enable / disable）", true
	}
}

// ---- settings view ----

// settingsView is the headless/test fallback that renders the settings
// cards in the main area. Production opens settingsBody in a child window.
func (a *app) settingsView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "设置").Bold()
			ui.Spacer(c)
		})
		a.secModel(c)
	})
}

// openSettings shows the settings child window (a real OS window that
// stays on top of the main one), or focuses the existing one. Headless
// runs (tests) have no window host and fall back to the main area.
func (a *app) openSettings() {
	if a.win == nil {
		a.page = "settings"
		return
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Focus()
		return
	}
	a.setSection = "账号与余额"
	if strings.TrimSpace(a.cfg.APIKey) != "" && !a.balFetched && !a.balLoading {
		go a.fetchBalance()
	}
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:     "设置 · DeepSeek Harness",
		Width:     960,
		Height:    700,
		MinWidth:  820,
		MinHeight: 560,
		Parent:    a.win,
		Modal:     true, // 打开期间主窗口不可操作
		StateKey:  "settings",
		Content:   ui.View(a.settingsWindowView),
	})
	a.settingsWin.OnClosed(func() { a.settingsWin = nil })
}

// settingsWindowView is the content of the settings child window.
func (a *app) settingsWindowView(c *ui.Context) {
	c.SetTheme(a.theme)
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(16, 20, 10).Children(func() {
			ui.Text(c, "设置").Bold().FontSize(17)
			ui.Spacer(c)
			if ui.Button(c, "打开配置文件").Clicked() {
				a.revealConfig()
			}
			// macOS 上模态子窗口以 sheet 呈现，没有原生关闭按钮，这里补一个
			if ui.Button(c, "✕").Width(30).Height(26).Clicked() {
				if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
					a.settingsWin.Close()
				}
			}
		})
		a.settingsBody(c)
	})
}

// settingsSections is the left nav of the settings window, mirroring the
// original app's sections.
var settingsSections = []struct {
	icon  *ui.SVG
	label string
}{
	{icoUser, "账号与余额"},
	{icoGear, "通用设置"},
	{icoBot, "模型"},
	{icoPlugin, "内置插件"},
	{icoWrench, "技能"},
	{icoList, "Agent 预设"},
	{icoArc, "已归档会话"},
	{icoGlobe, "Web 插件"},
	{icoPal, "皮肤"},
	{icoHeart, "宠物"},
	{icoPlus, "创意工坊"},
	{icoTraj, "使用统计"},
}

// settingsBody is the settings window content: left nav + right panel.
func (a *app) settingsBody(c *ui.Context) {
	t := c.Theme()
	if a.setSection == "" {
		a.setSection = "账号与余额"
	}
	ui.Row(c).Fill().Children(func() {
		// 左侧分区导航
		ui.Column(c).Width(196).FillHeight().Padding(6, 10).Gap(2).Children(func() {
			for _, sec := range settingsSections {
				active := a.setSection == sec.label
				row := ui.Row(c.Key("sec-"+sec.label)).Gap(10).AlignItems(ui.Center).Padding(9, 12).Radius(8)
				if active || row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Icon(c, sec.icon).FontSize(15)
					lbl := ui.Text(c, sec.label).FontSize(13)
					if active {
						lbl.TextColor(t.Text)
					} else {
						lbl.TextColor(t.TextMuted)
					}
				})
				if row.Clicked() {
					a.setSection = sec.label
					if sec.label == "账号与余额" && !a.balFetched && !a.balLoading {
						go a.fetchBalance()
					}
					if sec.label == "使用统计" && !a.statsLoaded && !a.statsLoading {
						go a.fetchStats()
					}
				}
			}
		})
		// 分隔线
		ui.Column(c).Width(1).FillHeight().Background(t.Border).Shrink(0)
		// 右侧内容
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			ui.List(c, &a.setList, 1, func(i int) {
				ui.Column(c).Fill().Padding(20, 24).Gap(14).Children(func() {
					switch a.setSection {
					case "通用设置":
						a.secGeneral(c)
					case "模型":
						a.secModel(c)
					case "内置插件":
						a.secPlugins(c)
					case "技能":
						a.secSkills(c)
					case "Agent 预设":
						a.secPresets(c)
					case "已归档会话":
						a.settingsPlaceholder(c, "已归档会话会出现在这里（归档功能与初代 dsh 对齐中）")
					case "Web 插件":
						a.settingsPlaceholder(c, "Web 插件（如浏览器扩展桥）与初代 dsh 对齐中")
					case "皮肤":
						a.secSkins(c)
					case "宠物":
						a.settingsPlaceholder(c, "宠物 🐱 与初代 dsh 对齐中")
					case "创意工坊":
						a.settingsPlaceholder(c, "创意工坊：社区模式与技能市场，与初代 dsh 对齐中")
					case "使用统计":
						a.secStats(c)
					default:
						a.secAccount(c)
					}
				})
			}).Grow(1)
		})
	})
}

func (a *app) settingsPlaceholder(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Margin(ui.Auto).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icoGear).FontSize(20)
		ui.Text(c, text).TextColor(t.TextMuted).FontSize(13)
	})
}

// settingsCard wraps a section block in the raised card look.
func settingsCard(c *ui.Context, title string, fn func()) {
	t := c.Theme()
	ui.Column(c).Gap(10).Padding(16).Radius(10).Background(t.Surface).Children(func() {
		if title != "" {
			ui.Text(c, title).Bold()
		}
		fn()
	})
}

// ---- 账号与余额 ----

func (a *app) secAccount(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "", func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Width(38).Height(38).Radius(19).Background(t.Accent).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, "D").FontSize(14).Bold().TextColor(t.AccentText)
			})
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "本地账号").Bold().FontSize(14)
				ui.Text(c, a.maskedKey()).TextColor(t.TextMuted).FontSize(12)
			})
			ui.Spacer(c)
			if ui.Text(c, "管理 API Key ↗").TextColor(t.TextMuted).FontSize(12).Clicked() {
				c.OpenURL("https://platform.deepseek.com")
			}
		})
	})
	settingsCard(c, "余额", func() {
		if strings.TrimSpace(a.cfg.APIKey) == "" {
			ui.Text(c, "尚未配置 API Key —— 到「模型」分区填写后即可查询余额").TextColor(t.Warning).FontSize(13)
			return
		}
		row := func(label, value string, big bool) {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, label).TextColor(t.TextMuted).FontSize(13)
				ui.Spacer(c)
				txt := ui.Text(c, value).Bold()
				if big {
					txt.FontSize(16)
				}
			})
		}
		if a.balLoading {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "查询中…").TextColor(t.TextMuted).FontSize(13)
			})
		} else if a.balErr != "" {
			ui.Text(c, "查询失败："+a.balErr).TextColor(t.Danger).FontSize(13)
		} else if a.balFetched {
			cur := a.balCurrency
			if cur == "" {
				cur = "CNY"
			}
			sym := "¥"
			if cur != "CNY" {
				sym = cur + " "
			}
			row("充值余额", sym+a.balTopped, true)
			row("赠金余额", sym+a.balGranted, false)
			row("总余额", sym+a.balTotal, false)
		} else {
			ui.Text(c, "点击「查询用量」获取最新余额").TextColor(t.TextMuted).FontSize(13)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 0, 0).Children(func() {
			ui.Text(c, "更多").TextColor(t.TextMuted).FontSize(13)
			ui.Spacer(c)
			if ui.Button(c, "查询用量").Clicked() {
				go a.fetchBalance()
			}
			if ui.PrimaryButton(c, "充值").Clicked() {
				c.OpenURL("https://platform.deepseek.com/topup")
			}
		})
	})
}

func (a *app) maskedKey() string {
	k := strings.TrimSpace(a.cfg.APIKey)
	if k == "" {
		return "未配置"
	}
	if len(k) <= 11 {
		return k[:3] + "…" + k[len(k)-2:]
	}
	return k[:7] + "…" + k[len(k)-4:]
}

func (a *app) revealConfig() {
	cmd := exec.Command("open", "-R", a.cfgPath)
	if err := cmd.Start(); err != nil {
		_ = exec.Command("xdg-open", a.cfgPath).Start()
	}
}

// ---- 通用设置 / 模型 ----

func (a *app) secGeneral(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "默认工作区", func() {
		ui.Text(c, "新建会话的目录；bash 的 cwd 与相对路径的根").TextColor(t.TextMuted).FontSize(12)
		ui.TextInput(c, &a.fWorkDir).Label("默认工作区输入框")
	})
	settingsCard(c, "工具权限", func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &a.autoApprove, "")
			ui.Text(c, "自动批准变更类工具（bash / 写文件 / 编辑）——不再逐次询问").FontSize(13)
		})
	})
	settingsCard(c, "配置", func() {
		ui.Text(c, "配置文件路径："+a.cfgPath).TextColor(t.TextMuted).FontSize(12)
		if ui.Button(c, "打开配置文件").Clicked() {
			a.revealConfig()
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.PrimaryButton(c, "保存设置").Clicked() {
			a.saveSettings()
		}
		if !a.savedAt.IsZero() {
			ui.Text(c, "已保存 "+a.savedAt.Format("15:04:05")).TextColor(t.Success).FontSize(12)
		}
		if a.errText != "" {
			ui.Text(c, a.errText).TextColor(t.Danger).FontSize(12)
		}
	})
}

func (a *app) secModel(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "模型 · DeepSeek", func() {
		ui.Text(c, "API Key（明文保存在 "+a.cfgPath+"）").TextColor(t.TextMuted).FontSize(12)
		ui.TextInput(c, &a.fKey).Password().Placeholder("sk-…").Label("API Key 输入框")
		ui.Text(c, "API Base URL（兼容 OpenAI 接口的服务都可填）").TextColor(t.TextMuted).FontSize(12)
		ui.TextInput(c, &a.fURL).Placeholder("https://api.deepseek.com").Label("Base URL 输入框")
		ui.Text(c, "模型").TextColor(t.TextMuted).FontSize(12)
		ui.Select(c, &a.fModel, []string{"deepseek-chat", "deepseek-reasoner"})
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.PrimaryButton(c, "保存设置").Clicked() {
			a.saveSettings()
		}
		if !a.savedAt.IsZero() {
			ui.Text(c, "已保存 "+a.savedAt.Format("15:04:05")).TextColor(t.Success).FontSize(12)
		}
		if a.errText != "" {
			ui.Text(c, a.errText).TextColor(t.Danger).FontSize(12)
		}
	})
}

// ---- 内置插件（设置分区，与插件页同一清单卡渲染）----

func (a *app) secPlugins(c *ui.Context) {
	t := c.Theme()
	q := strings.ToLower(strings.TrimSpace(a.setPluginQuery))
	var list []Plugin
	for _, p := range builtinPlugins() {
		if q == "" || strings.Contains(strings.ToLower(p.Name+p.ID+p.Description), q) {
			list = append(list, p)
		}
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &a.setPluginQuery).Placeholder("搜索插件…").Grow(1)
		ui.Text(c, fmt.Sprintf("%d 个内置插件", len(list))).FontSize(11).TextColor(t.TextMuted)
	})
	for _, p := range list {
		plugin := p
		a.builtinPluginCard(c, plugin)
	}
}

// ---- 技能 ----

func (a *app) secSkills(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "已发现的技能", func() {
		skills := a.currentSkills()
		if len(skills) == 0 {
			ui.Column(c).Gap(4).Padding(10, 12).Children(func() {
				ui.Text(c, "未发现技能").Bold().FontSize(13)
				ui.Text(c, "把技能放到 <数据目录>/skills/<名称>/SKILL.md 或 <工作区>/.harness/skills/<名称>/SKILL.md："+"frontmatter 写 name / description，正文是给模型的指令（标准/创造模式可经 skill 工具加载）。").TextColor(t.TextMuted).FontSize(12)
			})
			return
		}
		for _, s := range skills {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(8).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, s.Name).Bold().FontSize(13)
						ui.Text(c, s.Source).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
					})
					if s.Description != "" {
						ui.Text(c, ellipsis(s.Description, 100)).TextColor(t.TextMuted).FontSize(11)
					}
					ui.Text(c, s.Path).TextColor(t.TextMuted).FontSize(10)
				})
			})
		}
	})
}

// ---- Agent 预设 ----

func (a *app) secPresets(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "内置与自定义预设", func() {
		for _, p := range a.presets {
			current := a.cfg.Mode == p.ID
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(8).Children(func() {
				if current {
					ui.Text(c, "●").TextColor(t.Accent).FontSize(11)
				} else {
					ui.Text(c, "○").TextColor(t.TextMuted).FontSize(11)
				}
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, p.Label).Bold().FontSize(13)
						ui.Text(c, p.Description).TextColor(t.TextMuted).FontSize(12)
					})
					ui.Row(c).Gap(6).Children(func() {
						for _, n := range p.Tools {
							ui.Text(c, n).FontSize(10).Padding(1, 6).Radius(6).Background(t.Background).TextColor(t.TextMuted)
						}
					})
				})
			})
			if !current {
				if ui.Button(c, "设为默认").Clicked() {
					a.cfg.Mode = p.ID
					_ = saveConfig(a.cfg, a.cfgPath)
					if a.ses != nil {
						a.ses.Meta.Mode = p.ID
						a.ses.Touch()
						a.refreshSessions()
					}
					a.dropAgent()
				}
			}
		}
	})
	settingsCard(c, "创造新模式", func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &a.epOpen, "创造一个新模式（自定义预设）")
			ui.Spacer(c)
		})
		if a.epOpen {
			ui.Column(c).Gap(8).Padding(12).Radius(8).Background(t.Background).Children(func() {
				ui.Text(c, "名称").TextColor(t.TextMuted).FontSize(12)
				ui.TextInput(c, &a.epLabel).Placeholder("例如：写作")
				ui.Text(c, "系统提示词（{cwd} 与 {data} 会自动替换）").TextColor(t.TextMuted).FontSize(12)
				ui.TextArea(c, &a.epPrompt).Height(110)
				ui.Text(c, "挂载的插件（工具）").TextColor(t.TextMuted).FontSize(12)
				epToolNames := []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep", "web_fetch", "web_search", "todo", "task", "run_code"}
				ui.Row(c).Wrap().GapX(12).GapY(6).Children(func() {
					for _, n := range epToolNames {
						ui.Checkbox(c, a.epToolOn(n), n)
					}
				})
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c, "保存新模式").Clicked() {
						a.saveCustomPreset()
					}
				})
			})
		}
	})
}

// ---- 皮肤 ----

var skinNames = []string{"暖黑", "深海", "森绿"}

func (a *app) secSkins(c *ui.Context) {
	t := c.Theme()
	settingsCard(c, "主题", func() {
		for _, name := range skinNames {
			current := a.cfg.Theme == name
			row := ui.Row(c.Key("skin-"+name)).Gap(12).AlignItems(ui.Center).Padding(10, 12).Radius(8)
			if current || row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				ui.Text(c, name).FontSize(13)
				ui.Spacer(c)
				if current {
					ui.Text(c, "✓ 使用中").TextColor(t.Accent).FontSize(12)
				}
			})
			if row.Clicked() && !current {
				a.applySkin(name)
			}
		}
	})
	ui.Text(c, "皮肤即时生效，同时作用于主窗口与本窗口").TextColor(t.TextMuted).FontSize(12)
}

// ---- 使用统计 ----

func (a *app) secStats(c *ui.Context) {
	t := c.Theme()
	if !a.statsLoaded && !a.statsLoading {
		go a.fetchStats()
	}
	settingsCard(c, "本地统计", func() {
		if a.statsLoading {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "统计中…").TextColor(t.TextMuted).FontSize(13)
			})
			return
		}
		row := func(label, value string) {
			ui.Row(c).Padding(6, 0).Children(func() {
				ui.Text(c, label).TextColor(t.TextMuted).FontSize(13)
				ui.Spacer(c)
				ui.Text(c, value).Bold().FontSize(13)
			})
		}
		row("会话总数", fmt.Sprint(a.statsSessions))
		row("轨迹事件", fmt.Sprint(a.statsEvents))
		row("当前模型", a.cfg.Model)
		row("当前工作区", ellipsis(a.cfg.WorkDir, 40))
	})
	ui.Text(c, "Token 用量统计需要模型返回 usage 字段，当前流式接口未启用").TextColor(t.TextMuted).FontSize(12)
}

// ---- balance / stats 数据 ----

func (a *app) fetchBalance() {
	if a.balLoading {
		return
	}
	key := strings.TrimSpace(a.cfg.APIKey)
	if key == "" {
		a.applyBalance("", "", "", "", "尚未配置 API Key")
		return
	}
	a.balLoading = true
	a.balErr = ""
	go func() {
		req, err := http.NewRequest("GET", strings.TrimRight(a.cfg.BaseURL, "/")+"/user/balance", nil)
		if err != nil {
			a.applyBalance("", "", "", "", err.Error())
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			a.applyBalance("", "", "", "", err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			a.applyBalance("", "", "", "", fmt.Sprintf("API 返回 %d: %s", resp.StatusCode, ellipsis(string(b), 120)))
			return
		}
		var out struct {
			BalanceInformation struct {
				TotalBalance    string `json:"total_balance"`
				GrantedBalance  string `json:"granted_balance"`
				ToppedUpBalance string `json:"topped_up_balance"`
				Currency        string `json:"currency"`
			} `json:"balance_information"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			a.applyBalance("", "", "", "", "解析响应失败: "+err.Error())
			return
		}
		bi := out.BalanceInformation
		a.applyBalance(bi.TotalBalance, bi.ToppedUpBalance, bi.GrantedBalance, bi.Currency, "")
	}()
}

func (a *app) applyBalance(total, topped, granted, currency, errText string) {
	fn := func() {
		a.balLoading = false
		a.balFetched = true
		a.balErr = errText
		a.balTotal, a.balTopped, a.balGranted, a.balCurrency = total, topped, granted, currency
	}
	a.onSettingsWin(fn)
}

func (a *app) fetchStats() {
	if a.statsLoading {
		return
	}
	a.statsLoading = true
	go func() {
		sessions := a.store.List()
		events := 0
		for _, m := range sessions {
			if ses, err := a.store.Open(m.ID); err == nil {
				events += len(ses.Events())
			}
		}
		fn := func() {
			a.statsLoading = false
			a.statsLoaded = true
			a.statsSessions, a.statsEvents = len(sessions), events
		}
		a.onSettingsWin(fn)
	}()
}

// onSettingsWin runs fn on the settings window's UI pass (or inline in
// headless runs).
func (a *app) onSettingsWin(fn func()) {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Update(fn)
		return
	}
	fn()
}

func (a *app) applySkin(name string) {
	a.cfg.Theme = name
	_ = saveConfig(a.cfg, a.cfgPath)
	a.theme = dshTheme(name)
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Update(func() {})
	}
	a.onSettingsWin(func() {})
}

var epToolState = map[string]*bool{}

func (a *app) epToolOn(name string) *bool {
	if b, ok := epToolState[name]; ok {
		return b
	}
	v := a.enabledTool(name)
	epToolState[name] = &v
	return &v
}

func (a *app) saveCustomPreset() {
	label := strings.TrimSpace(a.epLabel)
	if label == "" {
		a.errText = "模式名称不能为空"
		return
	}
	var chosen []string
	for _, n := range []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep", "web_fetch", "web_search", "todo", "task", "run_code"} {
		if b := epToolState[n]; b != nil && *b {
			chosen = append(chosen, n)
		}
	}
	if len(chosen) == 0 {
		a.errText = "至少选择一个工具"
		return
	}
	var customs []Preset
	for _, p := range a.presets {
		if p.Custom {
			customs = append(customs, p)
		}
	}
	customs = append(customs, Preset{
		ID:           "custom-" + fmt.Sprintf("%d", time.Now().Unix()),
		Label:        label,
		Description:  "自定义模式 · " + strings.Join(chosen, ", "),
		Tools:        chosen,
		SystemPrompt: a.epPrompt,
		Custom:       true,
	})
	if err := saveCustomPresets(a.dataDir, customs); err != nil {
		a.errText = "保存失败: " + err.Error()
		return
	}
	a.presets = loadPresets(a.dataDir)
	a.epOpen = false
	a.epLabel = ""
	a.epPrompt = ""
	a.errText = ""
	a.note("新模式「" + label + "」已保存，可在会话头部的模式选择器里切换。")
}

func (a *app) saveSettings() {
	a.cfg.APIKey = strings.TrimSpace(a.fKey)
	a.cfg.BaseURL = strings.TrimSpace(a.fURL)
	a.cfg.Model = a.fModel
	a.cfg.WorkDir = strings.TrimSpace(a.fWorkDir)
	a.cfg.AutoApprove = a.autoApprove
	if err := saveConfig(a.cfg, a.cfgPath); err != nil {
		a.errText = "保存失败: " + err.Error()
		return
	}
	a.errText = ""
	a.savedAt = time.Now()
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Update(func() {}) // 让主窗口立即反映模型/工作区变化
	}
}

// ---- send / stop / compact / commands ----

func (a *app) stop() {
	if a.cancel != nil {
		a.status = "正在停止…"
		a.cancel()
	}
}

func (a *app) send() {
	text := strings.TrimSpace(a.input)
	if text == "" || a.busy {
		return
	}
	if strings.TrimSpace(a.cfg.APIKey) == "" {
		a.errText = "尚未设置 API Key，请到「设置」填写"
		a.selectedID = "page:settings"
		return
	}
	if strings.HasPrefix(text, "/") {
		a.command(text)
		return
	}
	if a.ses == nil {
		a.newSession()
		if a.ses == nil {
			return
		}
	}

	a.input = ""
	a.focusComposer = 2 // keep typing the next message after 发送
	a.errText = ""
	a.ses.AppendEvent("user", map[string]any{"text": text})
	if a.ses.Meta.Title == "新会话" {
		a.ses.Retitle(ellipsis(text, 24))
		a.refreshSessions()
	}
	a.nextID++
	a.msgs = append(a.msgs, &uiMsg{id: a.nextID, role: "user", text: text, done: true})
	a.nextID++
	reply := &uiMsg{id: a.nextID, role: "assistant", reasoningOpen: true}
	a.msgs = append(a.msgs, reply)
	a.busy = true
	a.status = "思考中…"

	if a.ag == nil {
		a.ag = a.newAgentFor(a.ses)
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	p := a.pump
	if p == nil {
		p = newPump(a.win)
		a.pump = p
	}
	ses := a.ses
	cbs := agent.Callbacks{
		OnContent:   func(d string) { p.add(func() { reply.text += d }) },
		OnReasoning: func(d string) { p.add(func() { reply.reasoning += d }) },
		OnToolStart: func(id, name, args string) {
			a.toolNames[id] = name
			p.add(func() {
				a.nextID++
				a.msgs = append(a.msgs, &uiMsg{
					id:   a.nextID,
					role: "tool",
					done: true,
					tool: &toolView{id: id, name: name, args: args, running: true, open: true},
				})
			})
			ses.AppendEvent("tool_start", map[string]any{"id": id, "name": name, "args": args})
		},
		OnToolEnd: func(id, result string, failed bool) {
			p.add(func() {
				for _, m := range a.msgs {
					if m.tool != nil && m.tool.id == id {
						m.tool.result = result
						m.tool.running = false
						m.tool.failed = failed
						m.tool.open = false
					}
				}
			})
			ses.AppendEvent("tool", map[string]any{"id": id, "name": a.toolNames[id], "result": result, "failed": failed})
		},
		OnStatus: func(s string) { p.add(func() { a.status = s }) },
		OnApproval: func(toolName, args string) agent.Decision {
			if a.cfg.AutoApprove {
				return agent.AllowOnce
			}
			if a.alwaysAllow[toolName] {
				return agent.AllowOnce
			}
			ch := make(chan agent.Decision, 1)
			p.add(func() { a.pending = &pendingApproval{tool: toolName, args: args, ch: ch} })
			d := <-ch
			p.add(func() { a.pending = nil })
			if d == agent.AllowAlways {
				a.alwaysAllow[toolName] = true
				d = agent.AllowOnce
			}
			ses.AppendEvent("approval", map[string]any{"tool": toolName, "args": args, "decision": int(d)})
			return d
		},
		OnTodo: func(items []agent.TodoItem) {
			p.add(func() {
				a.nextID++
				a.msgs = append(a.msgs, &uiMsg{id: a.nextID, role: "todo", todos: items, done: true})
			})
			b, _ := json.Marshal(items)
			var raw []map[string]any
			_ = json.Unmarshal(b, &raw)
			ses.AppendEvent("todo", map[string]any{"items": raw})
		},
		OnSubagentStart: func(desc string) {
			p.add(func() { a.status = "子代理：" + desc })
		},
		OnSubagentEnd: func(desc, output string) {
			p.add(func() {
				a.nextID++
				a.msgs = append(a.msgs, &uiMsg{id: a.nextID, role: "subagent", desc: desc, text: output, done: true})
				a.status = "思考中…"
			})
			ses.AppendEvent("subagent", map[string]any{"description": desc, "output": output})
		},
		OnAssistantMessage: func(m llm.Message) {
			b, _ := json.Marshal(m)
			var raw map[string]any
			_ = json.Unmarshal(b, &raw)
			ses.AppendEvent("assistant", map[string]any{"message": raw})
		},
		OnDone: func(err error) {
			p.stop()
			a.onMain(func() {
				a.busy = false
				a.cancel = nil
				reply.done = true
				reply.reasoningOpen = false
				a.status = ""
				if err != nil {
					a.errText = err.Error()
					ses.AppendEvent("error", map[string]any{"text": err.Error()})
				}
				if a.ag != nil {
					ses.SaveHistory(a.ag.History)
				}
				ses.Touch()
				a.refreshSessions()
			})
		},
	}
	go a.ag.Run(ctx, text, cbs)
}

// dropAgent discards the session agent, releasing its persistent shell.
func (a *app) dropAgent() {
	if a.ag != nil {
		if a.ag.Shell != nil {
			a.ag.Shell.Close()
		}
		a.ag = nil
	}
}

// currentSkills resolves the skill list for the active session's workspace.
func (a *app) currentSkills() []Skill {
	ws := a.cfg.WorkDir
	if a.ses != nil && a.ses.Meta.Workspace != "" {
		ws = a.ses.Meta.Workspace
	}
	return discoverSkills(a.dataDir, ws)
}

// skillControl backs the agent's skill tool (REQ-003): list the discovered
// SKILL.md packs or run one to load its instructions.
func (a *app) skillControl(action, name string) (string, bool) {
	skills := a.currentSkills()
	switch action {
	case "list":
		if len(skills) == 0 {
			return "(没有已安装的技能；把技能放到 <数据目录>/skills/<名称>/SKILL.md 或 <工作区>/.harness/skills/<名称>/SKILL.md)", false
		}
		var b strings.Builder
		for _, s := range skills {
			fmt.Fprintf(&b, "- %s（%s）— %s\n", s.Name, s.Source, s.Description)
		}
		return strings.TrimRight(b.String(), "\n"), false
	case "run":
		for _, s := range skills {
			if s.Name == name || s.ID == name {
				return "【技能：" + s.Name + "（" + s.Source + "）】请严格按以下指令执行：\n\n" + s.Body, false
			}
		}
		return "error: 未知技能 " + name + "；用 action=list 查看全部", true
	default:
		return "error: 未知 action " + action + "（可选 list / run）", true
	}
}

func (a *app) newAgentFor(ses *Session) *agent.Agent {
	preset := presetByID(a.presets, ses.Meta.Mode)
	var names []string
	for _, n := range preset.Tools {
		if a.enabledTool(n) {
			names = append(names, n)
		}
	}
	specs := tools.SpecsFor(names)
	for _, n := range preset.Tools {
		if n == "run_code" {
			specs = append(specs, agent.RunCodeSpec())
		}
	}
	prompt := applyCwdTemplate(preset.SystemPrompt, ses.Meta.Workspace, a.dataDir)
	base := agent.Agent{
		Client:       llm.New(a.cfg.BaseURL, a.cfg.APIKey),
		Model:        a.cfg.Model,
		Temperature:  a.cfg.Temperature,
		MaxTokens:    a.cfg.MaxTokens,
		SystemPrompt: prompt,
		WorkDir:      ses.Meta.Workspace,
		Specs:        specs,
	}
	// 持久 shell：极简（或自定义）preset 自带，或实验开关「持久终端」全局开启
	hasBash := false
	for _, n := range preset.Tools {
		if n == "bash" {
			hasBash = true
		}
	}
	if hasBash && a.enabledTool("bash") && (preset.PersistentShell || a.experimentalOn("persistent_terminals")) {
		base.Shell = tools.NewPersistentShell(ses.Meta.Workspace)
	}
	// 已安装插件包：spec + 运行器 + 审批标记（REQ-004）
	var pkgSpecs []llm.ToolSpec
	runners := map[string]func(ctx context.Context, argsJSON string) (string, bool){}
	mutating := map[string]bool{}
	workdir := ses.Meta.Workspace
	shell := base.Shell
	for _, pkg := range a.pkgCache {
		for _, tl := range pkg.Tools {
			if !a.cfg.packageChildOn(pkg.ID, tl.Name) {
				continue
			}
			tool := tl
			scriptPath := filepath.Join(pluginsRoot(a.dataDir), pkg.ID, tool.Script)
			runners[tool.Name] = func(ctx context.Context, argsJSON string) (string, bool) {
				b, rerr := os.ReadFile(scriptPath) //nolint:gosec
				if rerr != nil {
					return "error: 读取插件脚本失败: " + rerr.Error(), true
				}
				return agent.RunPackageTool(ctx, string(b), argsJSON, workdir, func(nm, aj string) (string, bool) {
					if nm == "bash" && shell != nil {
						return shell.Run(ctx, aj)
					}
					bt := tools.ByName(nm)
					if bt == nil {
						return "error: unknown tool " + nm, true
					}
					return bt.Run(ctx, aj, workdir)
				})
			}
			mutating[tool.Name] = tool.Mutating
			pkgSpecs = append(pkgSpecs, llm.ToolSpec{
				Type: "function",
				Function: llm.ToolDef{
					Name: tool.Name,
					Description: fmt.Sprintf("[插件包 %s v%s] %s",
						pkg.Name, pkg.Version, tool.Description),
					Parameters: map[string]any{"type": "object"},
				},
			})
		}
	}
	if len(pkgSpecs) > 0 {
		base.PackageSpecs = pkgSpecs
		base.PackageTools = runners
		base.MutatingTools = mutating
	}
	if preset.PersistentShell {
		base.Shell = tools.NewPersistentShell(ses.Meta.Workspace)
	}
	if preset.CanManagePlugins && a.enabledTool("plugins") {
		base.PluginTool = a.pluginControl
	}
	for _, n := range preset.Tools {
		if n == "skill" && a.enabledTool("skill") {
			base.SkillTool = a.skillControl
		}
	}
	// Every agent asks back which plugins are on, so toggles made in the
	// plugins page (or by the model itself) apply from the next message.
	base.PluginEnabled = a.enabledTool
	if h := ses.LoadHistory(); len(h) > 0 {
		base.History = h
	}
	ag := base
	ag.Subagent = func() *agent.Agent {
		child := base
		child.Depth = base.Depth + 1
		child.History = nil
		child.Subagent = nil
		return &child
	}
	return &ag
}

func (a *app) command(text string) {
	parts := strings.Fields(text)
	switch parts[0] {
	case "/help":
		a.input = ""
		a.note("可用命令：/new 新会话 · /compact 压缩历史 · /fork 分叉会话 · /mode <标准|PTC|极简|创造|…> 切换模式 · /cwd <目录> 切换工作区 · /trajectory 打开轨迹 · /help 本帮助")
	case "/new":
		a.input = ""
		a.newSession()
	case "/fork":
		a.input = ""
		a.forkSession()
	case "/trajectory":
		a.input = ""
		a.selectedID = "page:trajectory"
	case "/compact":
		a.input = ""
		a.compact()
	case "/mode":
		a.input = ""
		if len(parts) < 2 {
			a.errText = "用法：/mode <名称>，可选：" + strings.Join(presetLabels(a.presets), " / ")
			return
		}
		a.switchMode(strings.Join(parts[1:], " "))
	case "/cwd":
		a.input = ""
		if len(parts) < 2 {
			a.errText = "用法：/cwd <目录>"
			return
		}
		if a.ses == nil {
			a.errText = "先新建会话"
			return
		}
		a.ses.Meta.Workspace = parts[1]
		a.ses.Touch()
		a.refreshSessions()
		a.dropAgent()
		a.note("工作区已切换：" + parts[1])
	default:
		a.errText = "未知命令 " + parts[0] + "，/help 查看帮助"
	}
}

func (a *app) note(text string) {
	a.nextID++
	a.msgs = append(a.msgs, &uiMsg{id: a.nextID, role: "assistant", text: text, done: true})
	if a.ses != nil {
		a.ses.AppendEvent("note", map[string]any{"text": text})
	}
}

func (a *app) switchMode(label string) {
	if a.ses == nil {
		a.newSession()
		if a.ses == nil {
			return
		}
	}
	p := labelToPreset(a.presets, label)
	a.ses.Meta.Mode = p.ID
	a.ses.Touch()
	a.dropAgent()
	a.cfg.Mode = p.ID
	_ = saveConfig(a.cfg, a.cfgPath)
	a.refreshSessions()
	a.note("已切换到 " + p.Label + " 模式")
}

// compact summarizes the history with the model and replaces it.
func (a *app) compact() {
	if a.busy || a.ag == nil || len(a.ag.History) == 0 {
		a.errText = "没有可压缩的历史"
		return
	}
	a.busy = true
	a.status = "压缩历史中…"
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	var b strings.Builder
	for _, m := range a.ag.History {
		switch m.Role {
		case "user":
			b.WriteString("用户: " + m.Content + "\n")
		case "assistant":
			b.WriteString("助手: " + m.Content + "\n")
		case "tool":
			b.WriteString("工具结果: " + ellipsis(m.Content, 200) + "\n")
		}
	}
	transcript := b.String()
	if len(transcript) > 60000 {
		transcript = transcript[:60000]
	}
	p := a.pump
	ses := a.ses
	go func() {
		req := llm.Request{
			Model: a.cfg.Model,
			Messages: []llm.Message{
				{Role: "user", Content: "请把以下 agent 对话历史压缩成一份摘要，保留：任务目标、关键决定、涉及的文件路径、执行过的命令、未完成事项。直接输出摘要正文。\n\n" + transcript},
			},
			Temperature: 0.3,
		}
		msg, err := a.ag.Client.Chat(ctx, req, llm.StreamHandler{})
		p.stop()
		a.onMain(func() {
			a.busy = false
			a.cancel = nil
			a.status = ""
			if err != nil {
				a.errText = "压缩失败: " + err.Error()
				return
			}
			a.ag.Compact(msg.Content)
			ses.SaveHistory(a.ag.History)
			ses.AppendEvent("compact", map[string]any{"summary": ellipsis(msg.Content, 400)})
			a.note("历史已压缩：" + ellipsis(msg.Content, 60))
		})
	}()
}
