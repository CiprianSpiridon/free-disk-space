package size

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBulkMetadataMatchesStat(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a file", "unicode-λ", "normal"} {
		if err := os.WriteFile(filepath.Join(root, name), make([]byte, 32768), 0644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Create(filepath.Join(root, "sparse"))
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(1 << 30)
	f.WriteAt([]byte("tail"), (1<<30)-4)
	f.Close()
	os.Mkdir(filepath.Join(root, "dir"), 0755)
	os.Symlink("normal", filepath.Join(root, "link"))
	os.Link(filepath.Join(root, "normal"), filepath.Join(root, "hard"))
	entries, err := readBulkDir(root)
	if err != nil {
		t.Fatal("native bulk path unavailable", err)
	}
	if len(entries) != 7 {
		t.Fatal("lost directory entries", len(entries))
	}
	for _, e := range entries {
		fi, err := os.Lstat(filepath.Join(root, e.name))
		if err != nil {
			t.Fatal(err)
		}
		want := metadata(fi)
		if e.allocated != want.allocated || e.apparent != want.apparent || e.inode != want.inode || e.device != want.device || e.kind != want.kind || !e.modified.Equal(want.modified) {
			t.Fatalf("%s: bulk=%+v stat=%+v", e.name, e, want)
		}
	}
}

func TestPrivateBytesDoNotAddClonedSiblings(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "original")
	dst := filepath.Join(root, "clone")
	if err := os.WriteFile(src, make([]byte, 1<<20), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/cp", "-c", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("create APFS clone: %v %s", err, out)
	}
	r := NewScanner().Of(root)
	if !r.PrivateKnown {
		t.Fatal("private byte metadata unavailable on this APFS fixture", r)
	}
	if r.Private >= r.Allocated {
		t.Fatalf("clone sizes still added: private=%d allocated=%d", r.Private, r.Allocated)
	}
}

func TestNativeScannerParity(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "child"), 0755)
	os.WriteFile(filepath.Join(root, "child", "file"), make([]byte, 32768), 0644)
	old := time.Now().Add(-24 * time.Hour)
	os.Chtimes(root, old, old)
	native := NewScanner().Of(root)
	fallback := NewScanner()
	fallback.readEntries = readStatDir
	want := fallback.Of(root)
	if native.Allocated != want.Allocated || native.Apparent != want.Apparent || !native.Newest.Equal(want.Newest) {
		t.Fatalf("native=%+v fallback=%+v", native, want)
	}
}
