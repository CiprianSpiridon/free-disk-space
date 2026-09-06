package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestJSONRequiredKeys(t *testing.T) {
	var buf bytes.Buffer
	r := findings.NewReport("/Users/x")
	if err := JSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"generated_at", "host", "volume", "findings"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %s", k)
		}
	}
}

func TestJSONOmitsEmptyReclaim(t *testing.T) {
	r := findings.NewReport("/Users/x")
	r.Findings = []findings.Finding{{
		ID: "a", Path: "/p", Bytes: 1, Category: "c", Risk: findings.RiskAsk,
	}}
	var buf bytes.Buffer
	if err := JSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"reclaim"`) {
		t.Fatalf("empty reclaim should be omitted: %s", buf.String())
	}
}

func TestJSONInvalidUTF8Path(t *testing.T) {
	r := findings.NewReport("/Users/x")
	r.Findings = []findings.Finding{{
		ID: "a", Path: "foo\x80bar", Bytes: 1, Category: "c", Risk: findings.RiskAsk,
	}}
	var buf bytes.Buffer
	if err := JSON(&buf, r); err != nil {
		t.Fatal(err)
	}
}
