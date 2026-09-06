package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/report"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func init() {
	AddCommand("scan", scanLong, runScan)
}

const scanLong = `scan [--quick|--dev|--mode=NAME] [--json]

Modes:
  quick  volume + known catalog paths + tmp children (not cargo-target-only)
  dev    leftover worktrees + project artifacts + catalog paths tagged dev
  full   all enabled phases (default)

scan never deletes. catalog add --scans and scans disable change what runs.
`

func runScan(g *Global, args []string) error {
	mode := "full"
	quick, dev := false, false
	for _, a := range args {
		switch {
		case a == "--quick":
			quick = true
		case a == "--dev":
			dev = true
		case strings.HasPrefix(a, "--mode="):
			mode = strings.TrimPrefix(a, "--mode=")
		case a == "--mode":
			return fmt.Errorf("%w: --mode needs a value", ErrUsage)
		case a == "--json":
			g.JSON = true
		default:
			return fmt.Errorf("%w: unknown flag %s", ErrUsage, a)
		}
	}
	if quick && dev {
		return fmt.Errorf("%w: combine --quick and --dev", ErrUsage)
	}
	if quick {
		mode = "quick"
	}
	if dev {
		mode = "dev"
	}
	catPath, tried, err := catalog.ResolveCatalogPath(g.Catalog)
	if err != nil {
		return fmt.Errorf("catalog: %v (tried %s)", err, strings.Join(tried, ", "))
	}
	bundled, err := catalog.Load(catPath)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	ov, err := catalog.LoadOverlay(catalog.OverlayPath(g.Config))
	if err != nil {
		return err
	}
	merged := catalog.Merge(bundled, ov, home)
	if !scan.ValidMode(mode, merged) {
		return fmt.Errorf("%w: unknown --mode=%s", ErrUsage, mode)
	}
	rep := findings.NewReport(home)
	rep.Host.Arch = runtime.GOARCH
	ctx := &scan.Context{Mode: mode, Catalog: merged, Home: home, Report: &rep}
	if err := scan.Run(ctx); err != nil {
		return err
	}
	_ = scan.WriteLastScan(scan.LastScanPath(), *ctx.Report)
	if wantJSON(g) {
		return report.JSON(g.Stdout, *ctx.Report)
	}
	return report.Markdown(g.Stdout, *ctx.Report, report.Options{
		ArtifactIdleDays: merged.Thresholds.ArtifactIdleDays,
		TmpIdleDays:      merged.Thresholds.TmpIdleDays,
	})
}
