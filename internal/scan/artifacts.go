package scan

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "artifacts", Quick: false, Dev: true, Run: runArtifacts})
}

func gitTracked(repo, path string, cache map[string]map[string]bool) bool {
	set, ok := cache[repo]
	if !ok {
		set = gitLsFiles(repo)
		cache[repo] = set
	}
	slash := filepath.ToSlash(path)
	return set[path] || set[slash]
}

func gitLsFiles(repo string) map[string]bool {
	out := map[string]bool{}
	cmd := exec.Command("git", "-C", repo, "ls-files", "-z")
	b, err := cmd.Output()
	if err != nil {
		return out
	}
	for _, p := range strings.Split(string(b), "\x00") {
		if p != "" {
			out[p] = true
			out[filepath.FromSlash(p)] = true
		}
	}
	return out
}

func findGitRoot(start string) string {
	dir := start
	for {
		if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			_ = st
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			return ""
		}
		dir = next
	}
}

func isComposerVendor(path string) bool {
	parent := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(parent, "composer.json")); err == nil {
		if strings.Contains(filepath.ToSlash(path), "resources/views/vendor") {
			return false
		}
		return true
	}
	return false
}

func hasCargoToml(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Cargo.toml"))
	return err == nil
}

func artifactMatch(name string, arts []catalog.Artifact) (catalog.Artifact, bool) {
	for _, a := range arts {
		if a.Name != "" && a.Name == name {
			return a, true
		}
	}
	return catalog.Artifact{}, false
}

func discoverWorkRoots(cat *catalog.Catalog, home string) []string {
	return collectWorkRoots(cat, home, false)
}

func artifactWalkRoots(cat *catalog.Catalog, home string) []string {
	return collectWorkRoots(cat, home, true)
}

func collectWorkRoots(cat *catalog.Catalog, home string, artifactsOnly bool) []string {
	var roots []string
	listed := map[string]struct{}{}
	for _, e := range cat.WorkRoots {
		p := catalog.ExpandPath(e.Path, home)
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			listed[filepath.Base(p)] = struct{}{}
			if artifactsOnly && !e.WalkArtifacts {
				continue
			}
			roots = append(roots, p)
		}
	}
	d := cat.WorkRootDiscover
	if !d.Enabled {
		return roots
	}
	skip := map[string]struct{}{}
	for _, n := range d.SkipNames {
		skip[n] = struct{}{}
	}
	ents, err := os.ReadDir(home)
	if err != nil {
		return roots
	}
	markers := d.ProjectMarkers
	minRepos := d.MinChildRepos
	if minRepos == 0 {
		minRepos = 2
	}
	for _, ent := range ents {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		if _, ok := skip[ent.Name()]; ok {
			continue
		}
		if _, ok := listed[ent.Name()]; ok {
			continue
		}
		p := filepath.Join(home, ent.Name())
		hit := false
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(p, m)); err == nil {
				hit = true
				break
			}
		}
		if !hit && minRepos > 0 {
			kids, _ := os.ReadDir(p)
			n := 0
			for _, k := range kids {
				if !k.IsDir() {
					continue
				}
				if _, err := os.Stat(filepath.Join(p, k.Name(), ".git")); err == nil {
					n++
				}
			}
			if n >= minRepos {
				hit = true
			}
		}
		if hit {
			roots = append(roots, p)
		}
	}
	return roots
}

func runArtifacts(ctx *Context) error {
	max := ctx.Catalog.Thresholds.MaxWalkDepth
	if max == 0 {
		max = 8
	}
	idle := ctx.Catalog.Thresholds.ArtifactIdleDays
	if idle == 0 {
		idle = 30
	}
	prune := map[string]struct{}{}
	for _, n := range ctx.Catalog.WalkPrune {
		prune[n] = struct{}{}
	}
	for _, a := range ctx.Catalog.Artifacts {
		if a.Name != "" {
			prune[a.Name] = struct{}{}
		}
	}
	roots := artifactWalkRoots(ctx.Catalog, ctx.Home)
	seen := map[string]struct{}{}
	tracked := map[string]map[string]bool{}
	for _, root := range roots {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					noteUnreadable(ctx.Report, p)
				}
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			depth := 0
			if rel != "." {
				depth = strings.Count(rel, string(os.PathSeparator)) + 1
			}
			if depth > max {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() && name == ".git" {
				return filepath.SkipDir
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() && name == "vendor" {
				if isComposerVendor(p) {
					emitArtifact(ctx, p, "composer-vendor", findings.RiskRebuildable, idle, seen, tracked)
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() && name == "target" && hasCargoToml(filepath.Dir(p)) {
				emitArtifact(ctx, p, "rust-target", findings.RiskRebuildable, idle, seen, tracked)
				return filepath.SkipDir
			}
			if a, ok := artifactMatch(name, ctx.Catalog.Artifacts); ok {
				if a.File && d.IsDir() {
					return nil
				}
				if name == "env" {
					if _, err := os.Stat(filepath.Join(p, "pyvenv.cfg")); err != nil {
						return nil
					}
				}
				if d.IsDir() {
					emitArtifact(ctx, p, a.Ecosystem, findings.Risk(a.Risk), idle, seen, tracked)
					if _, skip := prune[name]; skip {
						return filepath.SkipDir
					}
				} else if a.File {
					emitArtifact(ctx, p, a.Ecosystem, findings.Risk(a.Risk), idle, seen, tracked)
				}
			}
			if d.IsDir() {
				if _, err := os.Stat(filepath.Join(p, "pyvenv.cfg")); err == nil {
					emitArtifact(ctx, p, "python-venv", findings.RiskRebuildable, idle, seen, tracked)
					return filepath.SkipDir
				}
			}
			return nil
		})
	}
	return nil
}

func emitArtifact(ctx *Context, p, cat string, risk findings.Risk, idleDays int, seen map[string]struct{}, tracked map[string]map[string]bool) {
	if _, ok := seen[p]; ok {
		return
	}
	if repo := findGitRoot(filepath.Dir(p)); repo != "" {
		rel, err := filepath.Rel(repo, p)
		if err == nil && gitTracked(repo, rel, tracked) {
			risk = findings.RiskKeep
		}
	}
	sz := size.Of(p)
	noteUnreadable(ctx.Report, sz.Unreadable...)
	if sz.Missing {
		return
	}
	seen[p] = struct{}{}
	last := ""
	why := cat
	if fi, err := os.Lstat(p); err == nil {
		last = fi.ModTime().UTC().Format("2006-01-02")
		days := int(time.Since(fi.ModTime()).Hours() / 24)
		if days >= idleDays {
			why = "untouched " + last
		}
	}
	cmd := "rm -rf " + p
	if cat == "rust-target" {
		cmd = "cargo clean"
	}
	f := findings.Finding{
		ID: findings.IDSlug(cat, p), Path: p, Bytes: sz.Allocated,
		Category: cat, Risk: risk, LastUsed: last, Why: why,
		Reclaim: &findings.Reclaim{Cmd: cmd},
	}
	if risk == findings.RiskKeep {
		f.Reclaim = nil
	}
	ctx.Report.Findings = append(ctx.Report.Findings, f)
}
