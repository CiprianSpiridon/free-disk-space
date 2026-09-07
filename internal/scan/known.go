package scan

import (
	"os"
	"path/filepath"
	"sort"
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

func pathDepth(p string) int {
	p = filepath.Clean(p)
	if p == "/" || p == "" {
		return 0
	}
	n := 0
	for _, c := range p {
		if c == '/' {
			n++
		}
	}
	return n
}

type sizedTree struct {
	path      string
	allocated int64
	apparent  int64
}

func nestedUnder(p string, trees []string) bool {
	clean := filepath.Clean(p)
	for _, t := range trees {
		if t != "" && clean != t && (strings.HasPrefix(clean, t+string(os.PathSeparator)) || strings.HasPrefix(clean, t+"/")) {
			return true
		}
	}
	return false
}

func skipUnder(parent string, sized []sizedTree) []string {
	parent = filepath.Clean(parent)
	var skip []string
	for _, s := range sized {
		if nestedUnder(s.path, []string{parent}) {
			skip = append(skip, s.path)
		}
	}
	return skip
}

func addSizedChildren(sz size.Result, parent string, sized []sizedTree) size.Result {
	parent = filepath.Clean(parent)
	for i, s := range sized {
		if !nestedUnder(s.path, []string{parent}) {
			continue
		}
		covered := false
		for j, o := range sized {
			if i == j {
				continue
			}
			if nestedUnder(s.path, []string{o.path}) && nestedUnder(o.path, []string{parent}) {
				covered = true
				break
			}
		}
		if !covered {
			sz.Allocated += s.allocated
			sz.Apparent += s.apparent
		}
	}
	return sz
}

func lookupSized(key string, sized []sizedTree) (sizedTree, bool) {
	for _, s := range sized {
		if s.path == key {
			return s, true
		}
	}
	return sizedTree{}, false
}

func catalogInodeOnly(e catalog.Entry) bool {
	if e.AlwaysDrill || e.Category == "tmp" || e.Category == "tmp-user" {
		return true
	}
	if e.GlobChildren != "" {
		return true
	}
	return e.Category == "library-root"
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
	if fi, err := os.Lstat(path); err == nil {
		f.LastUsed = fi.ModTime().UTC().Format("2006-01-02")
	}
	return f
}

func lastUsedOf(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format("2006-01-02")
}

func reportMinBytes(cat *catalog.Catalog) int64 {
	if cat != nil && cat.Thresholds.ReportBytes > 0 {
		return cat.Thresholds.ReportBytes
	}
	return 50 << 20
}

type catalogJob struct {
	e    catalog.Entry
	path string
}

// Known sizes catalog paths for the current mode. Does not Register.
func Known(ctx *Context) {
	if ctx == nil || ctx.Catalog == nil || ctx.Report == nil {
		return
	}
	cat := ctx.Catalog
	mode := ctx.Mode
	home := ctx.Home
	var jobs []catalogJob
	for _, e := range cat.ModePaths(mode) {
		for _, p := range catalog.ExpandEntry(e, home) {
			if p == "" {
				continue
			}
			jobs = append(jobs, catalogJob{e: e, path: p})
		}
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		di, dj := pathDepth(jobs[i].path), pathDepth(jobs[j].path)
		if di != dj {
			return di > dj
		}
		if len(jobs[i].path) != len(jobs[j].path) {
			return len(jobs[i].path) > len(jobs[j].path)
		}
		return jobs[i].path < jobs[j].path
	})
	seen := map[string]struct{}{}
	var sized []sizedTree
	minChild := reportMinBytes(cat)
	for _, j := range jobs {
		sized = sizeOne(ctx, j.e, j.path, seen, sized, minChild)
	}
	homeDepth1(ctx, seen, sized, minChild)
}

func sizeOne(ctx *Context, e catalog.Entry, p string, seen map[string]struct{}, sized []sizedTree, minChild int64) []sizedTree {
	rep := ctx.Report
	key := seenKey(p)
	if _, ok := seen[key]; ok {
		return sized
	}
	var sz size.Result
	switch {
	case catalogInodeOnly(e):
		ctx.logf("listing %s", p)
		fi, err := os.Lstat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return sized
			}
			if os.IsPermission(err) {
				rep.Unreadable = append(rep.Unreadable, p)
			}
			return sized
		}
		sz = size.OfFileInfo(fi)
	case e.Drill:
		ctx.logf("depth-1 %s", p)
		var kids []findings.Finding
		sz, kids, sized = depth1(ctx, p, e, seen, sized, minChild)
		rep.Findings = append(rep.Findings, kids...)
	default:
		ctx.logf("sizing %s", p)
		sz = size.OfSkipping(p, skipUnder(p, sized))
		sz = addSizedChildren(sz, p, sized)
	}
	noteUnreadable(rep, sz.Unreadable...)
	if sz.Missing {
		return sized
	}
	if sz.Err != nil {
		if os.IsPermission(sz.Err) {
			noteUnreadable(rep, p)
		}
		return sized
	}
	seen[key] = struct{}{}
	if !catalogInodeOnly(e) {
		sized = append(sized, sizedTree{path: key, allocated: sz.Allocated, apparent: sz.Apparent})
	}
	rep.Findings = append(rep.Findings, entryFinding(e, p, sz))
	if e.GlobChildren != "" {
		emitGlobChildren(e, p, seen, rep)
	}
	return sized
}

func depth1(ctx *Context, dir string, parent catalog.Entry, seen map[string]struct{}, sized []sizedTree, minChild int64) (size.Result, []findings.Finding, []sizedTree) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsPermission(err) {
			ctx.Report.Unreadable = append(ctx.Report.Unreadable, dir)
		}
		if os.IsNotExist(err) {
			return size.Result{Missing: true}, nil, sized
		}
		return size.Result{Err: err}, nil, sized
	}
	var alloc, app int64
	var unread []string
	var kids []findings.Finding
	for _, ent := range ents {
		if ent.Type()&os.ModeSymlink != 0 {
			continue
		}
		cp := filepath.Join(dir, ent.Name())
		key := seenKey(cp)
		var child size.Result
		if t, ok := lookupSized(key, sized); ok {
			child = size.Result{Allocated: t.allocated, Apparent: t.apparent}
		} else if _, ok := seen[key]; ok {
			continue
		} else {
			info, err := ent.Info()
			if err != nil {
				continue
			}
			if !info.IsDir() {
				child = size.OfFileInfo(info)
			} else {
				child = size.OfSkipping(cp, skipUnder(cp, sized))
				child = addSizedChildren(child, cp, sized)
			}
			noteUnreadable(ctx.Report, child.Unreadable...)
			unread = append(unread, child.Unreadable...)
			if child.Missing || child.Err != nil {
				continue
			}
			seen[key] = struct{}{}
			sized = append(sized, sizedTree{path: key, allocated: child.Allocated, apparent: child.Apparent})
			if child.Allocated >= minChild {
				kids = append(kids, findings.Finding{
					ID:       findings.IDSlug(parent.Category+"-child", cp),
					Path:     cp,
					Bytes:    child.Allocated,
					Category: parent.Category,
					Risk:     findings.RiskAsk,
					LastUsed: lastUsedOf(cp),
					Why:      "depth-1 of " + dir,
					Reclaim:  &findings.Reclaim{Cmd: rmRf(cp)},
				})
			}
		}
		alloc += child.Allocated
		app += child.Apparent
	}
	return size.Result{Allocated: alloc, Apparent: app, Unreadable: unread}, kids, sized
}

func homeDepth1(ctx *Context, seen map[string]struct{}, sized []sizedTree, min int64) {
	if ctx.Home == "" {
		return
	}
	ctx.logf("home depth-1 %s", ctx.Home)
	emitDepth1Unknown(ctx, ctx.Home, seen, sized, min, true, "home")
	lib := filepath.Join(ctx.Home, "Library")
	if st, err := os.Stat(lib); err == nil && st.IsDir() {
		ctx.logf("library depth-1 %s", lib)
		emitDepth1Unknown(ctx, lib, seen, sized, min, false, "library")
	}
}

func emitDepth1Unknown(ctx *Context, dir string, seen map[string]struct{}, sized []sizedTree, min int64, hidden bool, category string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsPermission(err) {
			ctx.Report.Unreadable = append(ctx.Report.Unreadable, dir)
		}
		return
	}
	for _, ent := range ents {
		name := ent.Name()
		if name == "." || name == ".." {
			continue
		}
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if ent.Type()&os.ModeSymlink != 0 {
			continue
		}
		p := filepath.Join(dir, name)
		key := seenKey(p)
		if _, ok := seen[key]; ok {
			continue
		}
		ctx.logf("sizing %s", p)
		sz := size.OfSkipping(p, skipUnder(p, sized))
		sz = addSizedChildren(sz, p, sized)
		noteUnreadable(ctx.Report, sz.Unreadable...)
		if sz.Missing || sz.Err != nil || sz.Allocated < min {
			continue
		}
		seen[key] = struct{}{}
		risk := findings.RiskAsk
		why := "home depth-1"
		if category == "home" && !strings.HasPrefix(name, ".") {
			risk = findings.RiskKeep
		}
		if category == "library" {
			why = "library depth-1"
		}
		ctx.Report.Findings = append(ctx.Report.Findings, findings.Finding{
			ID:       findings.IDSlug(category, p),
			Path:     p,
			Bytes:    sz.Allocated,
			Category: category,
			Risk:     risk,
			LastUsed: lastUsedOf(p),
			Why:      why,
			Reclaim:  &findings.Reclaim{Cmd: rmRf(p)},
		})
	}
}

func emitGlobChildren(e catalog.Entry, dir string, seen map[string]struct{}, rep *findings.Report) {
	matches, err := filepath.Glob(filepath.Join(dir, e.GlobChildren))
	if err != nil {
		return
	}
	n := 0
	for _, m := range matches {
		if n >= 4096 {
			break
		}
		key := seenKey(m)
		if _, ok := seen[key]; ok {
			continue
		}
		fi, err := os.Lstat(m)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		sz := size.Of(m)
		noteUnreadable(rep, sz.Unreadable...)
		if sz.Missing || sz.Err != nil {
			continue
		}
		seen[key] = struct{}{}
		child := e
		child.GlobChildren = ""
		rep.Findings = append(rep.Findings, entryFinding(child, m, sz))
		n++
	}
}
