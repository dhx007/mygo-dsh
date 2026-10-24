package main

import "github.com/egoist/mygo/ui"

// dshTheme reproduces the DeepSeek Harness desktop look from the current
// app: a warm near-black canvas, raised surfaces, hairline borders and a
// single warm-orange action accent (the send button, active tab).
// dshTheme returns the theme for a skin variant. 暖黑 is the default warm
// dark look; 深海 and 森绿 recolor the canvas and accent.
func dshTheme(variant string) *ui.Theme {
	switch variant {
	case "深海":
		return recolor("#10141d", "#181d29", "#232a3a", "#2c3448", "#232a3a",
			"#eef2f9", "#93a0b5", "#5686fe", "#7aa0ff", "#4370d6", "#0d1220",
			0x56, 0x86, 0xfe)
	case "森绿":
		return recolor("#0f1411", "#17201b", "#223029", "#2b3d33", "#233029",
			"#eef5f0", "#9ab0a3", "#34c98a", "#5cd8a6", "#28a974", "#0a120e",
			0x34, 0xc9, 0x8a)
	}
	return dshBase()
}

// recolor derives a variant from the base dark theme by swapping the
// canvas/surface/border/ink/accent colors.
func recolor(bg, surface, hover, pressed, border, text, muted, accent, accentHover, accentPressed, accentText string, ar, ag, ab uint8) *ui.Theme {
	t := dshBase()
	t.Background = ui.Hex(bg)
	t.Surface = ui.Hex(surface)
	t.SurfaceHover = ui.Hex(hover)
	t.SurfacePressed = ui.Hex(pressed)
	t.Border = ui.Hex(border)
	t.Text = ui.Hex(text)
	t.TextMuted = ui.Hex(muted)
	t.Accent = ui.Hex(accent)
	t.AccentHover = ui.Hex(accentHover)
	t.AccentPressed = ui.Hex(accentPressed)
	t.AccentText = ui.Hex(accentText)
	t.Selection = ui.RGBA(ar, ag, ab, 0.35)
	t.Focus = ui.RGBA(ar, ag, ab, 0.55)
	return t
}

func dshBase() *ui.Theme {
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
