package scan

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func init() {
	Register(Phase{Name: "apis", Quick: false, Dev: true, Run: runAPIs})
}

func execTimeout(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, name, args...).Output()
	return string(b), err
}

// DockerSystemDF is injected in tests.
var DockerSystemDF = func() (string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", err
	}
	return execTimeout("docker", "system", "df")
}

// BrewAutoremoveDry is injected in tests.
var BrewAutoremoveDry = func() (string, error) {
	if _, err := exec.LookPath("brew"); err != nil {
		return "", err
	}
	return execTimeout("brew", "autoremove", "--dry-run")
}

func runAPIs(ctx *Context) error {
	dockerDF(ctx)
	brewAutoremove(ctx)
	return nil
}

func dockerDF(ctx *Context) {
	out, err := DockerSystemDF()
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	why := "docker system df"
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 6 {
		lines = lines[:6]
	}
	why = strings.Join(lines, "; ")
	ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
		ID:       "docker-system-df",
		Path:     "docker",
		Category: "docker-df",
		Risk:     findings.RiskAsk,
		Why:      why,
		Reclaim: &findings.Reclaim{
			Cmd:    "docker system prune -a --volumes",
			DryRun: "docker system prune -a --volumes --dry-run",
		},
	})
}

func brewAutoremove(ctx *Context) {
	out, err := BrewAutoremoveDry()
	if err != nil {
		return
	}
	s := strings.TrimSpace(out)
	if s == "" {
		return
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "nothing") && strings.Contains(low, "uninstall") {
		return
	}
	why := s
	if len(why) > 400 {
		why = why[:400]
	}
	ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
		ID:       "brew-autoremove",
		Path:     "brew",
		Category: "homebrew",
		Risk:     findings.RiskAsk,
		Why:      why,
		Reclaim:  &findings.Reclaim{Cmd: "brew autoremove", DryRun: "brew autoremove --dry-run"},
	})
}
