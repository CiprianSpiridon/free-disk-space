package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestMarkdownNotPresent(t *testing.T) {
	r := findings.NewReport("/Users/x")
	r.NotPresent = []string{"conda"}
	var buf bytes.Buffer
	if err := Markdown(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Not present") || !strings.Contains(buf.String(), "conda") {
		t.Fatal(buf.String())
	}
}

func TestMarkdownKeepVsIdleRebuildableVsAskTmp(t *testing.T) {
	r := findings.NewReport("/Users/x")
	r.Volume = findings.Volume{ContainerBytes: 100, InUseBytes: 40, FreeBytes: 60}
	old := time.Now().Add(-90 * 24 * time.Hour).Format("2006-01-02")
	yest := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	r.Findings = []findings.Finding{
		{ID: "keep-src", Path: "/src", Bytes: 1, Category: "work", Risk: findings.RiskKeep},
		{ID: "old-nm", Path: "/a/node_modules", Bytes: 9, Category: "node_modules", Risk: findings.RiskRebuildable, LastUsed: old},
		{ID: "new-nm", Path: "/b/node_modules", Bytes: 9, Category: "node_modules", Risk: findings.RiskRebuildable, LastUsed: yest},
		{ID: "old-tmp", Path: "/tmp/x", Bytes: 9, Category: "tmp", Risk: findings.RiskAsk, LastUsed: time.Now().Add(-30 * 24 * time.Hour).Format("2006-01-02")},
		{ID: "kensi", Path: "/private/tmp/kensi-app", Bytes: 200 << 30, Category: "tmp", Risk: findings.RiskAsk, LastUsed: yest},
	}
	var buf bytes.Buffer
	if err := Markdown(&buf, r, Options{ArtifactIdleDays: 30, TmpIdleDays: 7}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	reclaim, _, _ := strings.Cut(s, "## Ask first")
	if strings.Contains(reclaim, "keep-src") {
		t.Fatal("keep in reclaimable")
	}
	if !strings.Contains(reclaim, "old-nm") {
		t.Fatal("old rebuildable should be reclaimable")
	}
	if !strings.Contains(s, "new-nm") || strings.Contains(reclaim, "new-nm") {
		t.Fatal("new rebuildable should be ask first")
	}
	if !strings.Contains(reclaim, "old-tmp") {
		t.Fatal("idle tmp child should be reclaimable")
	}
	if !strings.Contains(reclaim, "kensi") {
		t.Fatal("huge /private/tmp/kensi-* must be high-confidence even if recent")
	}
	if !strings.Contains(s, "40") && !strings.Contains(s, "B") {
		t.Fatal("volume missing", s)
	}
}
