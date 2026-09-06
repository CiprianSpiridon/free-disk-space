package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func TestScansListDisableEnable(t *testing.T) {
	dir := t.TempDir()
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, Config: dir, IsTTY: false}
	if err := runScans(g, []string{"list", "--json"}); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err, buf.String())
	}
	found := false
	for _, r := range rows {
		if r["name"] == "artifacts" && r["enabled"] == true {
			found = true
		}
	}
	if !found {
		t.Fatal(rows)
	}
	if err := runScans(g, []string{"disable", "volume"}); !errors.Is(err, ErrUsage) {
		t.Fatal("volume")
	}
	if err := runScans(g, []string{"disable", "artifacts"}); err != nil {
		t.Fatal(err)
	}
	ov, _ := catalog.LoadOverlay(catalog.OverlayPath(dir))
	ok := false
	for _, d := range ov.DisableScans {
		if d == "artifacts" {
			ok = true
		}
	}
	if !ok {
		t.Fatal(ov.DisableScans)
	}
	if err := runScans(g, []string{"enable", "artifacts"}); err != nil {
		t.Fatal(err)
	}
}

func TestScansAddRemove(t *testing.T) {
	dir := t.TempDir()
	g := &Global{Stdout: &bytes.Buffer{}, Config: dir}
	if err := runScans(g, []string{"add", "rust", "--types", "artifacts,worktrees"}); err != nil {
		t.Fatal(err)
	}
	ov, _ := catalog.LoadOverlay(catalog.OverlayPath(dir))
	if len(ov.Modes["rust"].Types) != 2 {
		t.Fatal(ov.Modes)
	}
	cat := catalog.Merge(&catalog.Catalog{}, ov, dir)
	if !scan.ValidMode("rust", cat) {
		t.Fatal("mode rust")
	}
	if err := runScans(g, []string{"remove", "rust"}); err != nil {
		t.Fatal(err)
	}
	ov, _ = catalog.LoadOverlay(catalog.OverlayPath(dir))
	cat = catalog.Merge(&catalog.Catalog{}, ov, dir)
	if scan.ValidMode("rust", cat) {
		t.Fatal("removed rust still valid")
	}
	if err := runScans(g, []string{"remove", "quick"}); !errors.Is(err, ErrUsage) {
		t.Fatal("remove quick")
	}
	if err := runScans(g, []string{"add", "artifacts", "--types", "known"}); !errors.Is(err, ErrUsage) {
		t.Fatal("collide")
	}
}
