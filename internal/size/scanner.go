package size

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scanner belongs to one scan. Directory aggregates let subsequent phases
// reuse descendants without persistent caches or competing disk walks.
type Scanner struct {
	cache             map[string]Result
	Visits            int
	started, lastBeat time.Time
	pinFiles          []string
	readEntries       func(string) ([]entryMetadata, error)
	BulkFallbacks     int
	Workers           int
	base              map[string]Result
	progressMu        *sync.Mutex
}

const DefaultWorkers = 5

func NewScanner() *Scanner {
	return &Scanner{cache: map[string]Result{}, started: time.Now(), readEntries: readBulkDir, Workers: DefaultWorkers, progressMu: &sync.Mutex{}}
}
func (s *Scanner) Of(path string) Result { return s.OfContext(context.Background(), path) }
func (s *Scanner) OfContext(ctx context.Context, path string) Result {
	return s.walk(ctx, filepath.Clean(path), path, 0, nil)
}

func (s *Scanner) walk(ctx context.Context, p, root string, rootDev uint64, supplied *entryMetadata) Result {
	if err := ctx.Err(); err != nil {
		return Result{Err: err}
	}
	if r, ok := s.cache[p]; ok {
		if rootDev != 0 && r.device != rootDev {
			return Result{SkippedMounts: []string{p}}
		}
		return r
	}
	if r, ok := s.base[p]; ok {
		if rootDev != 0 && r.device != rootDev {
			return Result{SkippedMounts: []string{p}}
		}
		return r
	}
	if skipWalk(p) {
		return Result{}
	}
	s.Visits++
	hbMu.Lock()
	fn, every := heartbeat, hbEvery
	hbMu.Unlock()
	if fn != nil && time.Since(s.lastBeat) >= every {
		s.progressMu.Lock()
		fn(root, p, s.Visits, time.Since(s.started))
		s.progressMu.Unlock()
		s.lastBeat = time.Now()
	}
	var e entryMetadata
	if supplied != nil {
		e = *supplied
	} else {
		fi, err := os.Lstat(p)
		if err != nil {
			e.err = err
		} else {
			e = metadata(fi)
			if e.kind != 2 || e.allocated != 0 {
				e.private, e.privateKnown = privateSize(p)
			}
		}
	}
	if e.err != nil {
		if os.IsNotExist(e.err) {
			return Result{Missing: true}
		}
		r := Result{Err: e.err}
		if os.IsPermission(e.err) {
			r.Unreadable = []string{p}
		}
		return r
	}
	if rootDev == 0 {
		rootDev = e.device
	} else if e.kind == 2 && (e.mount || e.device != rootDev) {
		return Result{Newest: e.modified, SkippedMounts: []string{p}}
	}
	r := Result{Allocated: e.allocated, Apparent: e.apparent, Newest: e.modified, Private: e.private, PrivateKnown: e.privateKnown, device: e.device}
	if e.kind == 2 && e.allocated == 0 {
		r.PrivateKnown = true
	}
	if e.flags&sfDataless != 0 {
		return Result{Newest: e.modified, PrivateKnown: true}
	}
	if e.kind == 5 {
		return r
	}
	if e.kind != 2 {
		if e.links > 1 {
			r.links = map[[2]uint64][2]int64{{e.device, e.inode}: {r.Allocated, r.Apparent}}
			r.Private = 0
			r.PrivateKnown = true
		}
		if e.name == "rust-toolchain" || e.name == "rust-toolchain.toml" {
			s.pinFiles = append(s.pinFiles, p)
		}
		return r
	}
	entries, err := s.readEntries(p)
	if err != nil {
		s.BulkFallbacks++
		entries, err = readStatDir(p)
	}
	if err != nil {
		r.Err = err
		r.PrivateKnown = false
		if os.IsPermission(err) {
			r.Unreadable = []string{p}
		}
		return r
	}

	var children []Result
	uncachedDirs := 0
	if p == root && s.Workers > 1 {
		for _, e := range entries {
			if e.kind == 2 {
				if _, ok := s.cache[filepath.Join(p, e.name)]; !ok {
					uncachedDirs++
				}
			}
		}
	}
	if uncachedDirs >= 2 {
		children = s.parallelChildren(ctx, p, root, rootDev, entries)
	}
	for i, e := range entries {
		if e.kind == 5 {
			continue
		}
		var c Result
		if children != nil {
			c = children[i]
		} else {
			c = s.walk(ctx, filepath.Join(p, e.name), root, rootDev, &e)
		}
		if ctx.Err() != nil {
			return Result{Err: ctx.Err(), Unreadable: r.Unreadable}
		}
		r.Allocated += c.Allocated
		r.Apparent += c.Apparent
		r.Private += c.Private
		r.PrivateKnown = r.PrivateKnown && c.PrivateKnown && c.Err == nil && len(c.Unreadable) == 0
		r.Unreadable = append(r.Unreadable, c.Unreadable...)
		r.SkippedMounts = append(r.SkippedMounts, c.SkippedMounts...)
		if c.Newest.After(r.Newest) {
			r.Newest = c.Newest
		}
		for inode, sz := range c.links {
			if r.links == nil {
				r.links = map[[2]uint64][2]int64{}
			}
			if _, ok := r.links[inode]; ok {
				r.Allocated -= sz[0]
				r.Apparent -= sz[1]
			} else {
				r.links[inode] = sz
			}
		}
	}
	s.cache[p] = r
	return r
}

// Workers own disjoint child trees and merge only after all reads finish. The
// parent cache is immutable during the read, so already-sized branches are reused.
func (s *Scanner) parallelChildren(ctx context.Context, p, root string, dev uint64, entries []entryMetadata) []Result {
	count := s.Workers
	if count > len(entries) {
		count = len(entries)
	}
	workers := make([]*Scanner, count)
	results := make([]Result, len(entries))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := range workers {
		w := NewScanner()
		w.Workers = 1
		w.base = s.cache
		w.readEntries = s.readEntries
		w.progressMu = s.progressMu
		workers[i] = w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				e := entries[j]
				if e.kind != 5 {
					results[j] = w.walk(ctx, filepath.Join(p, e.name), root, dev, &e)
				}
			}
		}()
	}
	for j := range entries {
		// Bulk metadata already contains ordinary file sizes. Reserve worker
		// dispatch for directory I/O rather than queueing every cached file.
		if entries[j].kind == 2 {
			jobs <- j
		} else if entries[j].kind != 5 {
			results[j] = s.walk(ctx, filepath.Join(p, entries[j].name), root, dev, &entries[j])
		}
	}
	close(jobs)
	wg.Wait()
	for _, w := range workers {
		for path, r := range w.cache {
			s.cache[path] = r
		}
		s.Visits += w.Visits
		s.BulkFallbacks += w.BulkFallbacks
		s.pinFiles = append(s.pinFiles, w.pinFiles...)
	}
	return results
}

func (s *Scanner) PinFiles(roots []string) ([]string, bool) {
	var files []string
	for _, root := range roots {
		r, ok := s.cache[filepath.Clean(root)]
		if !ok || r.Err != nil || len(r.Unreadable) > 0 || len(r.SkippedMounts) > 0 {
			return nil, false
		}
		for _, p := range s.pinFiles {
			rel, err := filepath.Rel(root, p)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				files = append(files, p)
			}
		}
	}
	return files, true
}
