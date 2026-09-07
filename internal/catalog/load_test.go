package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoCatalog(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		p := filepath.Join(dir, "catalog", "macos-hotspots.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("could not find catalog/macos-hotspots.yaml")
		}
		dir = next
	}
}

func TestLoadBundledAndroidAndTmpAndDiscover(t *testing.T) {
	c, err := Load(repoCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	var sawAndroid, sawTmp bool
	for _, e := range c.AllEntries() {
		if e.Path == "/opt/homebrew/share/android-commandlinetools" {
			sawAndroid = true
		}
		if e.Path == "/tmp" {
			sawTmp = true
		}
	}
	if !sawAndroid {
		t.Fatal("missing android homebrew SDK path")
	}
	if !sawTmp {
		t.Fatal("missing /tmp")
	}
	if !c.WorkRootDiscover.Enabled {
		t.Fatal("work_root_discover.enabled want true")
	}
}

func TestLoadBundled(t *testing.T) {
	c, err := LoadBundled()
	if err != nil {
		t.Fatal(err)
	}
	if !c.WorkRootDiscover.Enabled {
		t.Fatal("bundled discover")
	}
	sawTmp := false
	for _, e := range c.AllEntries() {
		if e.Path == "/tmp" {
			sawTmp = true
		}
	}
	if !sawTmp {
		t.Fatal("bundled missing /tmp")
	}
}

func TestLoadMissingFileErrorsWithPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.yaml")
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), p) {
		t.Fatalf("error %q should contain path", err)
	}
}

func TestGlobZeroMatchesNotLoadError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	body := "version: 1\nos: macos\nhome:\n  - { path: \"/no/such/glob-*\", glob: true, category: x, risk: ask }\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Home) != 1 || !c.Home[0].Glob {
		t.Fatalf("want glob entry, got %+v", c.Home)
	}
}
