package catalog

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Overlay is the user catalog file.
type Overlay struct {
	Add          []Entry             `yaml:"add"`
	Disable      []string            `yaml:"disable"`
	Unassign     []Unassign          `yaml:"unassign"`
	DisableScans []string            `yaml:"disable_scans"`
	Modes        map[string]UserMode `yaml:"modes"`
	WorkRoots    []Entry             `yaml:"work_roots"`
}

// Unassign removes a path from listed modes.
type Unassign struct {
	Path string   `yaml:"path"`
	From []string `yaml:"from"`
}

// UserMode is a named scan composed of phase types.
type UserMode struct {
	Types []string `yaml:"types"`
}

func defaultScans(e Entry, workRoot bool) []string {
	if len(e.Scans) > 0 {
		return append([]string{}, e.Scans...)
	}
	if workRoot {
		return []string{"dev", "full"}
	}
	return []string{"quick", "full"}
}

func hasScan(scans []string, mode string) bool {
	if mode == "full" {
		return true
	}
	for _, s := range scans {
		if s == mode {
			return true
		}
	}
	return false
}

func union(a, b []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if x == "" {
			continue
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

func subtract(scans, from []string) []string {
	drop := map[string]struct{}{}
	for _, f := range from {
		drop[f] = struct{}{}
	}
	var out []string
	for _, s := range scans {
		if _, ok := drop[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

func expandKey(p, home string) string {
	return ExpandPath(p, home)
}

func applyList(list []Entry, workRoot bool, home string, disabled map[string]struct{}, unassign map[string][]string) []Entry {
	var next []Entry
	for _, e := range list {
		exp := expandKey(e.Path, home)
		e.Path = exp
		e.Scans = defaultScans(e, workRoot)
		if _, ok := disabled[exp]; ok {
			continue
		}
		if from, ok := unassign[exp]; ok {
			e.Scans = subtract(e.Scans, from)
		}
		next = append(next, e)
	}
	return next
}

func mergeAdd(list *[]Entry, add Entry) bool {
	for i := range *list {
		if (*list)[i].Path == add.Path {
			(*list)[i].Scans = union((*list)[i].Scans, add.Scans)
			if add.Category != "" {
				(*list)[i].Category = add.Category
			}
			if add.Risk != "" {
				(*list)[i].Risk = add.Risk
			}
			(*list)[i].Glob = (*list)[i].Glob || add.Glob
			return true
		}
	}
	return false
}

// Merge applies overlay onto bundled catalog. Missing overlay is bundled only.
func Merge(bundled *Catalog, ov *Overlay, home string) *Catalog {
	if ov == nil {
		ov = &Overlay{}
	}
	out := *bundled
	disabled := map[string]struct{}{}
	for _, d := range ov.Disable {
		disabled[expandKey(d, home)] = struct{}{}
	}
	unassign := map[string][]string{}
	for _, u := range ov.Unassign {
		unassign[expandKey(u.Path, home)] = u.From
	}

	out.Home = applyList(bundled.Home, false, home, disabled, unassign)
	out.WorkRoots = applyList(bundled.WorkRoots, true, home, disabled, unassign)
	out.SystemWide = applyList(bundled.SystemWide, false, home, disabled, unassign)
	out.Tmp = applyList(bundled.Tmp, false, home, disabled, unassign)
	out.Library = applyList(bundled.Library, false, home, disabled, unassign)
	out.Xcode = applyList(bundled.Xcode, false, home, disabled, unassign)
	out.Android = applyList(bundled.Android, false, home, disabled, unassign)
	out.DockerVMs = applyList(bundled.DockerVMs, false, home, disabled, unassign)
	out.Node = applyList(bundled.Node, false, home, disabled, unassign)
	out.LanguageHomes = applyList(bundled.LanguageHomes, false, home, disabled, unassign)
	out.Python = applyList(bundled.Python, false, home, disabled, unassign)
	out.Rust = applyList(bundled.Rust, false, home, disabled, unassign)
	out.GoJavaPHRuby = applyList(bundled.GoJavaPHRuby, false, home, disabled, unassign)
	out.AILocal = applyList(bundled.AILocal, false, home, disabled, unassign)
	out.AgentCLIs = applyList(bundled.AgentCLIs, false, home, disabled, unassign)
	out.Editors = applyList(bundled.Editors, false, home, disabled, unassign)
	out.VMs = applyList(bundled.VMs, false, home, disabled, unassign)
	out.BrowserRuntimes = applyList(bundled.BrowserRuntimes, false, home, disabled, unassign)
	out.WorkRoots = append(out.WorkRoots, applyList(ov.WorkRoots, true, home, disabled, unassign)...)

	for _, add := range ov.Add {
		add.Path = expandKey(add.Path, home)
		add.Scans = defaultScans(add, false)
		if mergeAdd(&out.Home, add) || mergeAdd(&out.Node, add) || mergeAdd(&out.Tmp, add) || mergeAdd(&out.WorkRoots, add) {
			continue
		}
		out.Home = append(out.Home, add)
	}

	out.disableScans = append([]string{}, ov.DisableScans...)
	out.userModes = ov.Modes
	return &out
}

// DisabledScans returns overlay disable_scans.
func (c *Catalog) DisabledScans() []string { return c.disableScans }

// ModeTypes returns overlay modes.<name>.types.
func (c *Catalog) ModeTypes(name string) []string {
	if c.userModes == nil {
		return nil
	}
	return c.userModes[name].Types
}

// ModePaths returns entries tagged for mode.
func (c *Catalog) ModePaths(mode string) []Entry {
	var out []Entry
	for _, e := range c.AllEntries() {
		if hasScan(e.Scans, mode) {
			out = append(out, e)
		}
	}
	return out
}

// OverlayPath is the default user overlay location.
func OverlayPath(configDir string) string {
	if configDir != "" {
		return filepath.Join(configDir, "catalog.yaml")
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "freedisk", "catalog.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "freedisk", "catalog.yaml")
}

// LoadOverlay returns nil overlay if the file is missing.
func LoadOverlay(path string) (*Overlay, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Overlay{Modes: map[string]UserMode{}}, nil
		}
		return nil, err
	}
	var ov Overlay
	if err := yaml.Unmarshal(b, &ov); err != nil {
		return nil, err
	}
	if ov.Modes == nil {
		ov.Modes = map[string]UserMode{}
	}
	return &ov, nil
}

// SaveOverlay writes the overlay file, creating parent dirs.
func SaveOverlay(path string, ov *Overlay) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(ov)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
