package policy

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var denyExact = []string{
	"/tmp", "/private/tmp",
	"/var/tmp", "/private/var/tmp",
}

var denyPrefixes = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/private/var/vm",
	"/var/vm",
	"/dev",
	"/net",
}

// canonical expands well-known macOS alias roots only (/tmp → /private/tmp).
func canonical(path string) string {
	p := filepath.Clean(path)
	aliases := [][2]string{
		{"/tmp", "/private/tmp"},
		{"/var", "/private/var"},
		{"/etc", "/private/etc"},
	}
	for _, a := range aliases {
		if p == a[0] || strings.HasPrefix(p, a[0]+"/") {
			p = a[1] + strings.TrimPrefix(p, a[0])
		}
	}
	return p
}

func sameCanonical(a, b string) bool {
	return canonical(a) == canonical(b)
}

func resolveExisting(path string) (string, bool) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", false
	}
	cur := "/"
	rest := strings.TrimPrefix(clean, "/")
	if rest == "" {
		return "/", true
	}
	for _, part := range strings.Split(rest, "/") {
		if part == "" || part == "." {
			continue
		}
		next := filepath.Join(cur, part)
		fi, err := os.Lstat(next)
		if err != nil {
			return "", false
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", false
			}
			next = target
		}
		cur = next
	}
	return cur, true
}

func lexicalOK(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	if strings.Contains(path, "\x00") {
		return false
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, p := range strings.Split(path, string(os.PathSeparator)) {
		if p == ".." {
			return false
		}
	}
	clean := filepath.Clean(path)
	c := canonical(clean)
	if clean == "/" || c == "/" {
		return false
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		h := filepath.Clean(home)
		ch := canonical(h)
		if clean == h || c == ch {
			return false
		}
		if clean == filepath.Join(h, ".cargo") || c == filepath.Join(ch, ".cargo") {
			return false
		}
	}
	if tooBroad(clean, home) {
		return false
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		t := filepath.Clean(tmp)
		if t != "" && (clean == t || c == canonical(t)) {
			return false
		}
	}
	for _, d := range denyExact {
		if clean == d || c == canonical(d) {
			return false
		}
	}
	if strings.HasPrefix(clean, "/usr/local") || strings.HasPrefix(c, "/usr/local") {
		return true
	}
	for _, pre := range denyPrefixes {
		cp := canonical(pre)
		if clean == pre || c == cp || strings.HasPrefix(clean, pre+"/") || strings.HasPrefix(c, cp+"/") {
			return false
		}
	}
	return true
}

func tooBroad(clean, home string) bool {
	if clean == "/opt/homebrew" || clean == "/Library/Developer" {
		return true
	}
	if home == "" {
		return false
	}
	h := filepath.Clean(home)
	for _, b := range []string{"Downloads", "Desktop", "Documents", "Pictures", "Movies", "Music", "Library"} {
		if clean == filepath.Join(h, b) {
			return true
		}
	}
	if clean == filepath.Join(h, "Library", "Containers") {
		return true
	}
	return false
}

// CanDelete reports whether path is allowed as a delete target.
// Intermediate symlinks that escape the canonical path are refused.
// A last-component symlink is unlinked in place (os.RemoveAll does not follow it).
func CanDelete(path string) bool {
	if !lexicalOK(path) {
		return false
	}
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	if parent != "/" {
		if rp, ok := resolveExisting(parent); ok {
			if !sameCanonical(parent, rp) {
				return false
			}
		}
	}
	resolved, ok := resolveExisting(clean)
	if !ok {
		return true
	}
	if !lexicalOK(resolved) {
		return false
	}
	fi, err := os.Lstat(clean)
	if err == nil && fi.Mode()&os.ModeSymlink == 0 {
		if !sameCanonical(clean, resolved) {
			return false
		}
	}
	return true
}
