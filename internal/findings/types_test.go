package findings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalMinimalReportHasRequiredKeys(t *testing.T) {
	r := NewReport("/Users/test")
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"generated_at", "host", "volume", "findings"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing key %s in %s", k, b)
		}
	}
}

func TestParseRiskRejectsUnknown(t *testing.T) {
	if _, err := ParseRisk("not-a-risk"); err == nil {
		t.Fatal("expected error for not-a-risk")
	}
	r := Report{Findings: []Finding{{ID: "x", Path: "/p", Risk: "not-a-risk"}}}
	if err := r.ValidateRisks(); err == nil {
		t.Fatal("expected ValidateRisks to fail")
	}
}

func TestIDSlugDistinguishesSimilarPaths(t *testing.T) {
	a := IDSlug("node", "/Users/x/src/my-app/node_modules")
	b := IDSlug("node", "/Users/x/src/my/app/node_modules")
	if a == b {
		t.Fatalf("collision %s", a)
	}
	c := IDSlug("tmp", "/private/tmp/foo bar")
	d := IDSlug("tmp", "/private/tmp/foo-bar")
	if c == d {
		t.Fatalf("collision %s", c)
	}
}

func TestUniquifyIDs(t *testing.T) {
	r := Report{Findings: []Finding{
		{ID: "same", Path: "/a"},
		{ID: "same", Path: "/b"},
		{ID: "other", Path: "/c"},
	}}
	UniquifyIDs(&r)
	if r.Findings[0].ID != "same" || r.Findings[1].ID != "same-2" || r.Findings[2].ID != "other" {
		t.Fatalf("%+v", r.Findings)
	}
}

func TestMarshalHasNoDebugFields(t *testing.T) {
	r := NewReport("/Users/test")
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"debug", "TODO", "internal_"} {
		if strings.Contains(s, bad) {
			t.Fatalf("unexpected field fragment %q in %s", bad, s)
		}
	}
}
