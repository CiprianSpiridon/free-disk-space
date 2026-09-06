package policy

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var denyExact = []string{"/tmp", "/private/tmp", "/var/tmp"}

var denyPrefixes = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/private/var/vm",
	"/dev",
	"/net",
}

// CanDelete reports whether path is allowed as a delete target.
func CanDelete(path string) bool {
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
	home, _ := os.UserHomeDir()
	if home != "" && (clean == filepath.Join(home, ".cargo") || clean == home+"/.cargo") {
		return false
	}
	for _, d := range denyExact {
		if clean == d {
			return false
		}
	}
	if strings.HasPrefix(clean, "/usr/local") {
		return true
	}
	for _, pre := range denyPrefixes {
		if clean == pre || strings.HasPrefix(clean, pre+"/") {
			return false
		}
	}
	return true
}
