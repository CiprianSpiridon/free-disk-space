package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "worktrees", Quick: false, Dev: true, Run: runWorktrees})
}

func runWorktrees(ctx *Context) error {
	idle := ctx.Catalog.Thresholds.WorktreeIdleDays
	if idle == 0 {
		idle = 14
	}
	inflight := ctx.Catalog.Thresholds.WorktreeInflightHours
	if inflight == 0 {
		inflight = 24
	}
	found := false
	for _, m := range ctx.Catalog.WorktreeMarkers {
		p := m.Path
		if p != "" {
			p = catalog.ExpandPath(p, ctx.Home)
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				found = true
				emitWorktreeChildren(ctx, p, m.Kind, idle, inflight)
			}
		}
	}
	max := ctx.Catalog.Thresholds.MaxWalkDepth
	if max == 0 {
		max = 8
	}
	prune := map[string]struct{}{}
	for _, a := range ctx.Catalog.Artifacts {
		if a.Name != "" {
			prune[a.Name] = struct{}{}
		}
	}
	for _, root := range discoverWorkRoots(ctx.Catalog, ctx.Home) {
		root = filepath.Clean(root)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(root, p)
			if err == nil && rel != "." && max > 0 {
				depth := 1 + strings.Count(rel, string(os.PathSeparator))
				if depth > max {
					return filepath.SkipDir
				}
			}
			name := d.Name()
			if _, ok := prune[name]; ok {
				return filepath.SkipDir
			}
			slash := filepath.ToSlash(p)
			if strings.Contains(slash, "/.git/") && !strings.Contains(slash, "/.git/worktrees") {
				return filepath.SkipDir
			}
			for _, m := range ctx.Catalog.WorktreeMarkers {
				if m.PathSuffix != "" && strings.HasSuffix(slash, m.PathSuffix) {
					found = true
					emitWorktreeChildren(ctx, p, m.Kind, idle, inflight)
					return filepath.SkipDir
				}
			}
			return nil
		})
	}
	if !found {
		if _, err := os.Stat(catalog.ExpandPath("~/.grok/worktrees", ctx.Home)); err != nil {
			ctx.Report.NotPresent = append(ctx.Report.NotPresent, "grok-worktrees")
		}
	}
	return nil
}

func emitWorktreeChildren(ctx *Context, p, kind string, idleDays, inflightHours int) {
	ents, err := os.ReadDir(p)
	var kids []os.DirEntry
	if err == nil {
		for _, e := range ents {
			if e.IsDir() && e.Type()&os.ModeSymlink == 0 {
				kids = append(kids, e)
			}
		}
	}
	if len(kids) == 0 {
		emitOneWorktree(ctx, p, kind, idleDays, inflightHours, 0)
		return
	}
	switch kind {
	case "grok", "codex", "claude", "ulpi":
		for _, e := range kids {
			emitOneWorktree(ctx, filepath.Join(p, e.Name()), kind, idleDays, inflightHours, 0)
		}
	default:
		emitOneWorktree(ctx, p, kind, idleDays, inflightHours, len(kids))
	}
}

func newestMtime(p string) time.Time {
	fi, err := os.Lstat(p)
	if err != nil {
		return time.Time{}
	}
	newest := fi.ModTime()
	ents, err := os.ReadDir(p)
	if err != nil {
		return newest
	}
	for _, e := range ents {
		if !e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := filepath.Join(p, e.Name())
		cfi, err := e.Info()
		if err == nil && cfi.ModTime().After(newest) {
			newest = cfi.ModTime()
		}
		gcs, _ := os.ReadDir(child)
		for _, g := range gcs {
			if !g.IsDir() || g.Type()&os.ModeSymlink != 0 {
				continue
			}
			gi, err := g.Info()
			if err == nil && gi.ModTime().After(newest) {
				newest = gi.ModTime()
			}
		}
	}
	return newest
}

func emitOneWorktree(ctx *Context, p, kind string, idleDays, inflightHours, count int) {
	if count == 0 {
		ents, err := os.ReadDir(p)
		if err == nil {
			for _, e := range ents {
				if e.IsDir() {
					count++
				}
			}
		}
	}
	sz := size.Of(p)
	newest := newestMtime(p)
	risk := findings.RiskAsk
	why := kind + " worktrees"
	if !newest.IsZero() {
		age := time.Since(newest)
		if age < time.Duration(inflightHours)*time.Hour {
			risk = findings.RiskAsk
			why = "in-flight or recent worktrees"
		} else if age > time.Duration(idleDays)*24*time.Hour {
			risk = findings.RiskLeftoverWorktree
			why = "idle worktree"
		}
	}
	cmd := "git worktree prune --expire 1.day"
	ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
		ID: findings.IDSlug("worktree-"+kind, p), Path: p, Bytes: sz.Allocated,
		Category: "worktree-" + kind, Risk: risk, Why: why, Count: count,
		Reclaim: &findings.Reclaim{Cmd: cmd},
	})
}
