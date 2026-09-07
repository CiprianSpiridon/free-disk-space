package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "drill", Quick: false, Dev: false, Run: runDrill})
}

// Soft caps. Hitting them keeps children already sized and still prints a report.
var (
	drillBudget       = 3 * time.Minute
	drillChildTimeout = 20 * time.Second
)

func runDrill(ctx *Context) error {
	thr := ctx.Catalog.Thresholds.DrillBytes
	if thr == 0 {
		thr = 1 << 30
	}
	minChild := ctx.Catalog.Thresholds.ReportBytes
	deadline := time.Now().Add(drillBudget)
	seen := map[string]struct{}{}
	for _, f := range ctx.Report.Findings {
		seen[f.Path] = struct{}{}
	}
	var parents []findings.Finding
	for _, f := range ctx.Report.Findings {
		if f.Bytes < thr || skipDrill(f) {
			continue
		}
		parents = append(parents, f)
	}
	sort.SliceStable(parents, func(i, j int) bool {
		return parents[i].Bytes > parents[j].Bytes
	})
	var extra []findings.Finding
	skipped := 0
	for i, f := range parents {
		if time.Now().After(deadline) {
			skipped += len(parents) - i
			break
		}
		ctx.logf("drilling %s (%d/%d)", f.Path, i+1, len(parents))
		ents, err := os.ReadDir(f.Path)
		if err != nil {
			if os.IsPermission(err) {
				ctx.Report.Unreadable = append(ctx.Report.Unreadable, f.Path)
			}
			continue
		}
		for _, ent := range ents {
			if time.Now().After(deadline) {
				skipped++
				break
			}
			if ent.Type()&os.ModeSymlink != 0 {
				continue
			}
			p := filepath.Join(f.Path, ent.Name())
			if _, ok := seen[p]; ok {
				continue
			}
			info, err := ent.Info()
			if err != nil {
				continue
			}
			var sz size.Result
			if !info.IsDir() {
				sz = size.OfFileInfo(info)
			} else {
				cctx, cancel := context.WithTimeout(context.Background(), drillChildTimeout)
				sz = size.OfContext(cctx, p)
				cancel()
				if sz.Err != nil {
					ctx.logf("skip %s: %v", p, sz.Err)
					continue
				}
			}
			noteUnreadable(ctx.Report, sz.Unreadable...)
			if sz.Missing || sz.Allocated == 0 {
				continue
			}
			if minChild > 0 && sz.Allocated < minChild {
				continue
			}
			last := info.ModTime().UTC().Format("2006-01-02")
			seen[p] = struct{}{}
			extra = append(extra, findings.Finding{
				ID:       findings.IDSlug(f.Category+"-child", p),
				Path:     p,
				Bytes:    sz.Allocated,
				Category: f.Category,
				Risk:     f.Risk,
				LastUsed: last,
				Why:      "drill child of " + f.Path,
				Reclaim:  &findings.Reclaim{Cmd: rmRf(p)},
			})
		}
	}
	if skipped > 0 {
		ctx.logf("drill budget %s reached; skipped remaining, keeping %d children", drillBudget, len(extra))
	}
	ctx.Report.Findings = append(ctx.Report.Findings, extra...)
	return nil
}

func skipDrill(f findings.Finding) bool {
	switch f.Risk {
	case findings.RiskKeep, findings.RiskNever, findings.RiskLeftoverWorktree, findings.RiskSafeCache:
		return true
	}
	switch f.Category {
	case "tmp", "tmp-user", "tmp-child":
		return true
	}
	base := filepath.Base(f.Path)
	switch base {
	case "node_modules", "target", ".next", "vendor", "dist", "build", "out",
		".turbo", ".parcel-cache", "__pycache__":
		return true
	}
	return false
}
