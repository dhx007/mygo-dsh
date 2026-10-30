package main

import (
	"os"
	"strings"
	"testing"

	"mygo-harness/internal/llm"
)

// REQ-015 AC-2: recordUsage accumulates into today's bucket, persists to
// usage.json, and never mixes other days.
func TestRecordUsage(t *testing.T) {
	a := newTestApp(t)
	a.recordUsage(llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500})
	a.recordUsage(llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150})

	today := a.usageAll[todayKey()]
	if today == nil || today.Calls != 2 || today.PromptTokens != 1100 || today.CompletionTokens != 550 {
		t.Fatalf("today: %+v", today)
	}
	if a.usageUpdated.IsZero() {
		t.Fatal("usageUpdated not stamped")
	}
	// 落盘
	b, err := os.ReadFile(usagePath(a.dataDir))
	if err != nil {
		t.Fatalf("usage.json: %v", err)
	}
	if !strings.Contains(string(b), todayKey()) || !strings.Contains(string(b), `"calls": 2`) {
		t.Fatalf("persisted: %s", b)
	}
	// 昨日数据不混入
	a.usageAll["2026-01-01"] = &usageDay{Calls: 9}
	if got := estimateCost(*today); got <= 0 {
		t.Fatalf("estimate: %v", got)
	}
	if fmtTokens(1600000) != "1.6M" || fmtTokens(1500) != "1.5K" || fmtTokens(876) != "876" {
		t.Fatal("fmtTokens broken")
	}
}
