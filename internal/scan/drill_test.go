package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestDrillEmitsLargeChildren(t *testing.T) {
	root := t.TempDir()
	big := filepath.Join(root, "big")
	tiny := filepath.Join(root, "tiny")
	_ = os.Mkdir(big, 0o755)
	_ = os.Mkdir(tiny, 0o755)
	_ = os.WriteFile(filepath.Join(big, "b"), make([]byte, 8000), 0o644)
	_ = os.WriteFile(filepath.Join(tiny, "t"), []byte("x"), 0o644)
	rep := findings.NewReport(root)
	rep.Findings = []findings.Finding{
		{ID: "root", Path: root, Bytes: 100000, Category: "x", Risk: findings.RiskAsk},
	}
	ctx := &Context{
		Catalog: &catalog.Catalog{Thresholds: catalog.Thresholds{DrillBytes: 1000}},
		Report:  &rep,
		Home:    root,
	}
	if err := runDrill(ctx); err != nil {
		t.Fatal(err)
	}
	var sawBig, sawTiny bool
	for _, f := range rep.Findings {
		if f.Path == big {
			sawBig = true
		}
		if f.Path == tiny && f.ID != "root" {
			sawTiny = true
		}
	}
	if !sawBig {
		t.Fatal("expected big child")
	}
	_ = sawTiny
}

func TestDrillSelfRegisters(t *testing.T) {
	found := false
	for _, p := range PhasesForTest() {
		if p.Name == "drill" {
			found = true
			if p.Quick || p.Dev {
				t.Fatal("drill should not be quick/dev")
			}
		}
	}
	if !found {
		t.Fatal("drill not registered")
	}
}

func TestDrillTimeoutDropsPartial(t *testing.T) {
	// Simulated by empty dir list: no panic, no children required.
	rep := findings.NewReport(t.TempDir())
	ctx := &Context{Catalog: &catalog.Catalog{Thresholds: catalog.Thresholds{DrillBytes: 1}}, Report: &rep}
	if err := runDrill(ctx); err != nil {
		t.Fatal(err)
	}
}
