package size

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Layout and flags come from the macOS SDK sys/attr.h and getattrlistbulk(2).
// PACK_INVAL_ATTRS keeps offsets fixed; returned masks decide validity. No cgo
// is required, including in the arm64/amd64 release cross-builds.
type attrList struct {
	Count, Reserved                 uint16
	Common, Volume, Dir, File, Fork uint32
}

const bulkCommon uint32 = 0x80000000 | 0x20000000 | 0x1 | 0x2 | 0x8 | 0x400 | 0x40000 | 0x02000000
const bulkDir uint32 = 0x4 | 0x8 | 0x20
const bulkFile uint32 = 0x1 | 0x4 | 0x200

var bulkBuffers = sync.Pool{New: func() any { return make([]byte, 64<<10) }}

func readBulkDir(path string) ([]entryMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	attrs := attrList{Count: 5, Common: bulkCommon, Dir: bulkDir, File: bulkFile, Fork: 0x8}
	buf := bulkBuffers.Get().([]byte)
	defer bulkBuffers.Put(buf)
	var entries []entryMetadata
	for {
		count, _, errno := syscall.Syscall6(461, f.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0x8|0x20, 0)
		runtime.KeepAlive(f)
		runtime.KeepAlive(&attrs)
		runtime.KeepAlive(buf)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			return nil, errno
		}
		if count == 0 {
			return entries, nil
		}
		offset := 0
		for i := uintptr(0); i < count; i++ {
			if offset+4 > len(buf) {
				return nil, fmt.Errorf("bulk metadata: truncated record")
			}
			length := int(binary.LittleEndian.Uint32(buf[offset:]))
			if length < 72 || length > len(buf)-offset {
				return nil, fmt.Errorf("bulk metadata: invalid record length %d", length)
			}
			record := buf[offset : offset+length]
			offset += length
			e, err := parseBulkEntry(record)
			if err != nil {
				return nil, err
			}
			// File systems may omit fields; stat just those entries rather than guess.
			if e.kind == 0 {
				fi, err := os.Lstat(filepath.Join(path, e.name))
				if err != nil {
					e.err = err
				} else {
					e = metadata(fi)
				}
			}
			entries = append(entries, e)
		}
	}
}

func parseBulkEntry(b []byte) (entryMetadata, error) {
	if len(b) < 72 {
		return entryMetadata{}, fmt.Errorf("bulk metadata: truncated fields")
	}
	le := binary.LittleEndian
	common, dir, file, fork := le.Uint32(b[4:]), le.Uint32(b[12:]), le.Uint32(b[16:]), le.Uint32(b[20:])
	if errno := le.Uint32(b[24:]); errno != 0 {
		return entryMetadata{}, syscall.Errno(errno)
	}
	start := 28 + int(int32(le.Uint32(b[28:])))
	n := int(le.Uint32(b[32:]))
	if start < 72 || n < 2 || n > len(b)-start || b[start+n-1] != 0 {
		return entryMetadata{}, fmt.Errorf("bulk metadata: invalid name reference")
	}
	e := entryMetadata{name: string(b[start : start+n-1])}
	if filepath.Base(e.name) != e.name || e.name == "." || e.name == ".." {
		return e, fmt.Errorf("bulk metadata: invalid entry name")
	}
	required := bulkCommon &^ (0x80000000 | 0x20000000)
	if common&required != required {
		return e, nil
	}
	e.device = uint64(le.Uint32(b[36:]))
	e.kind = le.Uint32(b[40:])
	e.modified = time.Unix(int64(le.Uint64(b[44:])), int64(le.Uint64(b[52:])))
	e.flags = le.Uint32(b[60:])
	e.inode = le.Uint64(b[64:])
	// File and directory groups are mutually exclusive even with PACK_INVAL_ATTRS.
	if start < 100 {
		e.kind = 0
		return e, nil
	}
	if e.kind == 2 {
		if dir&bulkDir != bulkDir {
			e.kind = 0
			return e, nil
		}
		e.mount = le.Uint32(b[72:])&1 != 0
		e.allocated = int64(le.Uint64(b[76:]))
		e.apparent = int64(le.Uint64(b[84:]))
	} else {
		if file&bulkFile != bulkFile {
			e.kind = 0
			return e, nil
		}
		e.links = le.Uint32(b[72:])
		e.allocated = int64(le.Uint64(b[76:]))
		e.apparent = int64(le.Uint64(b[84:]))
	}
	if fork&0x8 != 0 {
		e.private = int64(le.Uint64(b[92:]))
		e.privateKnown = e.private >= 0
	}

	return e, nil
}

func privateSize(path string) (int64, bool) {
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return 0, false
	}
	attrs := attrList{Count: 5, Common: 0x80000000, Fork: 0x8}
	var buf [32]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_GETATTRLIST, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0x1|0x8|0x20, 0)
	runtime.KeepAlive(p)
	runtime.KeepAlive(&attrs)
	runtime.KeepAlive(buf)
	if errno != 0 || binary.LittleEndian.Uint32(buf[20:])&0x8 == 0 {
		return 0, false
	}
	n := int64(binary.LittleEndian.Uint64(buf[24:]))
	return n, n >= 0
}
