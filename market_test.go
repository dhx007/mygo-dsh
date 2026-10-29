package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-006 AC-3: index parsing — valid entries, invalid skipped, bad JSON
// reported, multi-index merge dedupes by id.
func TestFetchMarketIndex(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"name":"t","plugins":[
		{"id":"a","name":"A","version":"1.0.0","description":"第一个","source":"git","origin":"/tmp/a.git"},
		{"id":"bad","origin":""},
		{"id":"b","origin":"/tmp/b"}
	]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := fetchMarketIndex(good)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// bad（缺 origin）跳过；b（缺 name/version/source）回退
	if len(entries) != 2 || entries[0].ID != "a" {
		t.Fatalf("entries: %+v", entries)
	}
	if entries[1].Name != "b" || entries[1].Version != "0.0.0" || entries[1].Source != "git" {
		t.Fatalf("defaults: %+v", entries[1])
	}

	badJSON := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badJSON, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchMarketIndex(badJSON); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("bad json: %v", err)
	}

	// 合并去重：第二个索引里的 a 被丢弃
	second := filepath.Join(dir, "second.json")
	if err := os.WriteFile(second, []byte(`{"name":"s","plugins":[
		{"id":"a","name":"A2","version":"9.9.9","source":"git","origin":"/tmp/a2.git"}
	]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, merr := fetchAllIndexes([]string{good, second, badJSON})
	if merr == nil || !strings.Contains(merr.Error(), "bad.json") {
		t.Fatalf("aggregate error: %v", merr)
	}
	if len(merged) != 2 || merged[0].Name != "A" {
		t.Fatalf("dedupe: %+v", merged)
	}
	// 全部失败 → 无条目且有错
	none, nerr := fetchAllIndexes([]string{badJSON})
	if len(none) != 0 || nerr == nil {
		t.Fatalf("all-fail: %v %v", none, nerr)
	}
}

// REQ-006 AC-2: markets.json roundtrip.
func TestMarketsConfigRoundtrip(t *testing.T) {
	dataDir := t.TempDir()
	if len(loadMarketIndexes(dataDir)) != 0 {
		t.Fatal("missing file should give no indexes")
	}
	if err := saveMarketIndexes(dataDir, []string{"https://x/m.json", "/tmp/i.json"}); err != nil {
		t.Fatal(err)
	}
	got := loadMarketIndexes(dataDir)
	if len(got) != 2 || got[1] != "/tmp/i.json" {
		t.Fatalf("roundtrip: %v", got)
	}
}

// REQ-006 AC-4: market install dispatches by source; npm is rejected.
func TestInstallFromMarket(t *testing.T) {
	dataDir := t.TempDir()
	repo := t.TempDir()
	makeGitPluginRepo(t, repo, "mkt-pack")

	p, err := installFromMarketEntry(dataDir, MarketEntry{
		ID: "mkt-pack", Name: "市场包", Version: "1.0.0", Source: "git", Origin: repo,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if p.ID != "mkt-pack" || p.Source != "git" {
		t.Fatalf("installed: %+v", p)
	}
	if len(installedPackages(dataDir)) != 1 {
		t.Fatal("package not installed")
	}
	// 重复安装拒绝
	if _, err := installFromMarketEntry(dataDir, MarketEntry{ID: "mkt-pack", Source: "git", Origin: repo}); err == nil || !strings.Contains(err.Error(), "已安装") {
		t.Fatalf("conflict: %v", err)
	}
	// npm 来源拒绝
	if _, err := installFromMarketEntry(dataDir, MarketEntry{ID: "npm-pack", Source: "npm", Origin: "https://registry.npmjs.org/x"}); err == nil || !strings.Contains(err.Error(), "npm") {
		t.Fatalf("npm: %v", err)
	}
}
