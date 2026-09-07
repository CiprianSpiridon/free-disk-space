package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPrint(t *testing.T) {
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: true}
	if err := runSkill(g, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "name: freedisk") {
		t.Fatal(buf.String()[:min(120, buf.Len())])
	}
}

func TestSkillInstallAndList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: false}
	if err := runSkill(g, []string{"install", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "claude") {
		t.Fatal(buf.String())
	}
	p := filepath.Join(home, ".claude", "skills", "freedisk", "SKILL.md")
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}
