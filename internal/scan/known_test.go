package scan

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestKnownSkipMissingAndEmitNPM(t *testing.T) {
	home := t.TempDir()
	npm := filepath.Join(home, ".npm", "_cacache")
	if err := os.MkdirAll(npm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npm, "x"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		Node: []catalog.Entry{
			{Path: "~/.npm/_cacache", Category: "npm-cache", Risk: "safe-cache", Scans: []string{"quick", "full"}},
			{Path: "~/.rbenv", Category: "rbenv", Risk: "unused-runtime", Scans: []string{"quick", "full"}},
		},
	}
	rep := findings.NewReport(home)
	Known(cat, "quick", home, &rep)
	if len(rep.Findings) != 1 {
		t.Fatalf("findings=%d %+v", len(rep.Findings), rep.Findings)
	}
	if !strings.Contains(rep.Findings[0].Path, "_cacache") {
		t.Fatal(rep.Findings[0].Path)
	}
}

func TestKnownDisabledAndGlobAndDevTag(t *testing.T) {
	home := t.TempDir()
	cat := catalog.Merge(&catalog.Catalog{
		Node: []catalog.Entry{{Path: "~/.npm", Category: "npm-cache", Risk: "safe-cache"}},
	}, &catalog.Overlay{
		Disable: []string{"~/.npm"},
		Add:     []catalog.Entry{{Path: "~/onlydev", Scans: []string{"dev"}, Category: "user", Risk: "ask"}},
	}, home)
	_ = os.MkdirAll(filepath.Join(home, "onlydev"), 0o755)
	rep := findings.NewReport(home)
	Known(cat, "quick", home, &rep)
	for _, f := range rep.Findings {
		if strings.Contains(f.Path, ".npm") || strings.Contains(f.Path, "onlydev") {
			t.Fatalf("unexpected %s", f.Path)
		}
	}
	dir := t.TempDir()
	cat2 := &catalog.Catalog{Home: []catalog.Entry{{Path: filepath.Join(dir, "nope-*"), Glob: true, Category: "x", Risk: "ask", Scans: []string{"quick", "full"}}}}
	rep2 := findings.NewReport(home)
	Known(cat2, "quick", home, &rep2)
	if len(rep2.Findings) != 0 {
		t.Fatal("glob zero matches should skip")
	}
}

func TestKnownRealpathDedupeNoExec(t *testing.T) {
	src, err := os.ReadFile("known.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "known.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range f.Imports {
		if im.Path.Value == `"os/exec"` {
			t.Fatal("known.go must not import os/exec")
		}
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	_ = os.Mkdir(real, 0o755)
	link := filepath.Join(dir, "link")
	_ = os.Symlink(real, link)
	cat := &catalog.Catalog{Tmp: []catalog.Entry{
		{Path: real, Category: "tmp", Risk: "keep", Scans: []string{"quick", "full"}},
		{Path: link, Category: "tmp", Risk: "keep", Scans: []string{"quick", "full"}},
	}}
	rep := findings.NewReport(dir)
	Known(cat, "quick", dir, &rep)
	if len(rep.Findings) != 1 {
		t.Fatalf("dedupe want 1 got %d", len(rep.Findings))
	}
}
