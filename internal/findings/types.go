package findings

import (
	"fmt"
	"strings"
	"time"
)

// Risk is the reclaim classification from findings.schema.json.
type Risk string

const (
	RiskSafeCache        Risk = "safe-cache"
	RiskRebuildable      Risk = "rebuildable"
	RiskLeftoverWorktree Risk = "leftover-worktree"
	RiskUnusedRuntime    Risk = "unused-runtime"
	RiskAsk              Risk = "ask"
	RiskKeep             Risk = "keep"
	RiskNever            Risk = "never"
)

var validRisks = map[Risk]struct{}{
	RiskSafeCache: {}, RiskRebuildable: {}, RiskLeftoverWorktree: {},
	RiskUnusedRuntime: {}, RiskAsk: {}, RiskKeep: {}, RiskNever: {},
}

// ParseRisk returns an error for values outside the schema enum.
func ParseRisk(s string) (Risk, error) {
	r := Risk(s)
	if _, ok := validRisks[r]; !ok {
		return "", fmt.Errorf("invalid risk %q", s)
	}
	return r, nil
}

// Reclaim is a proposed command. Scan never executes it.
type Reclaim struct {
	Cmd    string `json:"cmd"`
	DryRun string `json:"dry_run,omitempty"`
}

// Finding is one reclaimable or notable path.
type Finding struct {
	ID             string  `json:"id"`
	Path           string  `json:"path"`
	Bytes          int64   `json:"bytes"`
	BytesApparent  int64   `json:"bytes_apparent,omitempty"`
	Category       string  `json:"category"`
	Risk           Risk    `json:"risk"`
	LastUsed       string  `json:"last_used,omitempty"`
	Why            string  `json:"why,omitempty"`
	Count          int     `json:"count,omitempty"`
	Reclaim        *Reclaim `json:"reclaim,omitempty"`
}

// Host is the scanned machine.
type Host struct {
	OS   string `json:"os"`
	Home string `json:"home"`
	Arch string `json:"arch,omitempty"`
}

// Volume is APFS container fullness, not df /.
type Volume struct {
	ContainerBytes     int64    `json:"container_bytes"`
	InUseBytes         int64    `json:"in_use_bytes"`
	FreeBytes          int64    `json:"free_bytes"`
	DataVolumeUsedBytes int64   `json:"data_volume_used_bytes,omitempty"`
	DFRootUsedBytes    int64    `json:"df_root_used_bytes,omitempty"`
	Snapshots          []string `json:"snapshots,omitempty"`
	Swap               string   `json:"swap,omitempty"`
}

// Report is the scan document.
type Report struct {
	GeneratedAt string    `json:"generated_at"`
	Host        Host      `json:"host"`
	Volume      Volume    `json:"volume"`
	Unreadable  []string  `json:"unreadable,omitempty"`
	NotPresent  []string  `json:"not_present,omitempty"`
	Findings    []Finding `json:"findings"`
}

// NewReport returns a schema-valid empty report.
func NewReport(home string) Report {
	return Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Host:        Host{OS: "macos", Home: home},
		Findings:    []Finding{},
	}
}

// ValidateRisks reports the first illegal risk value.
func (r Report) ValidateRisks() error {
	for _, f := range r.Findings {
		if _, err := ParseRisk(string(f.Risk)); err != nil {
			return err
		}
	}
	return nil
}

// IDSlug turns a path into a stable-ish finding id.
func IDSlug(category, path string) string {
	s := category
	if s == "" {
		s = "path"
	}
	repl := strings.NewReplacer("/", "-", " ", "-", "~", "home")
	p := repl.Replace(strings.Trim(path, "/"))
	if len(p) > 48 {
		p = p[len(p)-48:]
	}
	return strings.Trim(s+"-"+p, "-")
}
