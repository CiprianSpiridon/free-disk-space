package scan

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestRunQuickSkipsNonQuick(t *testing.T) {
	var called bool
	Register(Phase{Name: "stub-full", Quick: false, Dev: false, Run: func(*Context) error {
		called = true
		return nil
	}})
	home := t.TempDir()
	rep := findings.NewReport(home)
	ctx := &Context{
		Mode:    "quick",
		Catalog: &catalog.Catalog{},
		Home:    home,
		Report:  &rep,
		Diskutil: func() (string, error) {
			return `Size (Capacity Ceiling): 10 GB (10000000000 Bytes)
Capacity In Use By Volumes: 4 GB (4000000000 Bytes)
Capacity Not Allocated: 6 GB (6000000000 Bytes)
`, nil
		},
	}
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("non-quick stub should not run")
	}
}

func TestRunUnknownModeAndConflict(t *testing.T) {
	if ValidMode("nope", &catalog.Catalog{}) {
		t.Fatal("nope should be invalid")
	}
}

func TestRunNoDeleteImports(t *testing.T) {
	b, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "os.Remove") || strings.Contains(s, "internal/reclaim") {
		t.Fatal("run.go must not delete")
	}
	_ = bytes.MinRead
}

func TestDisabledScansSkip(t *testing.T) {
	var called bool
	Register(Phase{Name: "skipme", Quick: true, Dev: true, Run: func(*Context) error {
		called = true
		return nil
	}})
	home := t.TempDir()
	cat := catalog.Merge(&catalog.Catalog{}, &catalog.Overlay{DisableScans: []string{"skipme"}}, home)
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "full", Catalog: cat, Home: home, Report: &rep, Diskutil: func() (string, error) {
		return `Size (Capacity Ceiling): 10 GB (10000000000 Bytes)
Capacity In Use By Volumes: 4 GB (4000000000 Bytes)
Capacity Not Allocated: 6 GB (6000000000 Bytes)
`, nil
	}}
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("disabled phase ran")
	}
}
