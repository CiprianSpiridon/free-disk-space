package reclaim

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func TestLookupDuplicateIDsFailClosed(t *testing.T) {
	r := findings.Report{Findings: []findings.Finding{
		{ID: "x", Path: "/a", Risk: findings.RiskAsk},
		{ID: "x", Path: "/b", Risk: findings.RiskAsk},
	}}
	if _, ok := Lookup(r, "x"); ok {
		t.Fatal("duplicates must not resolve")
	}
}

func TestApplyOneKeepNeverAndTmpRoot(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("z"), 0o644)
	if err := ApplyOne(findings.Finding{ID: "k", Path: p, Risk: findings.RiskKeep}); err == nil {
		t.Fatal("keep")
	}
	if err := ApplyOne(findings.Finding{ID: "n", Path: p, Risk: findings.RiskNever}); err == nil {
		t.Fatal("never")
	}
	if err := ApplyOne(findings.Finding{ID: "t", Path: "/private/tmp", Risk: findings.RiskAsk}); err == nil {
		t.Fatal("tmp root")
	}
}

func TestApplyOneDockerBusy(t *testing.T) {
	old := ProcessRunning
	ProcessRunning = func(name string) bool { return name == "Docker" }
	defer func() { ProcessRunning = old }()
	p := filepath.Join(t.TempDir(), "Docker.raw")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	err := ApplyOne(findings.Finding{ID: "d", Path: p, Risk: findings.RiskAsk, Category: "docker"})
	if err == nil {
		t.Fatal("expected docker busy")
	}
}

func TestApplyOneRemovesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("z"), 0o644)
	f := findings.Finding{ID: "x", Path: p, Risk: findings.RiskSafeCache}
	if err := ApplyOne(f); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("still exists")
	}
}

func TestApplyOneGitTrackedAndCargoAndBusy(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git: %s %v", out, err)
	}
	p := filepath.Join(repo, "tracked")
	_ = os.WriteFile(p, []byte("t"), 0o644)
	_ = exec.Command("git", "-C", repo, "add", "tracked").Run()
	_ = exec.Command("git", "-C", repo, "-c", "user.email=a@a", "-c", "user.name=a", "commit", "-m", "t").Run()
	f := findings.Finding{ID: "t", Path: p, Risk: findings.RiskRebuildable}
	if err := ApplyOne(f); err == nil {
		t.Fatal("expected git-tracked refuse")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("file should remain")
	}
	home, _ := os.UserHomeDir()
	if err := ApplyOne(findings.Finding{ID: "c", Path: filepath.Join(home, ".cargo"), Risk: findings.RiskAsk}); err == nil {
		t.Fatal("cargo root")
	}
	old := ProcessRunning
	ProcessRunning = func(string) bool { return true }
	defer func() { ProcessRunning = old }()
	q := filepath.Join(t.TempDir(), "n")
	_ = os.WriteFile(q, []byte("n"), 0o644)
	err := ApplyOne(findings.Finding{
		ID: "n", Path: q, Risk: findings.RiskSafeCache,
		Reclaim: &findings.Reclaim{Cmd: "npm cache clean --force"},
	})
	if err == nil {
		t.Fatal("busy")
	}
	if _, e := os.Stat(q); e != nil {
		t.Fatal("should remain")
	}
}
