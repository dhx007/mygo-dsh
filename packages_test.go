package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writePluginSource writes a minimal valid plugin package source dir.
func writePluginSource(t *testing.T, dir, id string) {
	t.Helper()
	manifest := `{"id":"` + id + `","name":"示例包","version":"1.0.0","description":"测试用插件包",` +
		`"tools":[{"name":"demo_hello","description":"打招呼","mutating":false,"script":"hello.js"}]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.js"),
		[]byte("function run(args){ return \"hello \" + (args && args.who ? args.who : \"world\"); }"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// REQ-004: manifest validation rejects broken packages.
func TestManifestValidation(t *testing.T) {
	dir := t.TempDir()
	writePluginSource(t, dir, "demo-pack")
	if _, err := loadManifest(dir); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := map[string]string{
		"missing name":   `{"id":"x","version":"1.0.0","tools":[{"name":"a","script":"a.js"}]}`,
		"bad id":         `{"id":"Bad Id","name":"n","version":"1","tools":[{"name":"a","script":"a.js"}]}`,
		"empty tools":    `{"id":"x","name":"n","version":"1","tools":[]}`,
		"dup tools":      `{"id":"x","name":"n","version":"1","tools":[{"name":"a","script":"a.js"},{"name":"a","script":"b.js"}]}`,
		"missing script": `{"id":"x","name":"n","version":"1","tools":[{"name":"a"}]}`,
	}
	for label, body := range cases {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "plugin.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadManifest(d); err == nil {
			t.Errorf("%s: expected error", label)
		}
	}
}

// REQ-004 AC-3/4: local install copies the dir; conflicts and bad sources
// fail; uninstall removes; git install clones a local repository.
func TestInstallUninstallPackage(t *testing.T) {
	dataDir := t.TempDir()
	src := t.TempDir()
	writePluginSource(t, src, "demo-pack")

	p, err := installFromPath(dataDir, src)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if p.ID != "demo-pack" || p.Source != "local" {
		t.Fatalf("installed package: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "plugins", "demo-pack", "plugin.json")); err != nil {
		t.Fatalf("package not copied: %v", err)
	}
	if len(installedPackages(dataDir)) != 1 {
		t.Fatal("installedPackages should list 1")
	}
	// 源目录删除不影响已安装副本
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	if len(installedPackages(dataDir)) != 1 {
		t.Fatal("installed copy should survive source removal")
	}
	// 冲突
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writePluginSource(t, src, "demo-pack")
	if _, err := installFromPath(dataDir, src); err == nil || !strings.Contains(err.Error(), "已安装") {
		t.Fatalf("conflict: %v", err)
	}
	// 非法源
	bad := t.TempDir()
	if _, err := installFromPath(dataDir, bad); err == nil {
		t.Fatal("expected error for source without manifest")
	}
	// 卸载
	if err := uninstallPackage(dataDir, "demo-pack"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(installedPackages(dataDir)) != 0 {
		t.Fatal("package still listed after uninstall")
	}
}

func TestInstallFromGit(t *testing.T) {
	dataDir := t.TempDir()
	repo := t.TempDir()
	makeGitPluginRepo(t, repo, "git-pack")

	p, err := installFromGit(dataDir, repo)
	if err != nil {
		t.Fatalf("git install: %v", err)
	}
	if p.ID != "git-pack" || p.Source != "git" {
		t.Fatalf("installed: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "plugins", "git-pack", "hello.js")); err != nil {
		t.Fatalf("cloned files missing: %v", err)
	}
	// 半成品目录（.clone-*）不应残留
	entries, _ := os.ReadDir(filepath.Join(dataDir, "plugins"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".clone-") {
			t.Fatalf("clone temp dir leaked: %s", e.Name())
		}
	}
}

// makeGitPluginRepo writes a valid plugin package into dir and turns it
// into a local git repository (usable as a clone origin in tests).
func makeGitPluginRepo(t *testing.T, dir, id string) {
	t.Helper()
	writePluginSource(t, dir, id)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("add", ".")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "init")
}

// REQ-004 FR-3: package state mirrors the original's On / Partially on / Off.
func TestPackageState(t *testing.T) {
	pkg := PluginPackage{ID: "x", Tools: []PackageTool{{Name: "a"}, {Name: "b"}}}
	cfg := &Config{}
	if got := cfg.packageState(pkg); got != "On" {
		t.Fatalf("default state = %q, want On", got)
	}
	cfg.EnabledTools = map[string]bool{"x/a": true, "x/b": false}
	if got := cfg.packageState(pkg); !strings.Contains(got, "Partially on") || !strings.Contains(got, "1/2") {
		t.Fatalf("partial state = %q", got)
	}
	cfg.EnabledTools["x"] = false
	if got := cfg.packageState(pkg); got != "Off" {
		t.Fatalf("off state = %q", got)
	}
}
