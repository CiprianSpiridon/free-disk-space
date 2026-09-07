package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-scan.json")
	r := findings.NewReport("/Users/x")
	r.Findings = []findings.Finding{{ID: "a", Path: "/p", Bytes: 3, Category: "c", Risk: findings.RiskAsk}}
	if err := WriteLastScan(p, r); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLastScan(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 1 || got.Findings[0].ID != "a" {
		t.Fatalf("%+v", got)
	}
}

func TestPersistStaleLastScan(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-scan.json")
	r := findings.NewReport("/Users/x")
	r.GeneratedAt = "2000-01-01T00:00:00Z"
	r.Findings = []findings.Finding{{ID: "a", Path: "/p", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	if err := WriteLastScan(p, r); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastScan(p); err == nil {
		t.Fatal("expected stale")
	}
}

func TestPersistRejectsInvalidRisk(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-scan.json")
	if err := os.WriteFile(p, []byte(`{"generated_at":"t","host":{"os":"macos","home":"/u"},"volume":{},"findings":[{"id":"x","path":"/p","bytes":1,"category":"c","risk":"nope"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastScan(p); err == nil {
		t.Fatal("expected invalid risk")
	}
}

func TestPersistTruncated(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-scan.json")
	if err := os.WriteFile(p, []byte(`{"generated_at":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastScan(p); err == nil {
		t.Fatal("expected error")
	}
}

func TestPersistXDGCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if LastScanPath() != filepath.Join(dir, "freedisk", "last-scan.json") {
		t.Fatal(LastScanPath())
	}
}
