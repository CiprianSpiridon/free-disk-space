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
				emitWorktreeDir(ctx, p, m.Kind, idle, inflight)
			}
			continue
		}
	}
	// walk work roots for suffix markers
	for _, e := range ctx.Catalog.WorkRoots {
		root := catalog.ExpandPath(e.Path, ctx.Home)
		if root == "" {
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			slash := filepath.ToSlash(p)
			for _, m := range ctx.Catalog.WorktreeMarkers {
				if m.PathSuffix != "" && strings.HasSuffix(slash, m.PathSuffix) {
					found = true
					emitWorktreeDir(ctx, p, m.Kind, idle, inflight)
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

func emitWorktreeDir(ctx *Context, p, kind string, idleDays, inflightHours int) {
	ents, err := os.ReadDir(p)
	n := 0
	if err == nil {
		for _, e := range ents {
			if e.IsDir() {
				n++
			}
		}
	}
	sz := size.Of(p)
	fi, _ := os.Lstat(p)
	risk := findings.RiskAsk
	why := kind + " worktrees"
	cmd := "git worktree prune --expire 1.day"
	if fi != nil {
		age := time.Since(fi.ModTime())
		if age > time.Duration(idleDays)*24*time.Hour {
			risk = findings.RiskLeftoverWorktree
			why = "idle worktree parent"
		} else if age < time.Duration(inflightHours)*time.Hour {
			risk = findings.RiskAsk
			why = "in-flight or recent worktrees"
		}
	}
	if kind == "git-metadata" {
		cmd = "git worktree prune --expire 1.day"
	}
	ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
		ID: findings.IDSlug("worktree-"+kind, p), Path: p, Bytes: sz.Allocated,
		Category: "worktree-" + kind, Risk: risk, Why: why, Count: n,
		Reclaim: &findings.Reclaim{Cmd: cmd},
	})
}
