package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Plugin market: user-added indexes (URLs or local files) listing installable
// plugin packages. This is the equivalent of the original's "Custom plugin
// markets" experimental plugin — a Discover surface fed by indexes you add.
// Index content is untrusted display data; installing is an explicit user
// action that reuses all REQ-004 package validation.

type MarketEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Source      string `json:"source"` // git | local | npm
	Origin      string `json:"origin"` // git URL or local path
}

type marketIndexFile struct {
	Name    string        `json:"name"`
	Plugins []MarketEntry `json:"plugins"`
}

type marketsFile struct {
	Indexes []string `json:"indexes"`
}

func marketsConfigPath(dataDir string) string { return filepath.Join(dataDir, "markets.json") }

func loadMarketIndexes(dataDir string) []string {
	b, err := os.ReadFile(marketsConfigPath(dataDir)) //nolint:gosec
	if err != nil {
		return nil
	}
	var f marketsFile
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	return f.Indexes
}

func saveMarketIndexes(dataDir string, indexes []string) error {
	if indexes == nil {
		indexes = []string{}
	}
	b, err := json.MarshalIndent(&marketsFile{Indexes: indexes}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(marketsConfigPath(dataDir), b, 0o644)
}

var marketHTTPClient = &http.Client{Timeout: 10 * time.Second}

// fetchMarketIndex reads one index (http(s) URL or local file) and returns
// its valid entries.
func fetchMarketIndex(src string) ([]MarketEntry, error) {
	var body []byte
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequest(http.MethodGet, src, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) DeepSeekHarness/0.1")
		resp, err := marketHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		body, err = io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		if err != nil {
			return nil, err
		}
	} else {
		b, err := os.ReadFile(src) //nolint:gosec
		if err != nil {
			return nil, err
		}
		body = b
	}
	var f marketIndexFile
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("索引不是合法 JSON: %w", err)
	}
	var out []MarketEntry
	for _, e := range f.Plugins {
		if !pkgIDRe.MatchString(e.ID) || strings.TrimSpace(e.Origin) == "" {
			continue // 非法条目跳过
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		if e.Version == "" {
			e.Version = "0.0.0"
		}
		if e.Source == "" {
			e.Source = "git"
		}
		out = append(out, e)
	}
	return out, nil
}

// fetchAllIndexes merges every index, deduping by id (first index wins).
// All sources are tried; the first error is reported alongside results.
func fetchAllIndexes(srcs []string) (entries []MarketEntry, err error) {
	seen := map[string]bool{}
	for _, src := range srcs {
		list, ferr := fetchMarketIndex(src)
		if ferr != nil {
			if err == nil {
				err = fmt.Errorf("%s: %w", src, ferr)
			}
			continue
		}
		for _, e := range list {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			entries = append(entries, e)
		}
	}
	return entries, err
}

// installFromMarketEntry installs one market entry via its origin.
func installFromMarketEntry(dataDir string, e MarketEntry) (PluginPackage, error) {
	if _, err := os.Stat(filepath.Join(pluginsRoot(dataDir), e.ID)); err == nil {
		return PluginPackage{}, fmt.Errorf("插件 %s 已安装，先卸载再安装", e.ID)
	}
	switch {
	case e.Source == "git":
		return installFromGit(dataDir, e.Origin)
	case e.Source == "npm":
		return PluginPackage{}, fmt.Errorf("npm 来源暂不支持安装（可手动 git clone 后本地导入）")
	default:
		if st, serr := os.Stat(e.Origin); serr != nil || !st.IsDir() {
			return PluginPackage{}, fmt.Errorf("本地目录不存在：%s", e.Origin)
		}
		return installFromPath(dataDir, e.Origin)
	}
}
