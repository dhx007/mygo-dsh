package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Plugin packages are directories under <dataDir>/plugins/<id>/ holding a
// plugin.json manifest plus JavaScript tool scripts. Each tool of a package
// is a child plugin (individually toggleable via enabled_tools key
// "<pkgid>/<tool>"; the package itself toggles "<pkgid>"). This mirrors the
// original's plugin manager model (npm/git sources, versions, child
// plugins) with a goja sandbox instead of Node.

type PackageTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating,omitempty"`
	Script      string `json:"script"`
}

type PluginPackage struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Source      string        `json:"source"` // local | git | npm
	Origin      string        `json:"origin,omitempty"`
	Description string        `json:"description"`
	Tools       []PackageTool `json:"tools"`
}

var pkgIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-_.]*$`)

func pluginsRoot(dataDir string) string { return filepath.Join(dataDir, "plugins") }

// loadManifest reads and validates one plugin.json.
func loadManifest(dir string) (PluginPackage, error) {
	b, err := os.ReadFile(filepath.Join(dir, "plugin.json")) //nolint:gosec
	if err != nil {
		return PluginPackage{}, fmt.Errorf("plugin.json: %w", err)
	}
	var p PluginPackage
	if err := json.Unmarshal(b, &p); err != nil {
		return PluginPackage{}, fmt.Errorf("plugin.json 解析失败: %w", err)
	}
	if !pkgIDRe.MatchString(p.ID) {
		return PluginPackage{}, fmt.Errorf("id %q 不合法（小写字母/数字/-/_.，字母开头）", p.ID)
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Version) == "" {
		return PluginPackage{}, fmt.Errorf("name 和 version 必填")
	}
	if p.Source == "" {
		p.Source = "local"
	}
	if len(p.Tools) == 0 {
		return PluginPackage{}, fmt.Errorf("tools 不能为空")
	}
	seen := map[string]bool{}
	for i := range p.Tools {
		t := &p.Tools[i]
		if !pkgIDRe.MatchString(t.Name) {
			return PluginPackage{}, fmt.Errorf("工具名 %q 不合法", t.Name)
		}
		if seen[t.Name] {
			return PluginPackage{}, fmt.Errorf("工具名重复: %s", t.Name)
		}
		seen[t.Name] = true
		if strings.TrimSpace(t.Script) == "" {
			return PluginPackage{}, fmt.Errorf("工具 %s 缺 script", t.Name)
		}
		if t.Description == "" {
			t.Description = "(包作者未提供描述)"
		}
	}
	return p, nil
}

// installedPackages scans the plugins root; broken packages are skipped.
func installedPackages(dataDir string) []PluginPackage {
	root := pluginsRoot(dataDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []PluginPackage
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if p, err := loadManifest(filepath.Join(root, e.Name())); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// installFromPath validates the source dir's manifest and copies the whole
// directory into the plugins root.
func installFromPath(dataDir, src string) (PluginPackage, error) {
	p, err := loadManifest(src)
	if err != nil {
		return PluginPackage{}, err
	}
	dst := filepath.Join(pluginsRoot(dataDir), p.ID)
	if _, err := os.Stat(dst); err == nil {
		return PluginPackage{}, fmt.Errorf("插件 %s 已安装，先卸载再安装", p.ID)
	}
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return PluginPackage{}, err
	}
	// The manifest copy is the installed truth; record the origin.
	p.Origin = src
	p.Source = "local"
	if err := saveManifest(dst, p); err != nil {
		_ = os.RemoveAll(dst)
		return PluginPackage{}, err
	}
	return p, nil
}

// installFromGit clones url into the plugins root and validates it.
func installFromGit(dataDir, url string) (PluginPackage, error) {
	if err := os.MkdirAll(pluginsRoot(dataDir), 0o755); err != nil {
		return PluginPackage{}, err
	}
	dst, err := os.MkdirTemp(pluginsRoot(dataDir), ".clone-")
	if err != nil {
		return PluginPackage{}, err
	}
	defer os.RemoveAll(dst) // no-op once renamed away
	cmd := exec.Command("git", "clone", "--depth", "1", url, dst)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return PluginPackage{}, fmt.Errorf("git clone 失败: %s", strings.TrimSpace(string(out)))
	}
	p, err := loadManifest(dst)
	if err != nil {
		return PluginPackage{}, err
	}
	final := filepath.Join(pluginsRoot(dataDir), p.ID)
	if _, err := os.Stat(final); err == nil {
		return PluginPackage{}, fmt.Errorf("插件 %s 已安装，先卸载再安装", p.ID)
	}
	if err := os.Rename(dst, final); err != nil {
		return PluginPackage{}, err
	}
	p.Origin = url
	p.Source = "git"
	if err := saveManifest(final, p); err != nil {
		_ = os.RemoveAll(final)
		return PluginPackage{}, err
	}
	return p, nil
}

func uninstallPackage(dataDir, id string) error {
	if !pkgIDRe.MatchString(id) {
		return fmt.Errorf("id %q 不合法", id)
	}
	return os.RemoveAll(filepath.Join(pluginsRoot(dataDir), id))
}

func saveManifest(dir string, p PluginPackage) error {
	b, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "plugin.json"), b, 0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p) //nolint:gosec
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644) //nolint:gosec
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// packageChildOn reports whether a package's child tool is enabled
// (enabled_tools key "<pkgid>/<tool>"; the package switch is "<pkgid>").
func (c *Config) packageChildOn(pkgID, tool string) bool {
	return c.enabledTool(pkgID+"/"+tool) && c.enabledTool(pkgID)
}

// packageState renders the package switch state the way the original does.
func (c *Config) packageState(p PluginPackage) string {
	on := 0
	for _, t := range p.Tools {
		if c.packageChildOn(p.ID, t.Name) {
			on++
		}
	}
	switch {
	case on == 0:
		return "Off"
	case on == len(p.Tools):
		return "On"
	default:
		return fmt.Sprintf("Partially on（%d/%d 子插件开）", on, len(p.Tools))
	}
}
