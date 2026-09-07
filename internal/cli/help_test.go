package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHelpGuide(t *testing.T) {
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, Stderr: buf, IsTTY: true}
	if err := runHelp(g, nil); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, w := range []string{"catalog add", "unassign", "--glob", "scans disable", "scans add", "--mode", "--json", "delete", "never", "node_modules", "/tmp", "/private/tmp", "kensi", "git-tracked"} {
		if !strings.Contains(s, w) {
			t.Fatalf("missing %q in help", w)
		}
	}
	if !strings.Contains(s, "overlay") && !strings.Contains(s, "Starting point") {
		t.Fatal("starting/overlay")
	}
}

func TestHelpScanCatalog(t *testing.T) {
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf}
	if err := runHelp(g, []string{"scan"}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "quick") || !strings.Contains(s, "dev") || !strings.Contains(s, "full") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "never deletes") && !strings.Contains(s, "never delete") {
		t.Fatal(s)
	}
}

func TestHelpUnknown(t *testing.T) {
	g := &Global{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	err := runHelp(g, []string{"definitely-not-a-command"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("%v", err)
	}
}
