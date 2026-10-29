package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// Config persists the app settings as JSON in the user data directory.
type Config struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	WorkDir string `json:"work_dir"`

	Temperature  float64         `json:"temperature"`
	MaxTokens    int             `json:"max_tokens"`
	Mode         string          `json:"mode"`
	EnabledTools map[string]bool `json:"enabled_tools,omitempty"`
	AutoApprove  bool            `json:"auto_approve"`
	Theme        string          `json:"theme"`
	// Experimental holds the plugins-page experimental switches (e.g.
	// "persistent_terminals" — keep shells alive across tool calls for
	// every preset, the original's experimental plugin).
	Experimental map[string]bool `json:"experimental,omitempty"`
}

func defaultConfig() *Config {
	wd, _ := os.Getwd()
	return &Config{
		BaseURL:     "https://api.deepseek.com",
		Model:       "deepseek-chat",
		WorkDir:     wd,
		Temperature: 0.7,
		MaxTokens:   8192,
		Mode:        "标准",
		Theme:       "暖黑",
	}
}

// configPath resolves <user data>/config.json, falling back to the
// standard user config directory when MyGo's path is not available yet.
func configPath() string {
	dir := userDataDir()
	if dir == "" {
		if base, err := os.UserConfigDir(); err == nil {
			dir = filepath.Join(base, "mygo-harness")
		} else {
			dir = "."
		}
	}
	return filepath.Join(dir, "config.json")
}

func userDataDir() (dir string) {
	defer func() { _ = recover() }() // App.Path before App.Run may panic on some backends
	d, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || d == "" {
		return ""
	}
	return d
}

func loadConfig() (*Config, string) {
	p := configPath()
	cfg := defaultConfig()
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, cfg)
	}
	if cfg.APIKey == "" {
		if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
			cfg.APIKey = k
		}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.WorkDir == "" {
		wd, _ := os.Getwd()
		cfg.WorkDir = wd
	}
	if cfg.Mode == "" {
		cfg.Mode = "标准"
	}
	return cfg, p
}

func saveConfig(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// enabledTool reports whether a built-in tool (a "plugin") is on. A nil
// map means everything is enabled.
func (c *Config) enabledTool(name string) bool {
	if c.EnabledTools == nil {
		return true
	}
	v, ok := c.EnabledTools[name]
	if !ok {
		return true
	}
	return v
}

// experimentalOn reports whether an experimental switch is enabled.
func (c *Config) experimentalOn(name string) bool {
	return c.Experimental != nil && c.Experimental[name]
}
