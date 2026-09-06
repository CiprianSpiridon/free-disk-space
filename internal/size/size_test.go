package size

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAllocatedPositive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello allocated size test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Of(p)
	if r.Missing || r.Err != nil {
		t.Fatalf("missing=%v err=%v", r.Missing, r.Err)
	}
	if r.Allocated <= 0 {
		t.Fatalf("allocated=%d want >0", r.Allocated)
	}
}

func TestMissingPath(t *testing.T) {
	r := Of(filepath.Join(t.TempDir(), "nope"))
	if !r.Missing {
		t.Fatal("expected missing")
	}
	if r.Allocated != 0 {
		t.Fatalf("allocated=%d", r.Allocated)
	}
}

func TestNoOSexecImport(t *testing.T) {
	src, err := os.ReadFile("size.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "size.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range f.Imports {
		if im.Path.Value == `"os/exec"` {
			t.Fatal("internal/size must not import os/exec")
		}
	}
}
