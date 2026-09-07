package scan

import (
	"os"
	"path/filepath"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "tmp", Quick: true, Dev: true, Run: runTmp})
}

func resolveTmpRoot(root string) string {
	clean := filepath.Clean(root)
	switch clean {
	case "/tmp", "/private/tmp", "/var/tmp", "/private/var/tmp":
		if rp, err := filepath.EvalSymlinks(root); err == nil {
			return rp
		}
	}
	return root
}

func runTmp(ctx *Context) error {
	idle := ctx.Catalog.Thresholds.TmpIdleDays
	if idle == 0 {
		idle = 7
	}
	min := ctx.Catalog.Thresholds.ArtifactListMinBytes
	if min == 0 {
		min = 5 << 20
	}
	seen := map[string]struct{}{}
	for _, e := range ctx.Catalog.Tmp {
		root := catalog.ExpandPath(e.Path, ctx.Home)
		if root == "" {
			continue
		}
		root = resolveTmpRoot(root)
		key := seenKey(root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ents, err := os.ReadDir(root)
		if err != nil {
			if os.IsPermission(err) {
				ctx.Report.Unreadable = append(ctx.Report.Unreadable, root)
			}
			continue
		}
		for _, ent := range ents {
			p := filepath.Join(root, ent.Name())
			if ent.Type()&os.ModeSymlink != 0 {
				continue
			}
			sz := size.Of(p)
			noteUnreadable(ctx.Report, sz.Unreadable...)
			if sz.Missing || sz.Allocated < min {
				continue
			}
			info, err := ent.Info()
			last := ""
			days := 0
			if err == nil {
				last = info.ModTime().UTC().Format("2006-01-02")
				days = int(time.Since(info.ModTime()).Hours() / 24)
			}
			why := "tmp child"
			if days >= idle {
				why = "untouched " + last + " in tmp"
			}
			ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
				ID:       findings.IDSlug("tmp", p),
				Path:     p,
				Bytes:    sz.Allocated,
				Category: "tmp",
				Risk:     findings.RiskAsk,
				LastUsed: last,
				Why:      why,
				Reclaim:  &findings.Reclaim{Cmd: "rm -rf " + p},
			})
		}
	}
	return nil
}
