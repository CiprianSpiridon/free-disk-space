package size

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type entryMetadata struct {
	name                string
	kind                uint32 // Darwin VREG=1, VDIR=2, VLNK=5
	device, inode       uint64
	links, flags        uint32
	allocated, apparent int64
	modified            time.Time
	mount               bool
	private             int64
	privateKnown        bool
	err                 error
}

func metadata(fi os.FileInfo) entryMetadata {
	a, b := fromInfo(fi)
	e := entryMetadata{name: fi.Name(), kind: 1, allocated: a, apparent: b, modified: fi.ModTime()}
	if fi.IsDir() {
		e.kind = 2
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		e.kind = 5
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		e.device = uint64(st.Dev)
		e.inode = uint64(st.Ino)
		e.links = uint32(st.Nlink)
		e.flags = st.Flags
	}
	return e
}

func readStatDir(path string) ([]entryMetadata, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]entryMetadata, 0, len(entries))
	for _, e := range entries {
		fi, err := os.Lstat(filepath.Join(path, e.Name()))
		if err != nil {
			out = append(out, entryMetadata{name: e.Name(), err: err})
			continue
		}
		out = append(out, metadata(fi))
	}
	return out, nil
}
