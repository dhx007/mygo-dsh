package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skills are filesystem instruction packs (SKILL.md): frontmatter with
// name/description plus a markdown body the model loads on demand. This
// mirrors the original's dsh-skill / skill-filesystem packages. Two roots
// are scanned; a workspace skill shadows a global one with the same id.

const skillBodyLimit = 64 * 1024

type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Source      string `json:"source"` // 全局 | 工作区
	Path        string `json:"path"`
}

func skillRoots(dataDir, workspace string) (global, ws string) {
	if dataDir != "" {
		global = filepath.Join(dataDir, "skills")
	}
	if workspace != "" {
		ws = filepath.Join(workspace, ".harness", "skills")
	}
	return global, ws
}

// discoverSkills lists skills from both roots; workspace entries win over
// global ones with the same id. Missing or unreadable roots degrade to an
// empty list.
func discoverSkills(dataDir, workspace string) []Skill {
	global, ws := skillRoots(dataDir, workspace)
	out := map[string]Skill{}
	for _, root := range []struct{ dir, source string }{
		{global, "全局"},
		{ws, "工作区"},
	} {
		if root.dir == "" {
			continue
		}
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			s, err := parseSkillMD(filepath.Join(root.dir, e.Name()), root.source)
			if err != nil {
				continue
			}
			out[s.ID] = s
		}
	}
	// Stable order: workspace first, then global, each alphabetical.
	var skills []Skill
	for _, src := range []string{"工作区", "全局"} {
		var ids []string
		for id, s := range out {
			if s.Source == src {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			skills = append(skills, out[id])
		}
	}
	return skills
}

// parseSkillMD reads one SKILL.md: an optional `---` frontmatter with
// name/description (key: value per line, no YAML dependency), then the
// body. name falls back to the directory name.
func parseSkillMD(dir, source string) (Skill, error) {
	p := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(p) //nolint:gosec
	if err != nil {
		return Skill{}, err
	}
	text := string(b)
	if len(text) > skillBodyLimit {
		text = text[:skillBodyLimit] + "\n…[技能正文已截断]"
	}
	name, desc := "", ""
	body := strings.TrimSpace(text)
	if strings.HasPrefix(text, "---") {
		if end := strings.Index(text[3:], "\n---"); end >= 0 {
			fm := text[3 : 3+end]
			body = strings.TrimSpace(text[3+end+4:])
			for _, line := range strings.Split(fm, "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				switch strings.TrimSpace(k) {
				case "name":
					name = strings.TrimSpace(v)
				case "description":
					desc = strings.TrimSpace(v)
				}
			}
		}
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	return Skill{
		ID:          name,
		Name:        name,
		Description: desc,
		Body:        body,
		Source:      source,
		Path:        p,
	}, nil
}
