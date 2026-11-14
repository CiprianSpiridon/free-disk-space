package size

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScannerReusesDescendantsAndNewestFileDate(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(child, "file")
	if err := os.WriteFile(p, make([]byte, 16384), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * 24 * time.Hour)
	os.Chtimes(child, old, old)
	os.Chtimes(filepath.Dir(child), old, old)
	os.Chtimes(root, old, old)
	s := NewScanner()
	a := s.Of(root)
	visits := s.Visits
	b := s.Of(child)
	if b.Allocated <= 0 || a.Allocated < b.Allocated || s.Visits != visits {
		t.Fatalf("revisited subtree: %d -> %d (%+v %+v)", visits, s.Visits, a, b)
	}
	if a.Newest.Before(time.Now().Add(-time.Hour)) {
		t.Fatal("newest descendant lost", a.Newest)
	}
}

func TestScannerHardlinksCountOnceWithinTree(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "a"), 0755)
	os.Mkdir(filepath.Join(root, "b"), 0755)
	p := filepath.Join(root, "a", "x")
	q := filepath.Join(root, "b", "y")
	os.WriteFile(p, make([]byte, 32768), 0644)
	if err := os.Link(p, q); err != nil {
		t.Fatal(err)
	}
	s := NewScanner()
	got := s.Of(root)
	want := Of(root)
	if got.Allocated != want.Allocated {
		t.Fatalf("hardlinks %d want %d", got.Allocated, want.Allocated)
	}
	if s.Of(filepath.Dir(p)).Allocated != s.Of(filepath.Dir(q)).Allocated {
		t.Fatal("each sibling must still show its own size")
	}
}

func BenchmarkOverlappingSizes(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 80; i++ {
		d := filepath.Join(root, string(rune('A'+i)))
		os.Mkdir(d, 0755)
		for j := 0; j < 30; j++ {
			os.WriteFile(filepath.Join(d, string(rune('A'+j))), []byte("content"), 0644)
		}
	}
	children, _ := os.ReadDir(root)
	b.Run("separate-walks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Of(root)
			for _, d := range children {
				Of(filepath.Join(root, d.Name()))
			}
		}
	})
	b.Run("scan-cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s := NewScanner()
			s.Of(root)
			for _, d := range children {
				s.Of(filepath.Join(root, d.Name()))
			}
		}
	})
}
