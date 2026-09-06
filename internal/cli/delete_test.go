package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func TestDeleteFlags(t *testing.T) {
	g := &Global{Stdout: &bytes.Buffer{}, IsTTY: false}
	if err := runDelete(g, nil); !errors.Is(err, ErrUsage) {
		t.Fatal("empty")
	}
	if err := runDelete(g, []string{"--all"}); !errors.Is(err, ErrUsage) {
		t.Fatal("--all")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{{ID: "f1", Path: p, Bytes: 1, Category: "c", Risk: findings.RiskSafeCache}}
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	if err := runDelete(g, []string{"f1"}); !errors.Is(err, ErrUsage) {
		t.Fatalf("non-tty without yes: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("file should remain")
	}
}

func TestDeleteTTYRequiresYesWord(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{{ID: "f1", Path: p, Bytes: 1, Category: "c", Risk: findings.RiskSafeCache}}
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	g := &Global{Stdout: &bytes.Buffer{}, Stdin: strings.NewReader("no\n"), IsTTY: true}
	if err := runDelete(g, []string{"f1"}); err == nil {
		t.Fatal("expected abort")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("file should remain")
	}
	g2 := &Global{Stdout: &bytes.Buffer{}, Stdin: strings.NewReader("yes\n"), IsTTY: true}
	if err := runDelete(g2, []string{"f1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("expected deleted")
	}
}

func TestDeleteNonTTYYesRemoves(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	r := findings.NewReport(dir)
	r.Findings = []findings.Finding{{ID: "f1", Path: p, Bytes: 1, Category: "c", Risk: findings.RiskSafeCache}}
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
		t.Fatal(err)
	}
	g := &Global{Stdout: &bytes.Buffer{}, IsTTY: false}
	if err := runDelete(g, []string{"--yes", "f1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("expected deleted")
	}
}
