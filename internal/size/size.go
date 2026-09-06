package size

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Result is allocated vs apparent size for one path.
type Result struct {
	Allocated int64
	Apparent  int64
	Missing   bool
	Err       error
}

// Of returns allocated (st_blocks*512) and apparent (st_size) for path.
// Directories are walked in-process. Symlinks are not followed.
func Of(path string) Result {
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
	return dirSize(path)
}

func dirSize(root string) Result {
	var alloc, app int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		a, ap := fromInfo(info)
		alloc += a
		app += ap
		return nil
	})
	return Result{Allocated: alloc, Apparent: app, Err: err}
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
