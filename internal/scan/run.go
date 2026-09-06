package scan

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

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
	Mode    string
	Catalog *catalog.Catalog
	Home    string
	Report  *findings.Report
	Diskutil func() (string, error)
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
	ctx.Report.Volume = v
	return nil
}

func runKnown(ctx *Context) error {
	Known(ctx.Catalog, ctx.Mode, ctx.Home, ctx.Report)
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
	mu.Lock()
	list := append([]Phase{}, phases...)
	mu.Unlock()
	var first error
	for _, p := range list {
		if !wantPhase(ctx, p) {
			continue
		}
		if err := p.Run(ctx); err != nil {
			if p.Name == VolumePhaseName {
				return err
			}
			if first == nil {
				first = err
			}
		}
	}
	return nil
}

// PhasesForTest returns a copy of registered phases.
func PhasesForTest() []Phase {
	registerCore()
	mu.Lock()
	defer mu.Unlock()
	return append([]Phase{}, phases...)
}

