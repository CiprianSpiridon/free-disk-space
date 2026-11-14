package rustusage

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var channel = regexp.MustCompile(`(?m)^\s*channel\s*=\s*["']([^"']+)["']\s*(?:#.*)?$`)

// ReadPins accepts legacy rust-toolchain and the [toolchain] TOML channel.
// Any unreadable/unsupported pin makes unused classification inconclusive.
func ReadPins(paths []string) (map[string]string, bool) {
	refs := map[string]string{}
	complete := true
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			complete = false
			continue
		}
		s := strings.TrimSpace(string(b))
		name := ""
		if strings.HasPrefix(s, "[") {
			if m := channel.FindStringSubmatch(s); len(m) > 1 {
				name = m[1]
			}
		} else if filepath.Base(p) == "rust-toolchain" {
			name = strings.TrimSpace(strings.Split(s, "\n")[0])
		}
		if name == "" || strings.ContainsAny(name, " \t\r\n") {
			complete = false
			continue
		}
		refs[name] = p
	}
	return refs, complete
}

func Collect(roots []string) (map[string]string, bool) {
	var paths []string
	complete := true
	seen := map[string]bool{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				complete = false
				return nil
			}
			if seen[p] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			seen[p] = true
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules", "target", "vendor", ".venv", "venv", ".next", "build", "dist":
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.Name() == "rust-toolchain" || d.Name() == "rust-toolchain.toml" {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			complete = false
		}
	}
	refs, readOK := ReadPins(paths)
	return refs, complete && readOK
}

func Matches(name, ref string) bool {
	return name == ref || strings.HasPrefix(name, ref+"-") || strings.HasPrefix(name, ref+".")
}
