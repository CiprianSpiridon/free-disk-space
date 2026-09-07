package size

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Result is allocated vs apparent size for one path.
type Result struct {
	Allocated  int64
	Apparent   int64
	Missing    bool
	Err        error
	Unreadable []string
}

// Of returns allocated (st_blocks*512) and apparent (st_size) for path.
// Directories are walked in-process. Symlinks are not followed.
func Of(path string) Result {
	return of(context.Background(), path, nil)
}

// OfContext is Of with a cancel/deadline. On cancel the walk stops and
// Allocated is not returned (incomplete).
func OfContext(ctx context.Context, path string) Result {
	return of(ctx, path, nil)
}

// OfSkipping is Of, but does not descend into skip paths (already sized children).
func OfSkipping(path string, skip []string) Result {
	return of(context.Background(), path, skip)
}

func of(ctx context.Context, path string, skip []string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Result{Missing: true}
		}
		return Result{Err: err}
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		a, p := fromInfo(fi)
		return Result{Allocated: a, Apparent: p}
	}
	if !fi.IsDir() {
		a, p := fromInfo(fi)
		return Result{Allocated: a, Apparent: p}
	}
	return dirSize(ctx, path, skip)
}

func skipListed(p string, skip []string) bool {
	for _, s := range skip {
		if s == "" {
			continue
		}
		s = filepath.Clean(s)
		if p == s || strings.HasPrefix(p, s+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

var extraSkip []string

// SetSkipPrefixes adds catalog skip_system_prefixes to the walk denylist.
func SetSkipPrefixes(p []string) {
	extraSkip = append([]string{}, p...)
}

func skipWalk(p string) bool {
	if strings.HasPrefix(p, "/usr/local") {
		return false
	}
	prefs := []string{"/System", "/usr", "/bin", "/sbin", "/private/var/vm", "/dev", "/net"}
	prefs = append(prefs, extraSkip...)
	for _, pre := range prefs {
		if pre == "" {
			continue
		}
		if p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}

const sfDataless = 0x40000000

var (
	hbMu      sync.Mutex
	heartbeat func(root, current string, visited int, elapsed time.Duration)
	hbEvery   = 2 * time.Second
)

// SetHeartbeat is called about every 2s during a directory walk. nil clears.
func SetHeartbeat(fn func(root, current string, visited int, elapsed time.Duration)) {
	hbMu.Lock()
	heartbeat = fn
	hbMu.Unlock()
}

func setHeartbeatEvery(d time.Duration) {
	hbMu.Lock()
	if d <= 0 {
		hbEvery = 2 * time.Second
	} else {
		hbEvery = d
	}
	hbMu.Unlock()
}

func dirSize(ctx context.Context, root string, skip []string) Result {
	var alloc, app int64
	var unread []string
	seenIno := map[[2]uint64]struct{}{}
	t0 := time.Now()
	var lastHB time.Time
	visited := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if p != root && skipListed(p, skip) {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		visited++
		now := time.Now()
		hbMu.Lock()
		fn := heartbeat
		every := hbEvery
		hbMu.Unlock()
		if fn != nil {
			if lastHB.IsZero() {
				lastHB = now
			} else if now.Sub(lastHB) >= every {
				fn(root, p, visited, now.Sub(t0))
				lastHB = now
			}
		}
		if err != nil {
			if os.IsPermission(err) || errors.Is(err, fs.ErrPermission) {
				unread = append(unread, p)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
			}
			return nil
		}
		if skipWalk(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsPermission(err) {
				unread = append(unread, p)
			}
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			if st.Flags&sfDataless != 0 {
				return nil
			}
			k := [2]uint64{uint64(st.Dev), uint64(st.Ino)}
			if _, dup := seenIno[k]; dup {
				return nil
			}
			seenIno[k] = struct{}{}
		}
		a, ap := fromInfo(info)
		alloc += a
		app += ap
		return nil
	})
	if ctx.Err() != nil {
		return Result{Err: ctx.Err(), Unreadable: unread}
	}
	return Result{Allocated: alloc, Apparent: app, Err: err, Unreadable: unread}
}

// OfFileInfo sizes a single inode (no walk).
func OfFileInfo(fi os.FileInfo) Result {
	a, p := fromInfo(fi)
	return Result{Allocated: a, Apparent: p}
}

func fromInfo(fi os.FileInfo) (allocated, apparent int64) {
	apparent = fi.Size()
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return apparent, apparent
	}
	return st.Blocks * 512, st.Size
}

// Children lists depth-1 names under dir (no recursion, no symlink follow).
func Children(dir string) ([]os.DirEntry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	return ents, nil
}
