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
	for _, f := range rep.Findings {
		if f.Path == big {
			if f.Reclaim == nil || f.Reclaim.Cmd == "" {
				t.Fatal("drill child needs a reclaim command")
			}
			if f.LastUsed == "" {
				t.Fatal("drill child needs last used")
			}
		}
	}
}

func TestDrillSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	_ = os.Mkdir(nm, 0o755)
	_ = os.WriteFile(filepath.Join(nm, "b"), make([]byte, 8000), 0o644)
	rep := findings.NewReport(root)
	rep.Findings = []findings.Finding{
		{ID: "nm", Path: nm, Bytes: 1 << 20, Category: "node_modules", Risk: findings.RiskRebuildable},
	}
	ctx := &Context{
		Catalog: &catalog.Catalog{Thresholds: catalog.Thresholds{DrillBytes: 100}},
		Report:  &rep,
		Home:    root,
	}
	if err := runDrill(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("drilled artifact: %+v", rep.Findings)
	}
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

func TestDrillTimeoutDoesNotFail(t *testing.T) {
	oldB, oldC := drillBudget, drillChildTimeout
	drillBudget = 0
	drillChildTimeout = 0
	defer func() {
		drillBudget, drillChildTimeout = oldB, oldC
	}()
	root := t.TempDir()
	big := filepath.Join(root, "big")
	_ = os.Mkdir(big, 0o755)
	_ = os.WriteFile(filepath.Join(big, "b"), make([]byte, 8000), 0o644)
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
		t.Fatalf("timeout must not fail the scan: %v", err)
	}
}

func TestDrillSkipsSafeCache(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "blobdir")
	_ = os.Mkdir(child, 0o755)
	_ = os.WriteFile(filepath.Join(child, "b"), make([]byte, 8000), 0o644)
	rep := findings.NewReport(root)
	rep.Findings = []findings.Finding{
		{ID: "npm", Path: root, Bytes: 1 << 20, Category: "npm-cache", Risk: findings.RiskSafeCache},
	}
	ctx := &Context{
		Catalog: &catalog.Catalog{Thresholds: catalog.Thresholds{DrillBytes: 100}},
		Report:  &rep,
		Home:    root,
	}
	if err := runDrill(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("drilled safe-cache: %+v", rep.Findings)
	}
}
