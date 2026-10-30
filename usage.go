package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Usage accounting (REQ-015): per-local-day totals captured from the LLM
// responses, persisted to <dataDir>/usage.json and shown in the sidebar
// card. Cost is an estimate from deepseek-chat list prices (documented
// assumption; the real bill may differ).

type usageDay struct {
	Calls            int `json:"calls"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type usageFile struct {
	Days map[string]*usageDay `json:"days"`
}

// 估算牌价（每百万 token）：deepseek-chat 公开牌价假设。
const (
	pricePerMInput  = 2.0
	pricePerMOutput = 8.0
)

func usagePath(dataDir string) string { return filepath.Join(dataDir, "usage.json") }

func loadUsage(dataDir string) map[string]*usageDay {
	b, err := os.ReadFile(usagePath(dataDir)) //nolint:gosec
	if err != nil {
		return map[string]*usageDay{}
	}
	var f usageFile
	if json.Unmarshal(b, &f) != nil || f.Days == nil {
		return map[string]*usageDay{}
	}
	return f.Days
}

func saveUsage(dataDir string, days map[string]*usageDay) error {
	if days == nil {
		days = map[string]*usageDay{}
	}
	b, err := json.MarshalIndent(&usageFile{Days: days}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(usagePath(dataDir), b, 0o644)
}

func todayKey() string { return time.Now().Format("2006-01-02") }

// estimateCost renders the estimated cost of one day's usage.
func estimateCost(d usageDay) float64 {
	return (float64(d.PromptTokens)*pricePerMInput + float64(d.CompletionTokens)*pricePerMOutput) / 1e6
}

// fmtTokens humanizes a token count (13.6M / 1.2K / 876).
func fmtTokens(n int) string {
	switch {
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
