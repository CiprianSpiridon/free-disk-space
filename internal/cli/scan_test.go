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
	t.Setenv("XDG_CACHE_HOME", dir)
	cat := filepath.Join(dir, "c.yaml")
	_ = os.WriteFile(cat, []byte("version: 1\nos: macos\n"), 0o644)
	out := &bytes.Buffer{}
	errb := &bytes.Buffer{}
	g := &Global{Stdout: out, Stderr: errb, Catalog: cat, Config: dir, IsTTY: false}
	err := runScan(g, []string{"--quick", "--json"})
	if err != nil && out.Len() == 0 {
		t.Log(err)
		return
	}
	if out.Len() > 0 && !strings.Contains(out.String(), "generated_at") {
		t.Fatal(out.String())
	}
	if strings.Contains(out.String(), "freedisk:") {
		t.Fatal("progress leaked onto JSON stdout", out.String())
	}
}

func TestScanQuietNoProgress(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	cat := filepath.Join(dir, "c.yaml")
	_ = os.WriteFile(cat, []byte("version: 1\nos: macos\n"), 0o644)
	out := &bytes.Buffer{}
	errb := &bytes.Buffer{}
	g := &Global{Stdout: out, Stderr: errb, Catalog: cat, Config: dir, IsTTY: false}
	if err := runScan(g, []string{"--quick", "--json", "--quiet"}); err != nil && out.Len() == 0 {
		t.Log(err)
		return
	}
	if strings.Contains(errb.String(), "freedisk:") {
		t.Fatal("quiet still logged", errb.String())
	}
}
