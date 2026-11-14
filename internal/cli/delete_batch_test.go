package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func TestDeleteContinuesAndMeasuresContainerSpace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	p := filepath.Join(dir, "good")
	os.WriteFile(p, []byte("x"), 0644)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{{ID: "good", Path: p, Bytes: 100, Risk: findings.RiskSafeCache, Category: "test"}}
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	old := measureDeleteVolume
	defer func() { measureDeleteVolume = old }()
	n := 0
	measureDeleteVolume = func() (findings.Volume, error) { n++; return findings.Volume{FreeBytes: int64(n * 1000)}, nil }
	out := &bytes.Buffer{}
	g := &Global{Stdout: out, Stderr: &bytes.Buffer{}, IsTTY: false}
	if err := runDelete(g, []string{"missing", "good", "--yes"}); err == nil {
		t.Fatal("missing id must still report failure")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("later id was not attempted")
	}
	for _, s := range []string{"Unallocated container space before: 1000 bytes", "after: 2000 bytes", "measured change: +1000 bytes", "completed row sizes: 100 bytes"} {
		if !strings.Contains(out.String(), s) {
			t.Fatal("missing comparison", s, out.String())
		}
	}
}
