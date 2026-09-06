package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/report"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func init() {
	AddCommand("why", "why [--json] [--limit N]", runWhy)
}

func runWhy(g *Global, args []string) error {
	limit := 20
	for i := 0; i < len(args); i++ {
		if args[i] == "--json" {
			g.JSON = true
		}
	}
	p := scan.LastScanPath()
	r, err := scan.ReadLastScan(p)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(g.Stdout, "no last-scan at %s; run scan first\n", p)
			return nil
		}
		return err
	}
	var keep []findings.Finding
	for _, f := range r.Findings {
		switch f.Risk {
		case findings.RiskSafeCache, findings.RiskRebuildable, findings.RiskLeftoverWorktree, findings.RiskUnusedRuntime, findings.RiskAsk:
			keep = append(keep, f)
		}
	}
	sort.Slice(keep, func(i, j int) bool { return keep[i].Bytes > keep[j].Bytes })
	if len(keep) > limit {
		keep = keep[:limit]
	}
	r.Findings = keep
	if wantJSON(g) {
		return report.JSON(g.Stdout, r)
	}
	return report.Markdown(g.Stdout, r, report.Options{})
}
