package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	focusComposer int       // frames left to hand keyboard focus back to the composer
	savedAt       time.Time // last successful settings save
	userMenu      bool      // user-row popup menu open

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
		theme:         dshTheme(),
		page:          "chat",
		alwaysAllow:   map[string]bool{},
		toolNames:     map[string]string{},
		wsOpen:        map[string]*bool{},
		navOpen:       true,
		trajOpen:      map[int]*bool{},
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
	return a
}

func (a *app) refreshSessions() { a.sessions = a.store.List() }

// onMain runs fn on the UI thread; headless tests (no window) run inline.
func (a *app) onMain(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

func (a *app) enabledTool(name string) bool { return a.cfg.enabledTool(name) }

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
	a.ag = nil
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
	a.ag = nil
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

func (a *app) pluginsView(c *ui.Context) {
	t := c.Theme()
	toolDesc := map[string]string{}
	for _, tl := range tools.All() {
		toolDesc[tl.Name] = tl.Description
	}
	toolDesc["todo"] = "任务清单：维护本会话的 plan checklist"
	toolDesc["task"] = "子代理：把自包含的子任务委托给一个子 agent"
	toolDesc["run_code"] = "PTC：模型写一段脚本，把多次工具操作合并成一次执行"
	allNames := []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep", "web_fetch", "web_search", "todo", "task", "run_code"}

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "插件 · 一切皆插件").Bold()
			ui.Spacer(c)
		})
		ui.List(c, &a.pluginList, len(allNames)+1, func(i int) {
			if i == len(allNames) {
				ui.Column(c).Gap(4).Padding(14).Children(func() {
					ui.Text(c, "与初代一样，模型、工具、技能、会话、沙箱都是配置项；").TextColor(t.TextMuted).FontSize(12)
					ui.Text(c, "上面的开关决定模式挂载哪些内置插件。社区插件生态见 dsh-plugin 主题。").TextColor(t.TextMuted).FontSize(12)
				})
				return
			}
			name := allNames[i]
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, name).Bold().FontSize(13)
					ui.Text(c, ellipsis(toolDesc[name], 100)).TextColor(t.TextMuted).FontSize(11)
				})
				ui.Switch(c, a.pluginOn(name))
				if *a.pluginOn(name) != a.enabledTool(name) {
					a.setEnabled(name, *a.pluginOn(name))
				}
			})
		}).Grow(1).Padding(8, 8)
	})
}

func (a *app) setEnabled(name string, on bool) {
	if a.cfg.EnabledTools == nil {
		a.cfg.EnabledTools = map[string]bool{}
	}
	a.cfg.EnabledTools[name] = on
	_ = saveConfig(a.cfg, a.cfgPath)
}

// ---- settings view ----

func (a *app) settingsView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 16).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "设置").Bold()
			ui.Spacer(c)
		})
		ui.List(c, &a.settingsList, 1, func(i int) {
			ui.Column(c).Fill().Padding(20, 24).Gap(16).Children(func() {
				// 模型
				ui.Column(c).Gap(10).Padding(16).Radius(10).Background(t.Surface).Children(func() {
					ui.Text(c, "模型 · DeepSeek").Bold()
					ui.Text(c, "API Key（明文保存在 "+a.cfgPath+"）").TextColor(t.TextMuted).FontSize(12)
					ui.TextInput(c, &a.fKey).Password().Placeholder("sk-…").Label("API Key 输入框")
					ui.Text(c, "API Base URL（兼容 OpenAI 接口的服务都可填）").TextColor(t.TextMuted).FontSize(12)
					ui.TextInput(c, &a.fURL).Placeholder("https://api.deepseek.com").Label("Base URL 输入框")
					ui.Text(c, "模型").TextColor(t.TextMuted).FontSize(12)
					ui.Select(c, &a.fModel, []string{"deepseek-chat", "deepseek-reasoner"})
				})
				// 工作区
				ui.Column(c).Gap(10).Padding(16).Radius(10).Background(t.Surface).Children(func() {
					ui.Text(c, "默认工作区").Bold()
					ui.Text(c, "新建会话的目录；bash 的 cwd 与相对路径的根").TextColor(t.TextMuted).FontSize(12)
					ui.TextInput(c, &a.fWorkDir).Label("默认工作区输入框")
				})
				// 权限
				ui.Column(c).Gap(8).Padding(16).Radius(10).Background(t.Surface).Children(func() {
					ui.Text(c, "工具权限").Bold()
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Checkbox(c, &a.autoApprove, "")
						ui.Text(c, "自动批准变更类工具（bash / 写文件 / 编辑）——不再逐次询问").FontSize(13)
					})
				})
				// 模式
				ui.Column(c).Gap(8).Padding(16).Radius(10).Background(t.Surface).Children(func() {
					ui.Text(c, "模式").Bold()
					p := a.currentPreset()
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, p.Label).FontSize(13)
						ui.Text(c, p.Description).TextColor(t.TextMuted).FontSize(12)
						ui.Spacer(c)
					})
					ui.Row(c).Gap(8).Wrap().Children(func() {
						ui.Text(c, "挂载工具：").TextColor(t.TextMuted).FontSize(12)
						for _, n := range p.Tools {
							ui.Badge(c, n)
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
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
				// save
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
			})
		}).Grow(1)
	})
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
		a.ag = nil
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
	a.ag = nil
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
