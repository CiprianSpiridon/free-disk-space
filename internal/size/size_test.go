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

func TestUnreadablePermission(t *testing.T) {
	dir := t.TempDir()
	denied := filepath.Join(dir, "secret")
	if err := os.Mkdir(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(denied, 0o755)
	r := Of(dir)
	if len(r.Unreadable) == 0 && os.Getuid() == 0 {
		t.Skip("root bypasses chmod 000")
	}
	if os.Getuid() != 0 && len(r.Unreadable) == 0 {
		t.Fatalf("expected unreadable, got %+v", r)
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
