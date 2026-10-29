package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Workspace instruction files (AGENTS.md, falling back to CLAUDE.md) are
// loaded into the system prompt at agent build time — the original's
// agent-instructions plugin ("Workspace context loader for AGENTS.md/
// CLAUDE.md instruction files").

const instructionsLimit = 32 * 1024

// loadWorkspaceInstructions returns the instruction section for the
// workspace, or "" when no instruction file exists. The first non-empty
// file wins; oversized content is truncated with a note.
func loadWorkspaceInstructions(workspace string) string {
	if workspace == "" {
		return ""
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		b, err := os.ReadFile(filepath.Join(workspace, name)) //nolint:gosec
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(b))
		if text == "" {
			continue
		}
		if len(text) > instructionsLimit {
			text = text[:instructionsLimit] + "\n…[工作区指令超过 32KB，已截断]"
		}
		return "[工作区指令：" + name + "] 请遵循以下来自工作区的项目指令：\n\n" + text
	}
	return ""
}
