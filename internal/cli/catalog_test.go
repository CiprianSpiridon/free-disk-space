package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
)

func TestCatalogAddUnassignDisable(t *testing.T) {
	dir := t.TempDir()
	g := &Global{Stdout: &bytes.Buffer{}, Config: dir, IsTTY: true}
	if err := runCatalog(g, nil); !errors.Is(err, ErrUsage) {
		t.Fatal(err)
	}
	if err := runCatalog(g, []string{"add"}); !errors.Is(err, ErrUsage) {
		t.Fatal("add without path")
	}
	p := filepath.Join(dir, "mycache")
	_ = os.Mkdir(p, 0o755)
	if err := runCatalog(g, []string{"add", p, "--scans", "dev"}); err != nil {
		t.Fatal(err)
	}
	ov, err := catalog.LoadOverlay(catalog.OverlayPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Add) != 1 || ov.Add[0].Path != p {
		t.Fatalf("%+v", ov.Add)
	}
	bundled := &catalog.Catalog{}
	m := catalog.Merge(bundled, ov, dir)
	if !containsEntry(m.ModePaths("dev"), p) {
		t.Fatal("dev")
	}
	if containsEntry(m.ModePaths("quick"), p) {
		t.Fatal("quick")
	}
	if err := runCatalog(g, []string{"add", filepath.Join(dir, "g"), "--glob"}); err != nil {
		t.Fatal(err)
	}
	ov, _ = catalog.LoadOverlay(catalog.OverlayPath(dir))
	ok := false
	for _, a := range ov.Add {
		if a.Glob {
			ok = true
		}
	}
	if !ok {
		t.Fatal("glob")
	}
	if err := runCatalog(g, []string{"unassign", p, "--from", "nope"}); !errors.Is(err, ErrUsage) {
		t.Fatal("unassign nope")
	}
	if err := runCatalog(g, []string{"unassign", p, "--from", "dev"}); err != nil {
		t.Fatal(err)
	}
	if err := runCatalog(g, []string{"disable", p}); err != nil {
		t.Fatal(err)
	}
}

func containsEntry(ents []catalog.Entry, p string) bool {
	for _, e := range ents {
		if e.Path == p {
			return true
		}
	}
	return false
}

func TestCatalogListJSON(t *testing.T) {
	dir := t.TempDir()
	cat := filepath.Join(dir, "c.yaml")
	_ = os.WriteFile(cat, []byte("version: 1\nos: macos\n"), 0o644)
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, Catalog: cat, Config: dir, IsTTY: false}
	if err := runCatalog(g, []string{"list", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[") {
		t.Fatal(buf.String())
	}
}
