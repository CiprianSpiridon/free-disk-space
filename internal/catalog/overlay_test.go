package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOverlayAddDevOnly(t *testing.T) {
	home := t.TempDir()
	bundled := &Catalog{
		Home: []Entry{{Path: "~/.npm", Category: "npm-cache", Risk: "safe-cache"}},
	}
	ov := &Overlay{
		Add: []Entry{{Path: "~/custom-cache", Scans: []string{"dev"}, Risk: "ask", Category: "user"}},
	}
	m := Merge(bundled, ov, home)
	dev := m.ModePaths("dev")
	quick := m.ModePaths("quick")
	want := filepath.Join(home, "custom-cache")
	if !containsPath(dev, want) {
		t.Fatalf("dev missing %s: %+v", want, paths(dev))
	}
	if containsPath(quick, want) {
		t.Fatalf("quick should not include custom-cache")
	}
}

func TestOverlayDisableUnknownAndKnown(t *testing.T) {
	home := "/Users/test"
	bundled := &Catalog{
		Node: []Entry{{Path: "~/.npm", Category: "npm-cache", Risk: "safe-cache"}},
	}
	ov := &Overlay{Disable: []string{"~/.npm", "~/not-in-catalog"}}
	m := Merge(bundled, ov, home)
	if containsPath(m.ModePaths("full"), ExpandPath("~/.npm", home)) {
		t.Fatal("disabled ~/.npm still present")
	}
}

func TestOverlayUnassignAndDisableScans(t *testing.T) {
	home := "/Users/test"
	npm := ExpandPath("~/.npm", home)
	bundled := &Catalog{
		Node: []Entry{{Path: "~/.npm", Category: "npm-cache", Risk: "safe-cache"}},
	}
	ov := &Overlay{
		Unassign:     []Unassign{{Path: "~/.npm", From: []string{"quick"}}},
		DisableScans: []string{"drill"},
	}
	m := Merge(bundled, ov, home)
	if containsPath(m.ModePaths("quick"), npm) {
		t.Fatal("unassign from quick failed")
	}
	if !containsPath(m.ModePaths("full"), npm) {
		t.Fatal("full should still have npm")
	}
	ds := m.DisabledScans()
	if len(ds) != 1 || ds[0] != "drill" {
		t.Fatalf("DisabledScans=%v", ds)
	}
}

func containsPath(ents []Entry, p string) bool {
	for _, e := range ents {
		if e.Path == p {
			return true
		}
	}
	return false
}

func paths(ents []Entry) []string {
	var s []string
	for _, e := range ents {
		s = append(s, e.Path)
	}
	return s
}

func TestOverlayAddMergesLibraryNotDuplicateHome(t *testing.T) {
	home := "/Users/test"
	lib := ExpandPath("~/Library/Caches", home)
	bundled := &Catalog{
		Library: []Entry{{Path: "~/Library/Caches", Category: "caches", Risk: "ask"}},
	}
	ov := &Overlay{
		Add: []Entry{{Path: "~/Library/Caches", Scans: []string{"dev"}, Risk: "ask", Category: "caches"}},
	}
	m := Merge(bundled, ov, home)
	n := 0
	for _, e := range m.AllEntries() {
		if e.Path == lib {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want 1 library entry, got %d", n)
	}
	if !containsPath(m.Library, lib) {
		t.Fatal("should stay in Library")
	}
	if containsPath(m.Home, lib) {
		t.Fatal("should not duplicate into Home")
	}
}

func TestLoadOverlayMissing(t *testing.T) {
	ov, err := LoadOverlay(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil || ov == nil {
		t.Fatalf("missing overlay should be empty, err=%v", err)
	}
	_ = os.ErrNotExist
}
