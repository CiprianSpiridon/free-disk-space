package scan

import (
	"os"
	"path/filepath"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "android-sim", Quick: false, Dev: false, Run: runAndroidSim})
}

func runAndroidSim(ctx *Context) error {
	var images []catalog.Entry
	var avds []catalog.Entry
	for _, e := range ctx.Catalog.Android {
		switch e.Category {
		case "android-system-images":
			images = append(images, e)
		case "android-avds":
			avds = append(avds, e)
		}
	}
	avdCount := 0
	avdReadOK := true
	for _, e := range avds {
		p := catalog.ExpandPath(e.Path, ctx.Home)
		ents, err := os.ReadDir(p)
		if err != nil {
			if !os.IsNotExist(err) {
				avdReadOK = false
				if os.IsPermission(err) {
					ctx.Report.Unreadable = append(ctx.Report.Unreadable, p)
				}
			}
			continue
		}
		avdCount += len(ents)
	}
	if avdCount == 0 {
		ctx.Report.NotPresent = append(ctx.Report.NotPresent, "android AVDs")
	}
	for _, e := range images {
		p := catalog.ExpandPath(e.Path, ctx.Home)
		sz := size.Of(p)
		if sz.Missing {
			continue
		}
		risk := findings.Risk(e.Risk)
		if avdReadOK && avdCount == 0 {
			risk = findings.RiskUnusedRuntime
		}
		ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
			ID: findings.IDSlug("android-image", p), Path: p, Bytes: sz.Allocated,
			Category: e.Category, Risk: risk,
			Why: "android system-images; avds=" + filepath.Base(p),
		})
	}
	return nil
}
