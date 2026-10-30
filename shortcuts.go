package main

import (
	"log"

	"github.com/egoist/mygo"
)

// Application menu bar: keyboard shortcuts ride menu accelerators (the
// original's dsh-client-shortcuts behavior — ⌥⌘N new session, ⌥⌘, settings).
// Click handlers run on the main thread and reuse the existing actions.

func navigateAction(a *app, page string) func(*mygo.MenuItem, *mygo.Window) {
	return func(*mygo.MenuItem, *mygo.Window) {
		a.selectedID = "page:" + page
	}
}

// buildAppMenu assembles the application menu template (pure data, safe to
// construct headless for tests).
func buildAppMenu(a *app) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{ID: "menu-new-session", Label: "新会话", Accelerator: "Alt+CmdOrCtrl+N",
				Click: func(*mygo.MenuItem, *mygo.Window) { a.newSession() }},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "显示", Submenu: []*mygo.MenuItem{
			{ID: "menu-chat", Label: "对话", Accelerator: "CmdOrCtrl+1", Click: navigateAction(a, "chat")},
			{ID: "menu-trajectory", Label: "轨迹", Accelerator: "CmdOrCtrl+2", Click: navigateAction(a, "trajectory")},
			{ID: "menu-plugins", Label: "插件", Accelerator: "CmdOrCtrl+3", Click: navigateAction(a, "plugins")},
			mygo.Separator(),
			{ID: "menu-settings", Label: "设置", Accelerator: "Alt+CmdOrCtrl+,",
				Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
		}},
	})
}

// mountAppMenu attaches the menu bar in windowed mode; failure costs only
// the shortcuts (MyGo keeps a default menu).
func mountAppMenu(a *app) {
	if a.win == nil {
		return
	}
	if menu := buildAppMenu(a); menu != nil {
		mygo.App.SetMenu(menu)
	} else {
		log.Println("应用菜单构建失败，使用默认菜单")
	}
}
