package scan

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "drill", Quick: false, Dev: false, Run: runDrill})
}

func runDrill(ctx *Context) error {
	thr := ctx.Catalog.Thresholds.DrillBytes
	if thr == 0 {
		thr = 1 << 30
	}
	timeout := 30 * time.Second
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var extra []findings.Finding
	for _, f := range ctx.Report.Findings {
		if f.Bytes < thr {
			continue
		}
		select {
		case <-cctx.Done():
			return nil
		default:
		}
		ents, err := os.ReadDir(f.Path)
		if err != nil {
			continue
		}
		var kids []findings.Finding
		ok := true
		for _, ent := range ents {
			if cctx.Err() != nil {
				ok = false
				break
			}
			if ent.Type()&os.ModeSymlink != 0 {
				continue
			}
			p := filepath.Join(f.Path, ent.Name())
			sz := size.Of(p)
			if sz.Missing || sz.Allocated == 0 {
				continue
			}
			kids = append(kids, findings.Finding{
				ID:       findings.IDSlug(f.Category+"-child", p),
				Path:     p,
				Bytes:    sz.Allocated,
				Category: f.Category,
				Risk:     f.Risk,
				Why:      "drill child of " + f.Path,
			})
		}
		if ok {
			extra = append(extra, kids...)
		}
	}
	ctx.Report.Findings = append(ctx.Report.Findings, extra...)
	return nil
}
