package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestCatalogAdviceSuggestsQuickAndDevWithoutReaddingKnownPaths(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches", "new cache")
	build := filepath.Join(home, "project", "target")
	known := filepath.Join(home, "known")
	r := findings.NewReport(home)
	r.Findings = []findings.Finding{
		{Path: cache, Bytes: 3 << 30, Category: "caches", Risk: findings.RiskAsk, Why: "depth-1 of " + filepath.Dir(cache)},
		{Path: build, Bytes: 2 << 30, Category: "rust-target", Risk: findings.RiskRebuildable},
		{Path: known, Bytes: 4 << 30, Category: "home", Risk: findings.RiskAsk, Why: "home depth-1"},
		{Path: "/private/tmp/one-off", Bytes: 5 << 30, Category: "tmp", Risk: findings.RiskAsk},
	}
	ctx := &Context{Home: home, Report: &r, Catalog: &catalog.Catalog{Home: []catalog.Entry{{Path: known}}}, OverlayPath: filepath.Join(home, "custom", "catalog.yaml")}
	a := catalogAdvice(ctx)
	if !a.CanUpdateOverlay || !strings.Contains(a.Message, "per-user catalog overlay") {
		t.Fatal(a)
	}
	if len(a.Suggestions) != 2 || a.Suggestions[0].Scans[0] != "quick" || a.Suggestions[1].Scans[0] != "dev" {
		t.Fatal(a.Suggestions)
	}
	if !strings.Contains(a.Suggestions[0].Command, "--config "+filepath.Join(home, "custom")) || !strings.Contains(a.Suggestions[0].Command, "'"+cache+"'") {
		t.Fatal(a.Suggestions[0])
	}
}
