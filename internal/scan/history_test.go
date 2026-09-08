package scan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestHistoryFileID(t *testing.T) {
	id := HistoryFileID("2026-09-08T08:51:00Z", "quick")
	if id != "20260908T085100Z-quick" {
		t.Fatalf("id=%s", id)
	}
	id = HistoryFileID("2026-09-08T10:51:00+02:00", "Dev")
	if id != "20260908T085100Z-dev" {
		t.Fatalf("utc id=%s", id)
	}
	if HistoryFileID("2026-09-08T08:51:00Z", "") != "20260908T085100Z-full" {
		t.Fatal("empty mode")
	}
}

func TestHistoryDirXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	want := filepath.Join(dir, "freedisk", "history")
	if HistoryDir() != want {
		t.Fatal(HistoryDir())
	}
}

func TestWriteAndListHistory(t *testing.T) {
	dir := t.TempDir()
	old := findings.NewReport("/u")
	old.GeneratedAt = "2026-01-01T00:00:00Z"
	old.Mode = "full"
	old.Volume = findings.Volume{InUseBytes: 10, FreeBytes: 90}
	old.Findings = []findings.Finding{{ID: "a", Path: "/a", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	if _, _, err := WriteHistoryTo(dir, old); err != nil {
		t.Fatal(err)
	}
	quick := findings.NewReport("/u")
	quick.GeneratedAt = "2026-09-08T08:51:00Z"
	quick.Mode = "quick"
	quick.Volume = findings.Volume{InUseBytes: 50, FreeBytes: 50}
	quick.Findings = []findings.Finding{
		{ID: "b", Path: "/b", Bytes: 2, Category: "c", Risk: findings.RiskAsk},
		{ID: "c", Path: "/c", Bytes: 3, Category: "c", Risk: findings.RiskKeep},
	}
	id, path, err := WriteHistoryTo(dir, quick)
	if err != nil {
		t.Fatal(err)
	}
	if id != "20260908T085100Z-quick" {
		t.Fatalf("id=%s", id)
	}
	if filepath.Base(path) != id+".json" {
		t.Fatalf("path=%s", path)
	}
	list, err := ListHistoryDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != id || list[0].Mode != "quick" || list[0].Findings != 2 {
		t.Fatalf("%+v", list)
	}
	if list[1].Mode != "full" {
		t.Fatalf("older=%+v", list[1])
	}
	got, err := LookupHistory(dir, "latest")
	if err != nil || got.ID != id {
		t.Fatalf("latest %v %+v", err, got)
	}
	got, err = LookupHistory(dir, "1")
	if err != nil || got.ID != id {
		t.Fatalf("index %v %+v", err, got)
	}
	got, err = LookupHistory(dir, "20260908T085100Z-quick")
	if err != nil || got.Path != path {
		t.Fatalf("id %v %+v", err, got)
	}
	got, err = LookupHistory(dir, "20260908")
	if err != nil || got.Mode != "quick" {
		t.Fatalf("prefix %v %+v", err, got)
	}
	got, err = LookupHistory(dir, "2")
	if err != nil || got.Mode != "full" {
		t.Fatalf("index 2 %v %+v", err, got)
	}
}

func TestHistoryCollisionSuffix(t *testing.T) {
	dir := t.TempDir()
	r := findings.NewReport("/u")
	r.GeneratedAt = "2026-09-08T08:51:00Z"
	r.Mode = "quick"
	r.Findings = []findings.Finding{{ID: "a", Path: "/a", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	id1, _, err := WriteHistoryTo(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	id2, _, err := WriteHistoryTo(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "20260908T085100Z-quick" || id2 != "20260908T085100Z-quick-2" {
		t.Fatalf("%s %s", id1, id2)
	}
}

func TestHistoryMissingDir(t *testing.T) {
	list, err := ListHistoryDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil || len(list) != 0 {
		t.Fatalf("%v %+v", err, list)
	}
	if _, err := LookupHistory(filepath.Join(t.TempDir(), "nope"), "latest"); err == nil {
		t.Fatal("expected empty")
	}
}

func TestReadReportFileAllowsOldHistory(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "old.json")
	r := findings.NewReport("/u")
	r.GeneratedAt = "2000-01-01T00:00:00Z"
	r.Mode = "quick"
	r.Findings = []findings.Finding{{ID: "a", Path: "/p", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	if err := WriteLastScan(p, r); err != nil {
		t.Fatal(err)
	}
	got, err := ReadReportFile(p)
	if err != nil || got.Mode != "quick" {
		t.Fatalf("%v %+v", err, got)
	}
	if _, err := ReadLastScan(p); err == nil {
		t.Fatal("last-scan must still reject stale")
	}
}

func TestLookupHistoryUnknownAndAmbiguous(t *testing.T) {
	dir := t.TempDir()
	a := findings.NewReport("/u")
	a.GeneratedAt = "2026-09-08T08:51:00Z"
	a.Mode = "quick"
	a.Findings = []findings.Finding{{ID: "a", Path: "/a", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	b := findings.NewReport("/u")
	b.GeneratedAt = "2026-09-08T09:00:00Z"
	b.Mode = "dev"
	b.Findings = []findings.Finding{{ID: "b", Path: "/b", Bytes: 1, Category: "c", Risk: findings.RiskAsk}}
	if _, _, err := WriteHistoryTo(dir, a); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WriteHistoryTo(dir, b); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupHistory(dir, "nope"); err == nil {
		t.Fatal("unknown")
	}
	if _, err := LookupHistory(dir, "20260908"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous: %v", err)
	}
	if _, err := LookupHistory(dir, "9"); err == nil {
		t.Fatal("index")
	}
}
