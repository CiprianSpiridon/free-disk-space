package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestWorktreesClaudeCountNoRmRoot(t *testing.T) {
	home := t.TempDir()
	wt := filepath.Join(home, "work", "repo", ".claude", "worktrees")
	_ = os.MkdirAll(filepath.Join(wt, "a"), 0o755)
	_ = os.MkdirAll(filepath.Join(wt, "b"), 0o755)
	cat := &catalog.Catalog{
		WorkRoots: []catalog.Entry{{Path: filepath.Join(home, "work")}},
		WorktreeMarkers: []catalog.WorktreeMarker{
			{PathSuffix: "/.claude/worktrees", Kind: "claude"},
		},
		Thresholds: catalog.Thresholds{WorktreeIdleDays: 14, WorktreeInflightHours: 24},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runWorktrees(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("no findings")
	}
	sawChildRm := false
	for _, f := range rep.Findings {
		if f.Reclaim != nil && strings.Contains(f.Reclaim.Cmd, "rm -rf") && strings.HasSuffix(f.Path, "worktrees") {
			t.Fatalf("rm -rf of worktrees root: %s", f.Reclaim.Cmd)
		}
		if f.Reclaim != nil && strings.Contains(f.Reclaim.Cmd, "rm -rf") && (strings.HasSuffix(f.Path, "/a") || strings.HasSuffix(f.Path, "/b")) {
			sawChildRm = true
		}
		if f.Count != 2 && f.Path == wt {
			t.Fatalf("count=%d", f.Count)
		}
	}
	if !sawChildRm {
		t.Fatal("claude worktree children need rm -rf PATH")
	}
}

func TestWorktreesMissingGrokNotPresent(t *testing.T) {
	home := t.TempDir()
	cat := &catalog.Catalog{
		WorktreeMarkers: []catalog.WorktreeMarker{{Path: "~/.grok/worktrees", Kind: "grok"}},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Catalog: cat, Home: home, Report: &rep}
	_ = runWorktrees(ctx)
	ok := false
	for _, n := range rep.NotPresent {
		if n == "grok-worktrees" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("not_present=%v", rep.NotPresent)
	}
}

func TestWorktreesInflightNotLeftover(t *testing.T) {
	home := t.TempDir()
	wt := filepath.Join(home, ".claude", "worktrees")
	_ = os.MkdirAll(filepath.Join(wt, "now"), 0o755)
	_ = os.Chtimes(wt, time.Now(), time.Now())
	cat := &catalog.Catalog{
		WorktreeMarkers: []catalog.WorktreeMarker{{Path: wt, Kind: "claude"}},
		Thresholds:      catalog.Thresholds{WorktreeIdleDays: 14, WorktreeInflightHours: 24},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	_ = runWorktrees(ctx)
	for _, f := range rep.Findings {
		if f.Risk == findings.RiskLeftoverWorktree {
			t.Fatal("fresh should not be leftover")
		}
	}
	quick := false
	for _, p := range PhasesForTest() {
		if p.Name == "worktrees" && p.Quick {
			quick = true
		}
	}
	if quick {
		t.Fatal("worktrees must not be quick")
	}
}

func TestWorktreesAgesChildNotParent(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".grok", "worktrees")
	proj := filepath.Join(root, "app")
	fresh := filepath.Join(proj, "subagent-now")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-20 * 24 * time.Hour)
	_ = os.Chtimes(root, old, old)
	_ = os.Chtimes(proj, old, old)
	_ = os.Chtimes(fresh, time.Now(), time.Now())
	cat := &catalog.Catalog{
		WorktreeMarkers: []catalog.WorktreeMarker{{Path: root, Kind: "grok"}},
		Thresholds:      catalog.Thresholds{WorktreeIdleDays: 14, WorktreeInflightHours: 24},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runWorktrees(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("no findings")
	}
	for _, f := range rep.Findings {
		if f.Risk == findings.RiskLeftoverWorktree {
			t.Fatalf("fresh subagent should not make leftover: %+v", f)
		}
	}
}

func TestGitWorktreePorcelainSkipsMain(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "work", "repo")
	extra := filepath.Join(home, "work", "linked")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git: %s %v", out, err)
	}
	old := GitWorktreeList
	GitWorktreeList = func(string) ([]byte, error) {
		return []byte("worktree " + repo + "\nHEAD abc\n\nworktree " + extra + "\nHEAD def\n"), nil
	}
	defer func() { GitWorktreeList = old }()
	_ = os.MkdirAll(filepath.Join(repo, ".git", "worktrees", "linked"), 0o755)
	cat := &catalog.Catalog{
		WorkRoots:       []catalog.Entry{{Path: filepath.Join(home, "work")}},
		WorktreeMarkers: []catalog.WorktreeMarker{{PathSuffix: "/.git/worktrees", Kind: "git-metadata"}},
		Thresholds:      catalog.Thresholds{MaxWalkDepth: 8, WorktreeIdleDays: 14, WorktreeInflightHours: 24},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runWorktrees(ctx); err != nil {
		t.Fatal(err)
	}
	var sawExtra, sawMain, sawMeta bool
	for _, f := range rep.Findings {
		if f.Path == extra {
			sawExtra = true
		}
		if f.Path == repo {
			sawMain = true
		}
		if strings.HasSuffix(f.Path, ".git/worktrees") {
			sawMeta = true
		}
	}
	if !sawExtra {
		t.Fatalf("missing extra worktree: %+v", rep.Findings)
	}
	for _, f := range rep.Findings {
		if f.Path == extra && (f.Reclaim == nil || !strings.Contains(f.Reclaim.Cmd, "git worktree remove")) {
			t.Fatalf("git reclaim: %v", f.Reclaim)
		}
	}
	if sawMain || sawMeta {
		t.Fatalf("should skip main repo and metadata dir: %+v", rep.Findings)
	}
}

func TestWorktreesWalksDiscoveredRoots(t *testing.T) {
	home := t.TempDir()
	wt := filepath.Join(home, "work_cip", "repo", ".claude", "worktrees", "a")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(home, "work_cip", "package.json"), []byte("{}"), 0o644)
	cat := &catalog.Catalog{
		WorktreeMarkers: []catalog.WorktreeMarker{{PathSuffix: "/.claude/worktrees", Kind: "claude"}},
		WorkRootDiscover: catalog.WorkRootDiscover{
			Enabled:        true,
			ProjectMarkers: []string{"package.json"},
		},
		Thresholds: catalog.Thresholds{MaxWalkDepth: 8, WorktreeIdleDays: 14, WorktreeInflightHours: 24},
	}
	rep := findings.NewReport(home)
	ctx := &Context{Mode: "dev", Catalog: cat, Home: home, Report: &rep}
	if err := runWorktrees(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected discovered work root walk")
	}
}
