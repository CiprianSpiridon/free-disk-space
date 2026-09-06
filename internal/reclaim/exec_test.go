package reclaim

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

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
