package findings

import "testing"

func TestPrivateReclaimTotalExcludesParentsAndClonedSiblings(t *testing.T) {
	parent, a, b, clone := int64(1000), int64(30), int64(50), int64(0)
	r := NewReport("/u")
	r.Findings = []Finding{
		{ID: "parent", Path: "/u/cache", Bytes: 1000, ReclaimBytes: &parent, Risk: RiskAsk, LastUsed: "2026-03-11"},
		{ID: "a", Path: "/u/cache/a", Bytes: 100, ReclaimBytes: &a, Risk: RiskSafeCache, LastUsed: "2026-11-14"},
		{ID: "b", Path: "/u/cache/b", Bytes: 100, ReclaimBytes: &b, Risk: RiskSafeCache},
		{ID: "clone1", Path: "/u/clone1", Bytes: 1000, ReclaimBytes: &clone, Risk: RiskAsk},
		{ID: "clone2", Path: "/u/clone2", Bytes: 1000, ReclaimBytes: &clone, Risk: RiskAsk},
	}
	r = Normalize(r)
	if r.ReclaimTotal.Bytes == nil || *r.ReclaimTotal.Bytes != 80 {
		t.Fatalf("private total: %+v", r.ReclaimTotal)
	}
	if !r.Findings[0].SummaryOnly || r.Findings[0].LastUsed != "2026-11-14" {
		t.Fatal("parent date/eligibility", r.Findings[0])
	}
}
