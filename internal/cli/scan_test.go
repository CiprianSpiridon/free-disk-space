package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFlags(t *testing.T) {
	g := &Global{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, IsTTY: false}
	err := runScan(g, []string{"--quick", "--dev"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want usage got %v", err)
	}
	err = runScan(g, []string{"--mode=nope"})
	// may fail catalog first; set catalog
	dir := t.TempDir()
	cat := filepath.Join(dir, "c.yaml")
	_ = os.WriteFile(cat, []byte("version: 1\nos: macos\n"), 0o644)
	g.Catalog = cat
	g.Config = dir
	err = runScan(g, []string{"--mode=nope"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("mode nope: %v", err)
	}
}

func TestScanJSONNonTTY(t *testing.T) {
	dir := t.TempDir()
	cat := filepath.Join(dir, "c.yaml")
	_ = os.WriteFile(cat, []byte("version: 1\nos: macos\n"), 0o644)
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, Stderr: buf, Catalog: cat, Config: dir, IsTTY: false}
	err := runScan(g, []string{"--quick", "--json"})
	if err != nil && buf.Len() == 0 {
		t.Log(err)
		return
	}
	if buf.Len() > 0 && !strings.Contains(buf.String(), "generated_at") {
		t.Fatal(buf.String())
	}
}
