package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func TestWhyOrdersAndOmitsKeep(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{
		{ID: "small", Path: "/s", Bytes: 1, Category: "c", Risk: findings.RiskAsk},
		{ID: "big", Path: "/b", Bytes: 99, Category: "c", Risk: findings.RiskSafeCache},
		{ID: "keep", Path: "/k", Bytes: 1000, Category: "c", Risk: findings.RiskKeep},
	}
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: false}
	if err := runWhy(g, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var out findings.Report
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err, buf.String())
	}
	if len(out.Findings) != 2 || out.Findings[0].ID != "big" {
		t.Fatalf("%+v", out.Findings)
	}
}

func TestWhyLimit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{
		{ID: "a", Path: "/a", Bytes: 3, Category: "c", Risk: findings.RiskAsk},
		{ID: "b", Path: "/b", Bytes: 2, Category: "c", Risk: findings.RiskAsk},
		{ID: "c", Path: "/c", Bytes: 1, Category: "c", Risk: findings.RiskAsk},
	}
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: false}
	if err := runWhy(g, []string{"--json", "--limit", "1"}); err != nil {
		t.Fatal(err)
	}
	var out findings.Report
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 || out.Findings[0].ID != "a" {
		t.Fatalf("%+v", out.Findings)
	}
}

func TestWhyMissingLastScan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf}
	if err := runWhy(g, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "last-scan") {
		t.Fatal(buf.String())
	}
}

func TestWhyNoRemove(t *testing.T) {
	b, _ := os.ReadFile("why.go")
	if strings.Contains(string(b), "os.Remove") || strings.Contains(string(b), "reclaim.Apply") {
		t.Fatal("why must not delete")
	}
}
