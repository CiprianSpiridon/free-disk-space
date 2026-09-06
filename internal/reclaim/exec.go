package reclaim

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/policy"
)

// ProcessRunning is overridable in tests.
var ProcessRunning = func(name string) bool {
	if name == "" {
		return false
	}
	cmd := exec.Command("pgrep", "-x", name)
	return cmd.Run() == nil
}

func ownerBin(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	switch fields[0] {
	case "npm", "pnpm", "yarn", "cargo", "uv", "bun":
		return fields[0]
	}
	return ""
}

// ErrBusy is returned when the owner process is running.
var ErrBusy = fmt.Errorf("owner process running")

// ApplyOne deletes one finding after policy checks. Does not prompt.
func ApplyOne(f findings.Finding) error {
	if f.Risk == findings.RiskKeep || f.Risk == findings.RiskNever {
		return fmt.Errorf("refused risk %s", f.Risk)
	}
	if !policy.CanDelete(f.Path) {
		return fmt.Errorf("refused path %s", f.Path)
	}
	if repoGitTracked(f.Path) {
		return fmt.Errorf("git-tracked: %s", f.Path)
	}
	if f.Reclaim != nil {
		if bin := ownerBin(f.Reclaim.Cmd); bin != "" && ProcessRunning(bin) {
			return fmt.Errorf("%w: %s", ErrBusy, bin)
		}
	}
	return os.RemoveAll(f.Path)
}

func repoGitTracked(path string) bool {
	dir := path
	for i := 0; i < 12; i++ {
		if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil && st != nil {
			rel := path
			if r, err := filepath.Rel(dir, path); err == nil {
				rel = r
			}
			cmd := exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", rel)
			return cmd.Run() == nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// Lookup finding by id.
func Lookup(r findings.Report, id string) (findings.Finding, bool) {
	for _, f := range r.Findings {
		if f.ID == id {
			return f, true
		}
	}
	return findings.Finding{}, false
}
