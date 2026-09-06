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
