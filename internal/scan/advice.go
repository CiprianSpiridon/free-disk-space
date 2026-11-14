package scan

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func catalogAdvice(ctx *Context) *findings.CatalogAdvice {
	overlay := ctx.OverlayPath
	if overlay == "" {
		overlay = catalog.OverlayPath("")
	}
	advice := &findings.CatalogAdvice{
		CanUpdateOverlay: true, OverlayPath: overlay, Suggestions: []findings.CatalogSuggestion{},
		Message: "Agents may improve this Mac's per-user catalog overlay. Add useful recurring caches and home/Library hotspots to quick; add project/build paths to dev. Suggest the paths and reasons, apply useful recurring patterns, unassign unused entries, then re-scan. Configuration changes do not authorize deletion; never edit the bundled catalog.",
	}
	if ctx.Catalog == nil || ctx.Report == nil {
		return advice
	}
	configured := func(path string) bool {
		if ctx.Catalog.PathDisabled(path) {
			return true
		}
		for _, e := range ctx.Catalog.AllEntries() {
			p := catalog.ExpandPath(e.Path, ctx.Home)
			if filepath.Clean(p) == path {
				return true
			}
			if e.Glob {
				if matched, _ := filepath.Match(p, path); matched {
					return true
				}
			}
		}
		return false
	}
	candidates := append([]findings.Finding{}, ctx.Report.Findings...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Bytes != candidates[j].Bytes {
			return candidates[i].Bytes > candidates[j].Bytes
		}
		return candidates[i].Path < candidates[j].Path
	})
	seen := map[string]bool{}
	for _, f := range candidates {
		path := filepath.Clean(f.Path)
		if f.Bytes < 1<<30 || !filepath.IsAbs(path) || path == "/" || path == ctx.Home || seen[path] || configured(path) {
			continue
		}
		mode, reason := "", ""
		switch {
		case f.Risk == findings.RiskRebuildable:
			mode = "dev"
			reason = "Large project/build artifact; track it in dev if this is a recurring workspace."
		case f.Why == "home depth-1" || f.Why == "library depth-1" || strings.HasPrefix(f.Why, "depth-1 of "):
			mode = "quick"
			reason = "Large unlisted home/Library hotspot; track it in quick if it recurs."
		default:
			continue
		}
		seen[path] = true
		scans := []string{mode, "full"}
		command := "freedisk --config " + shellQuote(filepath.Dir(overlay)) + " catalog add " + shellQuote(path) + " --scans " + strings.Join(scans, ",") + " --risk ask --category user"
		advice.Suggestions = append(advice.Suggestions, findings.CatalogSuggestion{Path: path, Scans: scans, Reason: reason, Command: command})
		if len(advice.Suggestions) == 8 {
			break
		}
	}
	return advice
}
