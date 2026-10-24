// Package tools implements the agent's built-in tools: bash, read_file,
// write_file, glob and grep, plus their JSON schemas for the API.
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mygo-harness/internal/llm"
)

// Tool is one tool the agent can call. Run returns the result text and
// whether it counts as a failure (for the UI marker). Mutating tools
// require user approval before they run, as the original harness gates
// shell and file writes.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Mutating    bool
	Run         func(ctx context.Context, argsJSON, cwd string) (string, bool)
}

const maxOutput = 16 * 1024

func truncate(s string) string {
	if len(s) <= maxOutput {
		return s
	}
	return s[:maxOutput] + fmt.Sprintf("\n…[输出已截断，共 %d 字节]", len(s))
}

func resolve(cwd, p string) string {
	if p == "" {
		return cwd
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

func args(_ context.Context, argsJSON string, v any) error {
	if strings.TrimSpace(argsJSON) == "" {
		return nil
	}
	return json.Unmarshal([]byte(argsJSON), v)
}

var all = []Tool{
	{
		Name: "bash",
		Description: "Run a shell command with bash and return combined stdout/stderr. " +
			"Relative paths resolve against the working directory. " +
			"Use this for builds, tests, git, listing directories, and so on.",
		Mutating: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":         map[string]any{"type": "string", "description": "The bash command to run."},
				"timeout_seconds": map[string]any{"type": "integer", "description": "Timeout in seconds, default 60, max 600."},
			},
			"required": []string{"command"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Command   string `json:"command"`
				TimeoutOK int    `json:"timeout_seconds"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if strings.TrimSpace(a.Command) == "" {
				return "error: command is empty", true
			}
			timeout := 60 * time.Second
			if a.TimeoutOK > 0 {
				timeout = time.Duration(min(a.TimeoutOK, 600)) * time.Second
			}
			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cmd := exec.CommandContext(cctx, "bash", "-c", a.Command)
			cmd.Dir = cwd
			cmd.Env = os.Environ()
			out, err := cmd.CombinedOutput()
			text := truncate(string(out))
			if err != nil {
				text += fmt.Sprintf("\n[exit] %v", err)
				return text, true
			}
			if text == "" {
				text = "(no output)"
			}
			return text, false
		},
	},
	{
		Name: "read_file",
		Description: "Read a text file, with line numbers. " +
			"Use offset_line and limit to page through large files (default limit 400 lines).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string", "description": "File path, absolute or relative to the working directory."},
				"offset_line": map[string]any{"type": "integer", "description": "1-based first line to return."},
				"limit":       map[string]any{"type": "integer", "description": "Max lines to return, default 400, max 2000."},
			},
			"required": []string{"path"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Path       string `json:"path"`
				OffsetLine int    `json:"offset_line"`
				Limit      int    `json:"limit"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if a.Path == "" {
				return "error: path is empty", true
			}
			if a.OffsetLine < 1 {
				a.OffsetLine = 1
			}
			if a.Limit <= 0 {
				a.Limit = 400
			}
			a.Limit = min(a.Limit, 2000)
			f, err := os.Open(resolve(cwd, a.Path))
			if err != nil {
				return "error: " + err.Error(), true
			}
			defer f.Close()
			var buf bytes.Buffer
			sc := bufio.NewScanner(f) //nolint:gosec
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			line := 0
			shown := 0
			for sc.Scan() {
				line++
				if line < a.OffsetLine {
					continue
				}
				if shown >= a.Limit {
					fmt.Fprintf(&buf, "…[只显示了 %d 行，从第 %d 行起；用 offset_line 继续读]\n", shown, a.OffsetLine+shown-1)
					break
				}
				fmt.Fprintf(&buf, "%6d\t%s\n", line, sc.Text())
				shown++
				if buf.Len() > maxOutput {
					buf.WriteString("…[输出已截断]")
					break
				}
			}
			if err := sc.Err(); err != nil {
				return "error: " + err.Error(), true
			}
			if shown == 0 && buf.Len() == 0 {
				return fmt.Sprintf("(文件共 %d 行，第 %d 行之后没有内容)", line, a.OffsetLine-1), false
			}
			return buf.String(), false
		},
	},
	{
		Name:        "write_file",
		Description: "Create or overwrite a text file with content. Parent directories are created as needed.",
		Mutating:    true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "File path, absolute or relative to the working directory."},
				"content": map[string]any{"type": "string", "description": "Full content to write."},
			},
			"required": []string{"path", "content"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if a.Path == "" {
				return "error: path is empty", true
			}
			p := resolve(cwd, a.Path)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "error: " + err.Error(), true
			}
			if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
				return "error: " + err.Error(), true
			}
			return fmt.Sprintf("已写入 %s（%d 字节）", p, len(a.Content)), false
		},
	},
	{
		Name: "glob",
		Description: "Find files by a glob pattern, relative to the working directory (or path). " +
			"Supports ** across directories, e.g. \"**/*.go\" or \"src/**/*.ts\". Returns up to 500 paths.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern, e.g. **/*.go."},
				"path":    map[string]any{"type": "string", "description": "Root directory to search, default the working directory."},
			},
			"required": []string{"pattern"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if a.Pattern == "" {
				return "error: pattern is empty", true
			}
			root := resolve(cwd, a.Path)
			var out []string
			count := 0
			err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // skip unreadable entries
				}
				if d.IsDir() {
					if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
						return fs.SkipDir
					}
					return nil
				}
				rel, rerr := filepath.Rel(root, p)
				if rerr != nil {
					return nil
				}
				if matchPath(a.Pattern, rel) {
					count++
					if count <= 500 {
						out = append(out, rel)
					}
				}
				return nil
			})
			if err != nil {
				return "error: " + err.Error(), true
			}
			if count == 0 {
				return "(no matches)", false
			}
			text := strings.Join(out, "\n")
			if count > 500 {
				text += fmt.Sprintf("\n…[共 %d 个匹配，仅显示前 500 个]", count)
			}
			return text, false
		},
	},
	{
		Name: "grep",
		Description: "Search file contents with a regular expression (RE2 syntax). " +
			"Returns \"path:line: text\" matches, up to 200. Skips binary files, .git and node_modules.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Regular expression to search for."},
				"path":    map[string]any{"type": "string", "description": "File or directory to search, default the working directory."},
				"include": map[string]any{"type": "string", "description": "Optional glob for file names to search, e.g. *.go."},
			},
			"required": []string{"pattern"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
				Include string `json:"include"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if a.Pattern == "" {
				return "error: pattern is empty", true
			}
			re, err := regexp.Compile(a.Pattern)
			if err != nil {
				return "error: 无效正则: " + err.Error(), true
			}
			root := resolve(cwd, a.Path)
			var buf bytes.Buffer
			matches := 0
			skipped := 0
			stop := fmt.Errorf("stop")
			err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if matches >= 200 {
					return stop
				}
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
						return fs.SkipDir
					}
					return nil
				}
				if a.Include != "" {
					ok, _ := filepath.Match(a.Include, d.Name())
					if !ok {
						return nil
					}
				}
				info, ierr := d.Info()
				if ierr != nil || info.Size() > 2*1024*1024 {
					skipped++
					return nil
				}
				data, rerr := os.ReadFile(p) //nolint:gosec
				if rerr != nil {
					return nil
				}
				if bytes.IndexByte(data[:min(len(data), 512)], 0) >= 0 {
					return nil // binary
				}
				rel, _ := filepath.Rel(root, p)
				for i, line := range bytes.SplitAfter(data, []byte("\n")) {
					if len(line) == 0 {
						continue
					}
					if re.Match(line) {
						matches++
						text := strings.TrimRight(string(line), "\n")
						if len(text) > 400 {
							text = text[:400] + "…"
						}
						fmt.Fprintf(&buf, "%s:%d: %s\n", rel, i+1, text)
						if matches >= 200 {
							buf.WriteString("…[匹配数达到上限 200，已停止]\n")
							return stop
						}
					}
				}
				return nil
			})
			if err != nil && err != stop {
				return "error: " + err.Error(), true
			}
			if buf.Len() == 0 {
				if skipped > 0 {
					return "(no matches)", false
				}
				return "(no matches)", false
			}
			return buf.String(), false
		},
	},
	{
		Name: "edit_file",
		Description: "Edit a text file by replacing an exact string. old_string must appear " +
			"exactly once in the file unless replace_all is true.",
		Mutating: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string", "description": "File path, absolute or relative to the working directory."},
				"old_string":  map[string]any{"type": "string", "description": "Exact text to replace."},
				"new_string":  map[string]any{"type": "string", "description": "Replacement text."},
				"replace_all": map[string]any{"type": "boolean", "description": "Replace every occurrence, default false."},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if a.Path == "" || a.OldString == "" {
				return "error: path and old_string are required", true
			}
			p := resolve(cwd, a.Path)
			b, err := os.ReadFile(p) //nolint:gosec
			if err != nil {
				return "error: " + err.Error(), true
			}
			src := string(b)
			n := strings.Count(src, a.OldString)
			if n == 0 {
				return "error: old_string 在文件中不存在（先 read_file 确认原文）", true
			}
			if n > 1 && !a.ReplaceAll {
				return fmt.Sprintf("error: old_string 出现了 %d 次；扩大上下文使其唯一，或设 replace_all=true", n), true
			}
			dst := strings.ReplaceAll(src, a.OldString, a.NewString)
			if err := os.WriteFile(p, []byte(dst), 0o644); err != nil {
				return "error: " + err.Error(), true
			}
			return fmt.Sprintf("已编辑 %s（替换 %d 处）", p, n), false
		},
	},
	{
		Name: "web_fetch",
		Description: "Fetch a web page over HTTP(S) and return readable text (HTML tags stripped), up to 12000 characters. " +
			"Use for documentation, APIs and pages the user mentions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "Absolute http(s) URL to fetch."},
			},
			"required": []string{"url"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				URL string `json:"url"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			if !strings.HasPrefix(a.URL, "http://") && !strings.HasPrefix(a.URL, "https://") {
				return "error: 需要绝对 http(s) URL", true
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
			if err != nil {
				return "error: " + err.Error(), true
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) DeepSeekHarness/0.1")
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "error: " + err.Error(), true
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
			if err != nil {
				return "error: " + err.Error(), true
			}
			text := stripHTML(string(body))
			if len(text) > 12000 {
				text = text[:12000] + "…[已截断]"
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Sprintf("(HTTP %d，无文本内容)", resp.StatusCode), false
			}
			return fmt.Sprintf("[HTTP %d]\n%s", resp.StatusCode, text), false
		},
	},
	{
		Name: "web_search",
		Description: "Search the web (DuckDuckGo, no API key) and return result titles, URLs and snippets. " +
			"Follow up with web_fetch for the full page.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query."},
			},
			"required": []string{"query"},
		},
		Run: func(ctx context.Context, argsJSON, cwd string) (string, bool) {
			var a struct {
				Query string `json:"query"`
			}
			if err := args(ctx, argsJSON, &a); err != nil {
				return err.Error(), true
			}
			form := url.Values{"q": {a.Query}, "kl": {"cn-zh"}}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
			if err != nil {
				return "error: " + err.Error(), true
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) DeepSeekHarness/0.1")
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "error: " + err.Error(), true
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
			if err != nil {
				return "error: " + err.Error(), true
			}
			return parseDDG(string(body)), false
		},
	},
}

// stripHTML removes script/style blocks, tags and entities, and collapses
// whitespace — a readable-text approximation of a page.
func stripHTML(s string) string {
	reScript := regexp.MustCompile(`(?is)<(script|style|noscript|svg|head)[^>]*>.*?</\1>`)
	s = reScript.ReplaceAllString(s, " ")
	reTag := regexp.MustCompile(`(?s)<[^>]+>`)
	s = reTag.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&mdash;", "—").Replace(s)
	reSpace := regexp.MustCompile(`[ \t\r\f]+`)
	s = reSpace.ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// parseDDG scrapes DuckDuckGo's HTML results into "title\nURL\nsnippet".
func parseDDG(html string) string {
	reRes := regexp.MustCompile(`(?s)<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	reSnip := regexp.MustCompile(`(?s)<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</a>`)
	reClean := regexp.MustCompile(`<[^>]+>`)
	titles := reRes.FindAllStringSubmatch(html, -1)
	snips := reSnip.FindAllStringSubmatch(html, -1)
	if len(titles) == 0 {
		return "(无搜索结果，或被限流；请稍后重试或用 web_fetch 直接访问)"
	}
	var buf strings.Builder
	for i, t := range titles {
		if i >= 8 {
			break
		}
		title := strings.TrimSpace(reClean.ReplaceAllString(t[2], ""))
		link := ddgLink(t[1])
		buf.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, title, link))
		if i < len(snips) {
			sn := strings.TrimSpace(reClean.ReplaceAllString(snips[i][1], ""))
			if sn != "" {
				buf.WriteString("   " + sn + "\n")
			}
		}
		buf.WriteString("\n")
	}
	return strings.TrimSpace(buf.String())
}

// ddgLink resolves DuckDuckGo's redirect wrapper ("//duckduckgo.com/l/?uddg=…")
// down to the target URL, and unescapes HTML entities.
func ddgLink(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	if i := strings.Index(href, "uddg="); i >= 0 {
		if u, err := url.QueryUnescape(href[i+5:]); err == nil {
			if j := strings.Index(u, "&"); j >= 0 {
				u = u[:j]
			}
			return u
		}
	}
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"").Replace(href)
}

// All returns the tool set.
func All() []Tool { return all }

// ByName finds a tool by name.
func ByName(name string) *Tool {
	for i := range all {
		if all[i].Name == name {
			return &all[i]
		}
	}
	return nil
}

// Specs returns the tools' API schema list.
func Specs() []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(all))
	for _, t := range all {
		specs = append(specs, llm.ToolSpec{
			Type: "function",
			Function: llm.ToolDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return specs
}

// SpecsFor returns schemas for the named tools only (a preset's set).
// Unknown names are skipped; the run_code tool of PTC mode is provided by
// the agent package.
func SpecsFor(names []string) []llm.ToolSpec {
	var specs []llm.ToolSpec
	for _, n := range names {
		if t := ByName(n); t != nil {
			specs = append(specs, llm.ToolSpec{
				Type: "function",
				Function: llm.ToolDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}
	return specs
}

// matchPath matches rel against pattern where "**" spans directories.
func matchPath(pattern, rel string) bool {
	return matchSegs(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchSegs(p, s []string) bool {
	for len(p) > 0 {
		if p[0] == "**" {
			for i := 0; i <= len(s); i++ {
				if matchSegs(p[1:], s[i:]) {
					return true
				}
			}
			return false
		}
		if len(s) == 0 {
			return false
		}
		ok, _ := filepath.Match(p[0], s[0])
		if !ok {
			return false
		}
		p, s = p[1:], s[1:]
	}
	return len(s) == 0
}
