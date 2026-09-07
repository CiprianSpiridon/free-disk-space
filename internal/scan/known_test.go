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
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
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
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
	for _, f := range rep.Findings {
		if strings.Contains(f.Path, ".npm") || strings.Contains(f.Path, "onlydev") {
			t.Fatalf("unexpected %s", f.Path)
		}
	}
	dir := t.TempDir()
	cat2 := &catalog.Catalog{Home: []catalog.Entry{{Path: filepath.Join(dir, "nope-*"), Glob: true, Category: "x", Risk: "ask", Scans: []string{"quick", "full"}}}}
	rep2 := findings.NewReport(home)
	Known(&Context{Catalog: cat2, Mode: "quick", Home: home, Report: &rep2})
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
	Known(&Context{Catalog: cat, Mode: "quick", Home: dir, Report: &rep})
	if len(rep.Findings) != 1 {
		t.Fatalf("dedupe want 1 got %d", len(rep.Findings))
	}
}

func TestKnownEmitsNestedHotspotAndParent(t *testing.T) {
	home := t.TempDir()
	caches := filepath.Join(home, "Library", "Caches")
	store := filepath.Join(caches, "kache", "store")
	google := filepath.Join(caches, "Google")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(google, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "blob"), make([]byte, 32<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(google, "g"), make([]byte, 16<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		Library: []catalog.Entry{{Path: caches, Category: "caches", Risk: "ask", Drill: true, Scans: []string{"quick", "full"}}},
		AgentCLIs: []catalog.Entry{{
			Path: store, Category: "kache-store", Risk: "ask", Scans: []string{"quick", "full"},
		}},
		Thresholds: catalog.Thresholds{ReportBytes: 1},
	}
	rep := findings.NewReport(home)
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
	var sawStore, sawCaches, sawGoogle bool
	var storeB, cachesB int64
	for _, f := range rep.Findings {
		switch f.Path {
		case store:
			sawStore, storeB = true, f.Bytes
		case caches:
			sawCaches, cachesB = true, f.Bytes
		case google:
			sawGoogle = true
		}
	}
	if !sawStore {
		t.Fatalf("named hotspot missing: %+v", rep.Findings)
	}
	if !sawCaches {
		t.Fatalf("parent missing: %+v", rep.Findings)
	}
	if !sawGoogle {
		t.Fatalf("depth-1 sibling missing: %+v", rep.Findings)
	}
	if cachesB < storeB {
		t.Fatalf("parent %d < child %d", cachesB, storeB)
	}
}

func TestKnownKeepDrillIsSizedNotInode(t *testing.T) {
	home := t.TempDir()
	wt := filepath.Join(home, ".grok", "worktrees")
	proj := filepath.Join(wt, "cc-vs-oc")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "blob"), make([]byte, 64<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		AgentCLIs: []catalog.Entry{{
			Path: wt, Category: "grok-worktrees", Risk: "keep", Drill: true, Scans: []string{"quick", "full"},
		}},
		Thresholds: catalog.Thresholds{ReportBytes: 1},
	}
	rep := findings.NewReport(home)
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
	var parent, child bool
	for _, f := range rep.Findings {
		if f.Path == wt && f.Bytes > 4096 {
			parent = true
		}
		if f.Path == proj && f.Bytes > 0 {
			child = true
		}
	}
	if !parent {
		t.Fatalf("worktrees parent should be sized, not inode-only: %+v", rep.Findings)
	}
	if !child {
		t.Fatalf("worktrees child should be named: %+v", rep.Findings)
	}
}

func TestHomeDepth1DiscoversWorkCip(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "work_cip")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "blob"), make([]byte, 64<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Thresholds: catalog.Thresholds{ReportBytes: 1}}
	rep := findings.NewReport(home)
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
	found := false
	for _, f := range rep.Findings {
		if f.Path == work {
			found = true
			if f.Risk != findings.RiskKeep {
				t.Fatalf("work tree risk %s", f.Risk)
			}
		}
	}
	if !found {
		t.Fatalf("work_cip missing: %+v", rep.Findings)
	}
}

func TestWorkGlobMatchesWorkCip(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "work_cip")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "blob"), make([]byte, 64<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		WorkRoots: []catalog.Entry{{
			Path: filepath.Join(home, "work*"), Glob: true, Category: "work", Risk: "keep",
			WalkArtifacts: true, Scans: []string{"quick", "full"},
		}},
		Thresholds: catalog.Thresholds{ReportBytes: 1 << 30},
	}
	rep := findings.NewReport(home)
	Known(&Context{Catalog: cat, Mode: "quick", Home: home, Report: &rep})
	found := false
	for _, f := range rep.Findings {
		if f.Path == work && f.Category == "work" {
			found = true
		}
	}
	if !found {
		t.Fatalf("glob ~/work* should size work_cip: %+v", rep.Findings)
	}
}
