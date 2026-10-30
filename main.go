// DeepSeek Harness — a native desktop replica of the original DeepSeek
// Harness v0.1, drawn by MyGo's native UI toolkit: no webview, no HTML,
// no cgo. Sessions grouped by workspace, four agent modes, a timeline of
// tool calls, an append-only Trajectory, tool approvals and /commands.
//
//	go run .
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.SetName("DeepSeek Harness")
	mygo.App.SetVersion("0.2.0")

	a := newApp()

	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "DeepSeek Harness",
			Width:     1200,
			Height:    780,
			MinWidth:  960,
			MinHeight: 600,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
		mountAppMenu(a)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
