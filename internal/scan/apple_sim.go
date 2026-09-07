package scan

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/size"
)

func init() {
	Register(Phase{Name: "apple-sim", Quick: false, Dev: false, Run: runAppleSim})
}

func simctlTimeout(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "xcrun", args...).Output()
}

// SimctlJSON is injected in tests.
var SimctlJSON = func() ([]byte, error) {
	return simctlTimeout("simctl", "list", "devices", "-j")
}

// SimctlRuntimesJSON is injected in tests.
var SimctlRuntimesJSON = func() ([]byte, error) {
	return simctlTimeout("simctl", "list", "runtimes", "-j")
}

type simctlDevices struct {
	Devices map[string][]struct {
		UDID              string `json:"udid"`
		Name              string `json:"name"`
		State             string `json:"state"`
		RuntimeIdentifier string `json:"runtimeIdentifier"`
		LastBootedAt      string `json:"lastBootedAt"`
	} `json:"devices"`
}

type simctlRuntimes struct {
	Runtimes []struct {
		Identifier   string `json:"identifier"`
		Name         string `json:"name"`
		IsAvailable  bool   `json:"isAvailable"`
		Buildversion string `json:"buildversion"`
		RuntimeRoot  string `json:"runtimeRoot"`
	} `json:"runtimes"`
}

func runAppleSim(ctx *Context) error {
	if _, err := exec.LookPath("xcrun"); err != nil {
		ctx.Report.NotPresent = append(ctx.Report.NotPresent, "simctl")
		return nil
	}
	b, err := SimctlJSON()
	if err != nil {
		ctx.Report.NotPresent = append(ctx.Report.NotPresent, "simctl")
		return nil
	}
	booted, perr := ParseSimctlDevices(b, ctx.Report, ctx.Home)
	if perr != nil {
		ctx.Report.NotPresent = append(ctx.Report.NotPresent, "simctl-json")
	}
	rb, err := SimctlRuntimesJSON()
	if err == nil {
		ParseSimctlRuntimes(rb, ctx.Report, booted, "/Library/Developer/CoreSimulator/Volumes")
	}
	return nil
}

// ParseSimctlDevices joins on runtimeIdentifier, not display name.
// Path is the on-disk device directory when home is set.
func ParseSimctlDevices(raw []byte, rep *findings.Report, home string) (map[string]bool, error) {
	booted := map[string]bool{}
	var doc simctlDevices
	if err := json.Unmarshal(raw, &doc); err != nil {
		return booted, err
	}
	deviceDir := ""
	if home != "" {
		deviceDir = filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices")
	}
	for group, devs := range doc.Devices {
		for _, d := range devs {
			if d.RuntimeIdentifier != "" && d.LastBootedAt != "" {
				booted[d.RuntimeIdentifier] = true
			}
			risk := findings.RiskAsk
			if d.LastBootedAt == "" {
				risk = findings.RiskUnusedRuntime
			}
			why := "runtimeIdentifier=" + d.RuntimeIdentifier + " group=" + group
			p := d.UDID
			var bytes int64
			if deviceDir != "" && d.UDID != "" {
				p = filepath.Join(deviceDir, d.UDID)
				sz := size.Of(p)
				if !sz.Missing && sz.Err == nil {
					bytes = sz.Allocated
				}
			}
			rep.Findings = append(rep.Findings, findings.Finding{
				ID: "sim-" + d.UDID, Path: p, Bytes: bytes,
				Category: "simulator-device", Risk: risk,
				LastUsed: d.LastBootedAt, Why: why,
				Reclaim: &findings.Reclaim{Cmd: "xcrun simctl delete " + d.UDID},
			})
		}
	}
	return booted, nil
}

// ParseSimctlRuntimes marks a runtime unused-runtime only when none of its devices were ever booted.
func ParseSimctlRuntimes(raw []byte, rep *findings.Report, booted map[string]bool, volumesDir string) {
	var doc simctlRuntimes
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	if booted == nil {
		booted = map[string]bool{}
	}
	for _, rt := range doc.Runtimes {
		if rt.Identifier == "" {
			continue
		}
		risk := findings.RiskAsk
		why := "simctl runtime " + rt.Name
		if !booted[rt.Identifier] {
			risk = findings.RiskUnusedRuntime
			why = "runtime never booted identifier=" + rt.Identifier
		}
		p := rt.Identifier
		var bytes int64
		if vol := runtimeVolumePath(rt.RuntimeRoot, rt.Buildversion, rt.Name, volumesDir); vol != "" {
			p = vol
			sz := size.Of(vol)
			if !sz.Missing && sz.Err == nil {
				bytes = sz.Allocated
			}
		}
		rep.Findings = append(rep.Findings, findings.Finding{
			ID:       findings.IDSlug("sim-runtime", rt.Identifier),
			Path:     p,
			Bytes:    bytes,
			Category: "simulator-runtime",
			Risk:     risk,
			Why:      why,
			Reclaim:  &findings.Reclaim{Cmd: "xcrun simctl runtime delete " + rt.Identifier},
		})
	}
}

func runtimeVolumePath(runtimeRoot, build, name, volumesDir string) string {
	if runtimeRoot != "" {
		if i := strings.Index(runtimeRoot, "/Volumes/"); i >= 0 {
			rest := runtimeRoot[i+len("/Volumes/"):]
			vol, _, _ := strings.Cut(rest, "/")
			if vol != "" && volumesDir != "" {
				return filepath.Join(volumesDir, vol)
			}
		}
		return runtimeRoot
	}
	if volumesDir == "" {
		return ""
	}
	ents, err := os.ReadDir(volumesDir)
	if err != nil {
		return ""
	}
	want := ""
	if build != "" {
		want = strings.ToLower(build)
	}
	prefix := runtimeVolumePrefix(name)
	var fallback string
	for _, e := range ents {
		n := e.Name()
		low := strings.ToLower(n)
		if want != "" && strings.Contains(low, want) {
			return filepath.Join(volumesDir, n)
		}
		if prefix != "" && strings.HasPrefix(n, prefix) && fallback == "" {
			fallback = filepath.Join(volumesDir, n)
		}
	}
	return fallback
}

func runtimeVolumePrefix(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "watchos"):
		return "watchOS"
	case strings.Contains(n, "tvos"):
		return "tvOS"
	case strings.Contains(n, "ios"):
		return "iOS"
	case strings.Contains(n, "xros") || strings.Contains(n, "vision"):
		return "xrOS"
	default:
		return ""
	}
}
