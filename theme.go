package main

import "github.com/egoist/mygo/ui"

// dshTheme reproduces the original DeepSeek Harness dark design tokens
// (dsw-neutral-bluish scale, deepseek brand blues).
func dshTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Dark = true
	t.Background = ui.Hex("#151517")     // neutral-bluish-950
	t.Surface = ui.Hex("#2c2c2e")        // neutral-bluish-850: controls, cards
	t.SurfaceHover = ui.Hex("#353638")   // neutral-bluish-800
	t.SurfacePressed = ui.Hex("#3c3c3d") // neutral-bluish-700
	t.Border = ui.Hex("#2e2e31")         // ≈ border-l1 (#ffffff0f) over the base
	t.Text = ui.Hex("#f9fafb")           // neutral-bluish-50: label-primary
	t.TextMuted = ui.Hex("#adb2b8")      // neutral-bluish-400: label-tertiary
	t.Accent = ui.Hex("#5686fe")         // deepseek-450
	t.AccentHover = ui.Hex("#7aaaff")    // deepseek-400
	t.AccentPressed = ui.Hex("#4176e6")  // deepseek-500
	t.AccentText = ui.Hex("#0f1115")     // neutral-bluish-1000
	t.Danger = ui.Hex("#f25a5a")         // red-400
	t.Warning = ui.Hex("#f7ad31")        // amber-400
	t.Success = ui.Hex("#22c55e")        // green-500
	t.Selection = ui.RGBA(86, 134, 254, 0.40)
	t.Focus = ui.RGBA(122, 170, 255, 0.60)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.22)
	t.Radius = 8
	return t
}

// Secondary label color of the DSH theme (neutral-bluish-300), for text
// that is muted but still readable, as the original uses for captions.
var dshLabelSecondary = ui.Hex("#cfd3d6")
