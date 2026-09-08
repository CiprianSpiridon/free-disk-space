package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func TestHistoryEmpty(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: true}
	if err := runHistory(g, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no historic reports") {
		t.Fatal(buf.String())
	}
}

func TestHistoryListAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	quick := findings.NewReport("/u")
	quick.GeneratedAt = "2026-09-08T08:51:00Z"
	quick.Mode = "quick"
	quick.Volume = findings.Volume{ContainerBytes: 100, InUseBytes: 80, FreeBytes: 20}
	quick.Findings = []findings.Finding{{ID: "tmp-kensi", Path: "/private/tmp/kensi", Bytes: 9, Category: "tmp", Risk: findings.RiskAsk}}
	if _, _, err := scan.WriteHistoryTo(scan.HistoryDir(), quick); err != nil {
		t.Fatal(err)
	}
	dev := findings.NewReport("/u")
	dev.GeneratedAt = "2026-09-07T08:51:00Z"
	dev.Mode = "dev"
	dev.Volume = findings.Volume{ContainerBytes: 100, InUseBytes: 70, FreeBytes: 30}
	dev.Findings = []findings.Finding{{ID: "nm", Path: "/p/node_modules", Bytes: 4, Category: "node_modules", Risk: findings.RiskRebuildable}}
	if _, _, err := scan.WriteHistoryTo(scan.HistoryDir(), dev); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	g := &Global{Stdout: buf, IsTTY: false}
	if err := runHistory(g, []string{"list", "--json"}); err != nil {
		t.Fatal(err)
	}
	var rows []scan.HistoryEntry
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err, buf.String())
	}
	if len(rows) != 2 || rows[0].Mode != "quick" || rows[1].Mode != "dev" {
		t.Fatalf("%+v", rows)
	}

	buf.Reset()
	g.JSON = false
	g.IsTTY = true
	if err := runHistory(g, []string{"--type", "quick"}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "20260908T085100Z-quick") || strings.Contains(s, "dev") {
		t.Fatal(s)
	}

	buf.Reset()
	g.IsTTY = false
	g.JSON = false
	if err := runHistory(g, []string{"show", "latest", "--json"}); err != nil {
		t.Fatal(err)
	}
	var out findings.Report
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err, buf.String())
	}
	if out.Mode != "quick" || len(out.Findings) != 1 || out.Findings[0].Path != "/private/tmp/kensi" {
		t.Fatalf("%+v", out)
	}

	buf.Reset()
	g.JSON = false
	g.IsTTY = true
	if err := runHistory(g, []string{"show", "2"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "/p/node_modules") {
		t.Fatal(buf.String())
	}
}

func TestHistoryShowUnknown(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	g := &Global{Stdout: &bytes.Buffer{}, IsTTY: true}
	if err := runHistory(g, []string{"show", "nope"}); err == nil {
		t.Fatal("expected error")
	}
	if err := runHistory(g, []string{"show"}); !errors.Is(err, ErrUsage) {
		t.Fatalf("%v", err)
	}
	if err := runHistory(g, []string{"nope"}); !errors.Is(err, ErrUsage) {
		t.Fatalf("%v", err)
	}
}

func TestHistoryNoDelete(t *testing.T) {
	b, err := os.ReadFile("history.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "reclaim.Apply") || strings.Contains(s, "os.RemoveAll") {
		t.Fatal("history must not delete")
	}
}
