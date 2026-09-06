package catalog

import (
	"strings"
	"testing"
)

func TestStartingPoint(t *testing.T) {
	c, err := Load(repoCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range c.AllEntries() {
		paths = append(paths, e.Path)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "/tmp") {
		t.Fatal("missing /tmp")
	}
	if !c.WorkRootDiscover.Enabled {
		t.Fatal("discover")
	}
	sawAndroid := false
	sawGitAsk := false
	for _, e := range c.AllEntries() {
		if e.Path == "/opt/homebrew/share/android-commandlinetools" {
			sawAndroid = true
		}
		if strings.Contains(e.Path, ".cargo/git") && e.Risk == "ask" {
			sawGitAsk = true
		}
		if strings.Contains(e.Path, "work_cip") || strings.Contains(e.Path, "/tmp/hgdb") {
			t.Fatalf("forbidden path %s", e.Path)
		}
	}
	if !sawAndroid {
		t.Fatal("android sdk")
	}
	if !sawGitAsk {
		t.Fatal("cargo git ask")
	}
	arts := false
	for _, a := range c.Artifacts {
		if a.Name == ".tsx-cache" {
			arts = true
		}
	}
	if !arts {
		t.Fatal("tsx-cache")
	}
}
