package scan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

const KnownPhaseName = "known"

func noteUnreadable(rep *findings.Report, paths ...string) {
	if rep == nil {
		return
	}
	for _, p := range paths {
		if p != "" {
			rep.Unreadable = append(rep.Unreadable, p)
		}
	}
}

func seenKey(p string) string {
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return filepath.Clean(p)
}

func expandGlobs(e catalog.Entry) []string {
	p := e.Path
	if !e.Glob {
		return []string{p}
	}
	m, err := filepath.Glob(p)
	if err != nil || len(m) == 0 {
		return nil
	}
	const capN = 4096
	if len(m) > capN {
		return m[:capN]
	}
	return m
}

func entryFinding(e catalog.Entry, path string, sz size.Result) findings.Finding {
	risk := findings.Risk(e.Risk)
	if e.Category == "cargo-git" || strings.Contains(e.Path, ".cargo/git") || strings.Contains(e.Path, ".cargo/registry/src") {
		risk = findings.RiskAsk
	}
	f := findings.Finding{
		ID:       findings.IDSlug(e.Category, path),
		Path:     path,
		Bytes:    sz.Allocated,
		Category: e.Category,
		Risk:     risk,
		Why:      e.Note,
	}
	if e.Sparse && sz.Apparent > sz.Allocated {
		f.BytesApparent = sz.Apparent
	}
	if e.Reclaim != "" {
		f.Reclaim = &findings.Reclaim{Cmd: e.Reclaim}
	}
	return f
}

// Known sizes catalog paths for the current mode. Does not Register.
func Known(cat *catalog.Catalog, mode, home string, rep *findings.Report) {
	seen := map[string]struct{}{}
	for _, e := range cat.ModePaths(mode) {
		for _, raw := range expandGlobs(e) {
			p := catalog.ExpandPath(raw, home)
			if p == "" {
				continue
			}
			key := seenKey(p)
			if _, ok := seen[key]; ok {
				continue
			}
			var sz size.Result
			if e.AlwaysDrill || e.Category == "tmp" || e.Category == "tmp-user" {
				// Do not recurse; tmp children are a separate phase.
				fi, err := os.Lstat(p)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					if os.IsPermission(err) {
						rep.Unreadable = append(rep.Unreadable, p)
					}
					continue
				}
				sz = size.OfFileInfo(fi)
			} else {
				sz = size.Of(p)
			}
			noteUnreadable(rep, sz.Unreadable...)
			if sz.Missing {
				continue
			}
			if sz.Err != nil {
				if os.IsPermission(sz.Err) {
					noteUnreadable(rep, p)
				}
				continue
			}
			seen[key] = struct{}{}
			rep.Findings = append(rep.Findings, entryFinding(e, p, sz))
		}
	}
}
