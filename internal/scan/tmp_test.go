package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
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
