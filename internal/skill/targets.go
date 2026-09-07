package skill

import (
	"os"
	"path/filepath"
)

// Target is one agent CLI skill location.
type Target struct {
	Name     string
	HomeRel  string // directory that must exist for the tool to count as installed
	SkillRel string // skills dir relative to user home
}

// DefaultTargets are user-level skill dirs for CLIs this machine may have.
var DefaultTargets = []Target{
	{Name: "claude", HomeRel: ".claude", SkillRel: ".claude/skills"},
	{Name: "codex", HomeRel: ".codex", SkillRel: ".codex/skills"},
	{Name: "cursor", HomeRel: ".cursor", SkillRel: ".cursor/skills"},
	{Name: "grok", HomeRel: ".grok", SkillRel: ".grok/skills"},
	{Name: "agents", HomeRel: ".agents", SkillRel: ".agents/skills"},
	{Name: "opencode", HomeRel: ".config/opencode", SkillRel: ".config/opencode/skills"},
	{Name: "kiro", HomeRel: ".kiro", SkillRel: ".kiro/skills"},
	{Name: "gemini", HomeRel: ".gemini", SkillRel: ".gemini/skills"},
	{Name: "factory", HomeRel: ".factory", SkillRel: ".factory/skills"},
	{Name: "goose", HomeRel: ".config/goose", SkillRel: ".config/goose/skills"},
	{Name: "continue", HomeRel: ".continue", SkillRel: ".continue/skills"},
	{Name: "windsurf", HomeRel: ".codeium/windsurf", SkillRel: ".codeium/windsurf/skills"},
}

// Available reports whether the tool's home directory exists.
func (t Target) Available(home string) bool {
	if home == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(home, t.HomeRel))
	return err == nil && st.IsDir()
}

// SkillDir is ~/.…/skills/freedisk
func (t Target) SkillDir(home string) string {
	return filepath.Join(home, t.SkillRel, "freedisk")
}

func (t Target) SkillFile(home string) string {
	return filepath.Join(t.SkillDir(home), "SKILL.md")
}
