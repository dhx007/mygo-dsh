package main

import (
	"testing"

	"github.com/egoist/mygo"
)

func findItem(m *mygo.Menu, id string) *mygo.MenuItem {
	for _, it := range m.Items() {
		if it.ID == id {
			return it
		}
		for _, sub := range it.Submenu {
			if sub.ID == id {
				return sub
			}
		}
	}
	return nil
}

// REQ-012 AC-1: accelerators and roles are complete.
func TestAppMenuAccelerators(t *testing.T) {
	a := newTestApp(t)
	menu := buildAppMenu(a)

	cases := map[string]string{
		"menu-new-session": "Alt+CmdOrCtrl+N",
		"menu-settings":    "Alt+CmdOrCtrl+,",
		"menu-chat":        "CmdOrCtrl+1",
		"menu-trajectory":  "CmdOrCtrl+2",
		"menu-plugins":     "CmdOrCtrl+3",
	}
	for id, acc := range cases {
		item := findItem(menu, id)
		if item == nil {
			t.Errorf("%s missing", id)
			continue
		}
		if item.Accelerator != acc {
			t.Errorf("%s accelerator = %q, want %q", id, item.Accelerator, acc)
		}
		if item.Click == nil {
			t.Errorf("%s has no Click", id)
		}
	}

	hasEdit, hasClose := false, false
	for _, it := range menu.Items() {
		if it.Role == mygo.RoleEditMenu {
			hasEdit = true
		}
		for _, sub := range it.Submenu {
			if sub.Role == mygo.RoleClose {
				hasClose = true
			}
		}
	}
	if !hasEdit || !hasClose {
		t.Errorf("roles missing: edit=%v close=%v", hasEdit, hasClose)
	}
}

// REQ-012 AC-2: clicking menu items runs the existing actions.
func TestAppMenuActions(t *testing.T) {
	a := newTestApp(t)
	menu := buildAppMenu(a)

	if item := findItem(menu, "menu-new-session"); item != nil {
		item.Click(item, nil)
	}
	if a.ses == nil {
		t.Fatal("new session action did not run")
	}

	if item := findItem(menu, "menu-trajectory"); item != nil {
		item.Click(item, nil)
	}
	if a.selectedID != "page:trajectory" {
		t.Fatalf("navigate: %q", a.selectedID)
	}

	// 无头模式 openSettings 回退主区设置页
	if item := findItem(menu, "menu-settings"); item != nil {
		item.Click(item, nil)
	}
	if a.page != "settings" {
		t.Fatalf("settings: page=%q", a.page)
	}
}

// REQ-012 FR-4: headless never mounts the menu (mountAppMenu is a no-op).
func TestMountAppMenuHeadlessNoop(t *testing.T) {
	a := newTestApp(t)
	mountAppMenu(a) // 不应 panic
}
