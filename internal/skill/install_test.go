package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownHasFrontmatter(t *testing.T) {
	s := string(Markdown())
	if !strings.Contains(s, "name: freedisk") {
		t.Fatal(s[:min(200, len(s))])
	}
	if !strings.Contains(s, "scan never deletes") && !strings.Contains(strings.ToLower(s), "scan never deletes") {
		t.Fatal("missing never deletes")
	}
}

func TestInstallOnlyAvailableCLIs(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	targets := []Target{
		{Name: "claude", HomeRel: ".claude", SkillRel: ".claude/skills"},
		{Name: "codex", HomeRel: ".codex", SkillRel: ".codex/skills"},
		{Name: "missing", HomeRel: ".nope", SkillRel: ".nope/skills"},
	}
	res := Install(home, targets)
	if len(res) != 3 {
		t.Fatalf("%+v", res)
	}
	if res[2].Skipped != true || res[0].Error != "" || res[1].Error != "" {
		t.Fatalf("%+v", res)
	}
	b, err := os.ReadFile(filepath.Join(home, ".claude", "skills", "freedisk", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "name: freedisk") {
		t.Fatal(string(b)[:80])
	}
	if _, err := os.Stat(filepath.Join(home, ".nope")); !os.IsNotExist(err) {
		t.Fatal("must not create missing cli home")
	}
}

func TestListSkipsMissing(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".grok"), 0o755)
	res := List(home, []Target{{Name: "grok", HomeRel: ".grok", SkillRel: ".grok/skills"}, {Name: "x", HomeRel: ".x", SkillRel: ".x/skills"}})
	if !res[0].Skipped && res[0].Path == "" {
		t.Fatal(res[0])
	}
	if !res[1].Skipped {
		t.Fatal(res[1])
	}
}
