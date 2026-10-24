package main

import "github.com/egoist/mygo/ui"

// dshTheme reproduces the DeepSeek Harness desktop look from the current
// app: a warm near-black canvas, raised surfaces, hairline borders and a
// single warm-orange action accent (the send button, active tab).
func dshTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Dark = true
	t.Background = ui.Hex("#131110")     // warm near-black canvas
	t.Surface = ui.Hex("#1e1b19")        // sidebar / raised cards
	t.SurfaceHover = ui.Hex("#2a2624")   // hover on rows and controls
	t.SurfacePressed = ui.Hex("#332e2b") // pressed
	t.Border = ui.Hex("#2e2a27")         // hairline borders
	t.Text = ui.Hex("#f3f0ea")           // warm white
	t.TextMuted = ui.Hex("#a39d95")      // muted warm gray
	t.Accent = ui.Hex("#e85d2a")         // action orange (send / active tab)
	t.AccentHover = ui.Hex("#ff7040")
	t.AccentPressed = ui.Hex("#c94e23")
	t.AccentText = ui.Hex("#171310")
	t.Danger = ui.Hex("#f25a5a")
	t.Warning = ui.Hex("#f7ad31")
	t.Success = ui.Hex("#22c55e")
	t.Selection = ui.RGBA(232, 93, 42, 0.35)
	t.Focus = ui.RGBA(255, 112, 64, 0.55)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.20)
	t.Radius = 10
	return t
}

// Secondary label color, for captions that are muted but still readable.
var dshLabelSecondary = ui.Hex("#cfc9c2")
