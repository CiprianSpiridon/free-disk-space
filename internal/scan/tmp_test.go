package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/policy"
)

func TestTmpChildIdleNotRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "old")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 6<<20)
	if err := os.WriteFile(filepath.Join(child, "blob"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(child, old, old)
	cat := &catalog.Catalog{
		Tmp:        []catalog.Entry{{Path: root, Category: "tmp", Risk: "keep", Scans: []string{"quick", "dev", "full"}}},
		Thresholds: catalog.Thresholds{TmpIdleDays: 7, ArtifactListMinBytes: 5 << 20},
	}
	rep := findings.NewReport(root)
	ctx := &Context{Mode: "quick", Catalog: cat, Home: root, Report: &rep}
	if err := runTmp(ctx); err != nil {
		t.Fatal(err)
	}
	var sawChild, sawRoot bool
	for _, f := range rep.Findings {
		if f.Path == root && f.Risk != findings.RiskKeep {
			sawRoot = true
		}
		if f.Path == child && f.Risk == findings.RiskAsk {
			sawChild = true
		}
	}
	if !sawChild {
		t.Fatalf("missing child %+v", rep.Findings)
	}
	if sawRoot {
		t.Fatal("root should not be reclaimable from tmp phase")
	}
}

func TestTmpKensiStyleChildrenAreCaught(t *testing.T) {
	root := t.TempDir()
	// /private/tmp/kensi* leftovers: large named children, not the tmp root.
	names := []string{"kensi-app", "kensi-build", "kensi-cache"}
	old := time.Now().Add(-30 * 24 * time.Hour)
	for _, n := range names {
		p := filepath.Join(root, n)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "blob"), make([]byte, 6<<20), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, old, old)
	}
	tiny := filepath.Join(root, "tiny")
	_ = os.Mkdir(tiny, 0o755)
	cat := &catalog.Catalog{
		Tmp:        []catalog.Entry{{Path: root, Category: "tmp", Risk: "keep", Scans: []string{"quick", "dev", "full"}}},
		Thresholds: catalog.Thresholds{TmpIdleDays: 7, ArtifactListMinBytes: 5 << 20},
	}
	rep := findings.NewReport(root)
	ctx := &Context{Mode: "quick", Catalog: cat, Home: root, Report: &rep}
	if err := runTmp(ctx); err != nil {
		t.Fatal(err)
	}
	found := map[string]findings.Finding{}
	for _, f := range rep.Findings {
		if f.Path == root {
			t.Fatal("tmp root must not be a reclaim finding")
		}
		found[filepath.Base(f.Path)] = f
	}
	for _, n := range names {
		f, ok := found[n]
		if !ok {
			t.Fatalf("missed %s in %+v", n, rep.Findings)
		}
		if f.Risk != findings.RiskAsk {
			t.Fatalf("%s risk %s", n, f.Risk)
		}
		if f.Bytes < 5<<20 {
			t.Fatalf("%s bytes %d", n, f.Bytes)
		}
		if f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, n) {
			t.Fatalf("%s reclaim %+v", n, f.Reclaim)
		}
		if !policy.CanDelete(f.Path) {
			t.Fatalf("delete must allow tmp child %s", f.Path)
		}
	}
	if _, ok := found["tiny"]; ok {
		t.Fatal("sub-5MiB child should be skipped")
	}
}

func TestTmpQuickIncludesPhase(t *testing.T) {
	var names []string
	for _, p := range PhasesForTest() {
		names = append(names, p.Name)
		if p.Name == "tmp" && !p.Quick {
			t.Fatal("tmp must be Quick")
		}
	}
	found := false
	for _, n := range names {
		if n == "tmp" {
			found = true
		}
	}
	if !found {
		t.Fatal("tmp phase not registered")
	}
}

func TestTmpRealpathDedupe(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "t")
	_ = os.Mkdir(real, 0o755)
	link := filepath.Join(dir, "l")
	_ = os.Symlink(real, link)
	_ = os.WriteFile(filepath.Join(real, "blob"), make([]byte, 6<<20), 0o644)
	cat := &catalog.Catalog{
		Tmp: []catalog.Entry{
			{Path: real, Category: "tmp", Risk: "keep"},
			{Path: link, Category: "tmp", Risk: "keep"},
		},
		Thresholds: catalog.Thresholds{ArtifactListMinBytes: 5 << 20},
	}
	rep := findings.NewReport(dir)
	ctx := &Context{Catalog: cat, Home: dir, Report: &rep}
	_ = runTmp(ctx)
	n := 0
	for _, f := range rep.Findings {
		if f.Category == "tmp" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("dup children %d %+v", n, rep.Findings)
	}
}
