package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestParentIsInventoryOnlyAndSharedSizesAreNotPromised(t *testing.T) {
	r := findings.NewReport("/u")
	r.Findings = []findings.Finding{
		{ID: "parent", Path: "/u/Devices", Bytes: 280 << 30, Category: "simulators", Risk: findings.RiskAsk, LastUsed: "2026-03-11"},
		{ID: "a", Path: "/u/Devices/A", Bytes: 66 << 30, Category: "simulator-device", Risk: findings.RiskUnusedRuntime, LastUsed: "2026-11-14"},
		{ID: "b", Path: "/u/Devices/B", Bytes: 66 << 30, Category: "simulator-device", Risk: findings.RiskUnusedRuntime},
	}
	var out bytes.Buffer
	if err := Markdown(&out, r, Options{}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "| `parent` |") {
		t.Fatal("parent offered as a delete row", s)
	}
	if !strings.Contains(s, "Unallocated container space") || !strings.Contains(s, "Reclaim total: unknown") {
		t.Fatal("missing honest accounting", s)
	}
	if !strings.Contains(s, "2026-11-14") {
		t.Fatal("missing recent child date", s)
	}
}

func TestCatalogAdviceIsVisibleToAgentsInBothFormats(t *testing.T) {
	r := findings.NewReport("/u")
	r.CatalogAdvice = &findings.CatalogAdvice{CanUpdateOverlay: true, OverlayPath: "/u/.config/freedisk/catalog.yaml", Message: "Agents may improve this Mac's per-user catalog overlay.", Suggestions: []findings.CatalogSuggestion{{Path: "/u/cache", Scans: []string{"quick", "full"}, Reason: "Recurring cache", Command: "freedisk catalog add /u/cache --scans quick,full"}}}
	var markdown, js bytes.Buffer
	if err := Markdown(&markdown, r, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&js, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown.String(), "Agent catalog advice") || !strings.Contains(markdown.String(), "Recurring cache") || !strings.Contains(js.String(), `"can_update_overlay": true`) || !strings.Contains(js.String(), `"catalog_advice"`) {
		t.Fatal(markdown.String(), js.String())
	}
}
