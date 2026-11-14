package cli

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/report"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	AddCommand("scan", scanLong, runScan)
}

const scanLong = `scan [--quick|--dev|--mode=NAME] [--workers=N] [--json] [--quiet]

--workers=N bounds parallel directory reads (1-64; default 5).

Modes:
  quick  volume + known catalog paths (named children, not parent blobs) + home/Library depth-1 + tmp children
         Includes Apple simulator inventory at >=95% container fullness.
         Typical 1-5 min; up to ~10 min if $HOME has a 100GB+ work dir.
  dev    leftover worktrees + project artifacts + catalog paths tagged dev
         Typical 3-10 min.
  full   all enabled phases (default)
         Typical 5-15 min. Drill is capped at ~3 min and never aborts the scan.

Agents: do not kill a scan that is still printing "freedisk: phase" or
"still walking" on stderr. Prefer --quick first. Progress is stderr-only
so JSON/markdown stdout stays usable. --quiet silences progress.
Markdown tables list id, full path, last used, and a reclaim command
(catalog command or rm -rf PATH).

scan never deletes. catalog add --scans and scans disable change what runs.
Each scan also archives $XDG_CACHE_HOME/freedisk/history/YYYYMMDDTHHMMSSZ-MODE.json
(display-only; why/delete still use last-scan.json).
`

func runScan(g *Global, args []string) error {
	mode := "full"
	workers := size.DefaultWorkers
	quick, dev, quiet := false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--workers" || strings.HasPrefix(a, "--workers="):
			value := strings.TrimPrefix(a, "--workers=")
			if a == "--workers" {
				i++
				if i >= len(args) {
					return fmt.Errorf("%w: --workers needs a value", ErrUsage)
				}
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 64 {
				return fmt.Errorf("%w: --workers must be 1-64", ErrUsage)
			}
			workers = n
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
		case a == "--quiet" || a == "-q":
			quiet = true
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
	bundled, tried, err := catalog.LoadDefault(g.Catalog)
	if err != nil {
		return fmt.Errorf("catalog: %v (tried %s)", err, strings.Join(tried, ", "))
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
	ctx := &scan.Context{Mode: mode, Catalog: merged, Home: home, Report: &rep, Workers: workers, OverlayPath: catalog.OverlayPath(g.Config)}
	if !quiet && g.Stderr != nil {
		ctx.Log = func(msg string) {
			fmt.Fprintf(g.Stderr, "freedisk: %s\n", msg)
		}
	}
	if err := scan.Run(ctx); err != nil {
		return err
	}
	ctx.Report.Mode = mode
	if err := scan.WriteLastScan(scan.LastScanPath(), *ctx.Report); err != nil {
		return fmt.Errorf("write last-scan: %w", err)
	}
	if err := scan.WriteHistory(*ctx.Report); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	if wantJSON(g) {
		return report.JSON(g.Stdout, *ctx.Report)
	}
	return report.Markdown(g.Stdout, *ctx.Report, report.Options{
		ArtifactIdleDays: merged.Thresholds.ArtifactIdleDays,
		TmpIdleDays:      merged.Thresholds.TmpIdleDays,
	})
}
