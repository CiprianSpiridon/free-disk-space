package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	catalogdata "github.com/CiprianSpiridon/free-disk-space/catalog"
	"gopkg.in/yaml.v3"
)

// Entry is one catalog path row.
type Entry struct {
	Path          string   `yaml:"path"`
	Category      string   `yaml:"category"`
	Risk          string   `yaml:"risk"`
	Drill         bool     `yaml:"drill"`
	AlwaysDrill   bool     `yaml:"always_drill"`
	WalkArtifacts bool     `yaml:"walk_artifacts"`
	Glob          bool     `yaml:"glob"`
	Sparse        bool     `yaml:"sparse"`
	Reclaim       string   `yaml:"reclaim"`
	Note          string   `yaml:"note"`
	Scans         []string `yaml:"scans"`
	GlobChildren  string   `yaml:"glob_children"`
}

// Artifact is a walk marker.
type Artifact struct {
	Name       string   `yaml:"name"`
	MarkerFile string   `yaml:"marker_file"`
	Ecosystem  string   `yaml:"ecosystem"`
	Risk       string   `yaml:"risk"`
	Confirm    string   `yaml:"confirm"`
	File       bool     `yaml:"file"`
	Note       string   `yaml:"note"`
	ThenCheck  []string `yaml:"then_check"`
}

// WorkRootDiscover finds extra $HOME project containers.
type WorkRootDiscover struct {
	Enabled        bool     `yaml:"enabled"`
	HomeDepth      int      `yaml:"home_depth"`
	MinChildRepos  int      `yaml:"min_child_repos"`
	ProjectMarkers []string `yaml:"project_markers"`
	SkipNames      []string `yaml:"skip_names"`
}

// WorktreeMarker locates leftover worktrees.
type WorktreeMarker struct {
	PathSuffix string `yaml:"path_suffix"`
	Path       string `yaml:"path"`
	Kind       string `yaml:"kind"`
}

// Thresholds are scan cutoffs.
type Thresholds struct {
	DrillBytes            int64    `yaml:"drill_bytes"`
	ReportBytes           int64    `yaml:"report_bytes"`
	ArtifactListMinBytes  int64    `yaml:"artifact_list_min_bytes"`
	ArtifactIdleDays      int      `yaml:"artifact_idle_days"`
	TmpIdleDays           int      `yaml:"tmp_idle_days"`
	WorktreeIdleDays      int      `yaml:"worktree_idle_days"`
	WorktreeInflightHours int      `yaml:"worktree_inflight_hours"`
	MaxWalkDepth          int      `yaml:"max_walk_depth"`
	SkipSystemPrefixes    []string `yaml:"skip_system_prefixes"`
}

// Catalog is the bundled hotspot file.
type Catalog struct {
	Version          int                 `yaml:"version"`
	OS               string              `yaml:"os"`
	Home             []Entry             `yaml:"home"`
	WorkRoots        []Entry             `yaml:"work_roots"`
	WorkRootDiscover WorkRootDiscover    `yaml:"work_root_discover"`
	SystemWide       []Entry             `yaml:"system_wide"`
	Tmp              []Entry             `yaml:"tmp"`
	Library          []Entry             `yaml:"library"`
	Xcode            []Entry             `yaml:"xcode"`
	Android          []Entry             `yaml:"android"`
	DockerVMs        []Entry             `yaml:"docker_vms"`
	Node             []Entry             `yaml:"node"`
	LanguageHomes    []Entry             `yaml:"language_homes"`
	Python           []Entry             `yaml:"python"`
	Rust             []Entry             `yaml:"rust"`
	GoJavaPHRuby     []Entry             `yaml:"go_java_php_ruby"`
	AILocal          []Entry             `yaml:"ai_local"`
	AgentCLIs        []Entry             `yaml:"agent_clis"`
	Editors          []Entry             `yaml:"editors"`
	VMs              []Entry             `yaml:"vms"`
	BrowserRuntimes  []Entry             `yaml:"browser_runtimes"`
	Artifacts        []Artifact          `yaml:"artifacts"`
	WorktreeMarkers  []WorktreeMarker    `yaml:"worktree_markers"`
	WalkPrune        []string            `yaml:"walk_prune"`
	Apis             map[string][]string `yaml:"apis"`
	Thresholds       Thresholds          `yaml:"thresholds"`
	source           string
	disableScans     []string
	disablePaths     []string
	userModes        map[string]UserMode
}

// PathDisabled reports whether overlay disabled this expanded path.
func (c *Catalog) PathDisabled(p string) bool {
	if c == nil {
		return false
	}
	for _, d := range c.disablePaths {
		if d == p {
			return true
		}
	}
	return false
}

// AllEntries concatenates path groups (not artifacts).
func (c *Catalog) AllEntries() []Entry {
	groups := [][]Entry{
		c.Home, c.WorkRoots, c.SystemWide, c.Tmp, c.Library, c.Xcode, c.Android,
		c.DockerVMs, c.Node, c.LanguageHomes, c.Python, c.Rust, c.GoJavaPHRuby,
		c.AILocal, c.AgentCLIs, c.Editors, c.VMs, c.BrowserRuntimes,
	}
	var out []Entry
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func defaults(t *Thresholds) {
	if t.ArtifactIdleDays == 0 {
		t.ArtifactIdleDays = 30
	}
	if t.TmpIdleDays == 0 {
		t.TmpIdleDays = 7
	}
	if t.WorktreeInflightHours == 0 {
		t.WorktreeInflightHours = 24
	}
	if t.WorktreeIdleDays == 0 {
		t.WorktreeIdleDays = 14
	}
	if t.MaxWalkDepth == 0 {
		t.MaxWalkDepth = 8
	}
}

func parse(source string, b []byte) (*Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("catalog parse %s: %w", source, err)
	}
	defaults(&c.Thresholds)
	c.source = source
	return &c, nil
}

// Load reads a catalog YAML file. Missing file is a hard error.
func Load(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog not found: %s: %w", path, err)
	}
	return parse(path, b)
}

// LoadBundled returns the catalog compiled into the binary.
func LoadBundled() (*Catalog, error) {
	return parse("bundled", catalogdata.YAML)
}

// LoadDefault is --catalog, FREEDISK_CATALOG, then the embedded catalog (no cwd walk).
func LoadDefault(explicit string) (*Catalog, []string, error) {
	path, tried, err := ResolveCatalogPath(explicit)
	if path != "" {
		c, lerr := Load(path)
		return c, tried, lerr
	}
	if explicit != "" || os.Getenv("FREEDISK_CATALOG") != "" {
		return nil, tried, err
	}
	c, berr := LoadBundled()
	if berr != nil {
		return nil, tried, berr
	}
	return c, append(tried, "bundled"), nil
}

// ResolveCatalogPath search order: --catalog, FREEDISK_CATALOG. Otherwise LoadDefault uses the embedded YAML.
func ResolveCatalogPath(explicit string) (string, []string, error) {
	var tried []string
	if explicit != "" {
		tried = append(tried, explicit)
		if _, err := os.Stat(explicit); err == nil {
			return explicit, tried, nil
		}
		return "", tried, fmt.Errorf("catalog not found: %s", explicit)
	}
	if env := os.Getenv("FREEDISK_CATALOG"); env != "" {
		tried = append(tried, env)
		if _, err := os.Stat(env); err == nil {
			return env, tried, nil
		}
		return "", tried, fmt.Errorf("catalog not found: %s", env)
	}
	return "", tried, nil
}

// ExpandEntry expands ~/$ENV and optional globs for one catalog row.
func ExpandEntry(e Entry, home string) []string {
	p := ExpandPath(e.Path, home)
	if p == "" {
		return nil
	}
	if !e.Glob {
		return []string{p}
	}
	m, err := filepath.Glob(p)
	if err != nil || len(m) == 0 {
		return nil
	}
	const capN = 4096
	if len(m) > capN {
		return m[:capN]
	}
	return m
}

// ExpandPath expands ~ and $ENV in a catalog path.
func ExpandPath(p, home string) string {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[2:])
	} else if p == "~" {
		p = home
	}
	if strings.Contains(p, "$") {
		p = os.ExpandEnv(p)
	}
	return p
}
