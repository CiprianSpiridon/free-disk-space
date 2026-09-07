package scan

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

// recipeOrder is RECIPE §4: volume, known, tmp, drill, artifacts, worktrees, sims.
var recipeOrder = []string{
	VolumePhaseName, KnownPhaseName, "tmp", "drill",
	"artifacts", "worktrees", "apple-sim", "android-sim",
}

// Phase is a named scan step.
type Phase struct {
	Name  string
	Quick bool
	Dev   bool
	Run   func(*Context) error
}

var (
	mu     sync.Mutex
	phases []Phase
	core   sync.Once
)

// Register adds a phase. Later files call this from init().
func Register(p Phase) {
	mu.Lock()
	defer mu.Unlock()
	for _, x := range phases {
		if x.Name == p.Name {
			return
		}
	}
	phases = append(phases, p)
}

// Context is one scan run.
type Context struct {
	Mode     string
	Catalog  *catalog.Catalog
	Home     string
	Report   *findings.Report
	Diskutil func() (string, error)
	Log      func(string)
}

func (ctx *Context) logf(format string, args ...any) {
	if ctx == nil || ctx.Log == nil {
		return
	}
	ctx.Log(fmt.Sprintf(format, args...))
}

func registerCore() {
	core.Do(func() {
		Register(Phase{Name: VolumePhaseName, Quick: true, Dev: true, Run: runVolume})
		Register(Phase{Name: KnownPhaseName, Quick: true, Dev: true, Run: runKnown})
	})
}

func runVolume(ctx *Context) error {
	fn := ctx.Diskutil
	if fn == nil {
		fn = func() (string, error) {
			if _, err := exec.LookPath("diskutil"); err != nil {
				return "", fmt.Errorf("diskutil missing")
			}
			out, err := exec.Command("diskutil", "apfs", "list").Output()
			return string(out), err
		}
	}
	out, err := fn()
	if err != nil {
		if ctx.Report != nil {
			ctx.Report.NotPresent = append(ctx.Report.NotPresent, "diskutil")
		}
		return err
	}
	v, err := ParseVolume(out)
	if err != nil {
		return err
	}
	v.Snapshots = listLocalSnapshots()
	ctx.Report.Volume = v
	return nil
}

func listLocalSnapshots() []string {
	out, err := exec.Command("tmutil", "listlocalsnapshots", "/").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Snapshots") {
			continue
		}
		names = append(names, line)
	}
	return names
}

func runKnown(ctx *Context) error {
	Known(ctx)
	return nil
}

func disabled(ctx *Context, name string) bool {
	if name == VolumePhaseName {
		return false
	}
	for _, d := range ctx.Catalog.DisabledScans() {
		if d == name {
			return true
		}
	}
	return false
}

func wantPhase(ctx *Context, p Phase) bool {
	if disabled(ctx, p.Name) {
		return false
	}
	switch ctx.Mode {
	case "quick":
		return p.Quick
	case "dev":
		return p.Dev
	case "full":
		return true
	default:
		types := ctx.Catalog.ModeTypes(ctx.Mode)
		if types == nil {
			return false
		}
		if p.Name == VolumePhaseName {
			return true
		}
		for _, t := range types {
			if t == p.Name {
				return true
			}
		}
		return false
	}
}

// ValidMode reports whether mode is built-in or a user overlay mode.
func ValidMode(mode string, cat *catalog.Catalog) bool {
	switch mode {
	case "quick", "dev", "full":
		return true
	}
	return cat != nil && cat.ModeTypes(mode) != nil
}

// Run executes registered phases for mode.
func Run(ctx *Context) error {
	registerCore()
	if ctx.Report == nil {
		r := findings.NewReport(ctx.Home)
		ctx.Report = &r
	}
	ctx.Report.Host.OS = "macos"
	ctx.Report.Host.Home = ctx.Home
	ctx.Report.Host.Arch = runtime.GOARCH
	if ctx.Catalog != nil {
		size.SetSkipPrefixes(ctx.Catalog.Thresholds.SkipSystemPrefixes)
	}
	size.SetHeartbeat(func(root, current string, visited int, elapsed time.Duration) {
		ctx.logf("still walking %s (%d inodes, %s) … %s", root, visited, elapsed.Round(time.Second), current)
	})
	defer size.SetHeartbeat(nil)
	ctx.logf("scan %s starting", ctx.Mode)
	mu.Lock()
	list := append([]Phase{}, phases...)
	mu.Unlock()
	list = orderPhases(list)
	var first error
	for _, p := range list {
		if !wantPhase(ctx, p) {
			continue
		}
		ctx.logf("phase %s", p.Name)
		t0 := time.Now()
		if err := p.Run(ctx); err != nil {
			ctx.logf("phase %s failed after %s: %v", p.Name, time.Since(t0).Round(time.Millisecond), err)
			if p.Name == VolumePhaseName {
				return err
			}
			if first == nil {
				first = err
			}
			continue
		}
		ctx.logf("phase %s done in %s", p.Name, time.Since(t0).Round(time.Millisecond))
	}
	if ctx.Report != nil {
		findings.UniquifyIDs(ctx.Report)
		ctx.logf("scan done %d findings", len(ctx.Report.Findings))
	}
	return first
}

func orderPhases(list []Phase) []Phase {
	rank := map[string]int{}
	for i, n := range recipeOrder {
		rank[n] = i + 1
	}
	sort.SliceStable(list, func(i, j int) bool {
		ri, rj := rank[list[i].Name], rank[list[j].Name]
		if ri == 0 && rj == 0 {
			return false
		}
		if ri == 0 {
			return false
		}
		if rj == 0 {
			return true
		}
		return ri < rj
	})
	return list
}

// PhasesForTest returns a copy of registered phases.
func PhasesForTest() []Phase {
	registerCore()
	mu.Lock()
	defer mu.Unlock()
	return append([]Phase{}, phases...)
}
