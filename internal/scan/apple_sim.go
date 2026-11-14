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
	"github.com/CiprianSpiridon/free-disk-space/internal/simulator"
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

var SimctlImagesJSON = func() ([]byte, error) { return simctlTimeout("simctl", "runtime", "list", "-j") }

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
	booted, perr := parseSimctlDevices(b, ctx.Report, ctx.Home, ctx.measure)
	if perr != nil {
		ctx.Report.NotPresent = append(ctx.Report.NotPresent, "simctl-json")
		booted = nil
	}
	rb, err := SimctlRuntimesJSON()
	if err == nil {
		ib, ierr := SimctlImagesJSON()
		images, parseErr := simulator.ParseImages(ib)
		if ierr != nil || parseErr != nil {
			ctx.Report.NotPresent = append(ctx.Report.NotPresent, "simctl-runtime-images")
		}
		parseSimctlRuntimes(rb, ctx.Report, booted, "", images, ctx.measure)
	}
	return nil
}

// ParseSimctlDevices joins on runtimeIdentifier, not display name.
// Path is the on-disk device directory when home is set.
func ParseSimctlDevices(raw []byte, rep *findings.Report, home string) (map[string]bool, error) {
	return parseSimctlDevices(raw, rep, home, size.NewScanner().Of)
}

func parseSimctlDevices(raw []byte, rep *findings.Report, home string, measure func(string) size.Result) (map[string]bool, error) {
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
			runtimeID := d.RuntimeIdentifier
			if runtimeID == "" && strings.HasPrefix(group, "com.apple.CoreSimulator.SimRuntime.") {
				runtimeID = group
			}
			if runtimeID != "" && (d.LastBootedAt != "" || d.State == "Booted") {
				booted[runtimeID] = true
			}
			risk := findings.RiskAsk
			if d.LastBootedAt == "" && d.State != "Booted" {
				risk = findings.RiskUnusedRuntime
			}
			why := d.Name + " state=" + d.State + " runtimeIdentifier=" + runtimeID
			p := d.UDID
			var bytes int64
			var reclaimBytes *int64
			if deviceDir != "" && d.UDID != "" {
				p = filepath.Join(deviceDir, d.UDID)
				sz := measure(p)
				if !sz.Missing && sz.Err == nil {
					bytes = sz.Allocated
					reclaimBytes = privateBytes(sz)
				}
			}
			rep.Findings = append(rep.Findings, findings.Finding{
				ID: "sim-" + d.UDID, Path: p, Bytes: bytes, ReclaimBytes: reclaimBytes,
				Category: "simulator-device", Risk: risk,
				LastUsed: d.LastBootedAt, Why: why,
				Reclaim: &findings.Reclaim{Cmd: "xcrun simctl shutdown " + d.UDID + "; xcrun simctl delete " + d.UDID, Kind: "simulator-device", Target: d.UDID},
			})
		}
	}
	return booted, nil
}

// ParseSimctlRuntimes marks a runtime unused-runtime only when none of its devices were ever booted.
func ParseSimctlRuntimes(raw []byte, rep *findings.Report, booted map[string]bool, volumesDir string) {
	parseSimctlRuntimes(raw, rep, booted, volumesDir, nil, size.NewScanner().Of)
}

func parseSimctlRuntimes(raw []byte, rep *findings.Report, booted map[string]bool, volumesDir string, images []simulator.Image, measure func(string) size.Result) {
	var doc simctlRuntimes
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	for _, rt := range doc.Runtimes {
		if rt.Identifier == "" {
			continue
		}
		risk := findings.RiskAsk
		why := "simctl runtime " + rt.Name
		if booted != nil && !booted[rt.Identifier] {
			risk = findings.RiskUnusedRuntime
			why = "runtime never booted identifier=" + rt.Identifier
		}
		p := rt.Identifier
		var bytes int64
		if vol := runtimeVolumePath(rt.RuntimeRoot, rt.Buildversion, rt.Name, volumesDir); volumesDir != "" && vol != "" {
			p = vol
			sz := measure(vol)
			if !sz.Missing && sz.Err == nil {
				bytes = sz.Allocated
			}
		}
		var matches []simulator.Image
		for _, img := range images {
			if img.RuntimeIdentifier == rt.Identifier || (img.RuntimeIdentifier == "" && rt.Buildversion != "" && img.Build == rt.Buildversion) {
				matches = append(matches, img)
			}
		}
		if len(matches) > 0 {
			for _, img := range matches {
				imagePath := img.Path
				if imagePath == "" {
					imagePath = "simulator-runtime:" + img.Identifier
				}
				imageBytes := img.SizeBytes
				var reclaimBytes *int64
				var apparent int64
				basis := "runtime-image-reported"
				if img.Path != "" {
					sz := measure(img.Path)
					if !sz.Missing && sz.Err == nil {
						imageBytes = sz.Allocated
						reclaimBytes = privateBytes(sz)
						apparent = sz.Apparent
						basis = ""
					}
				}
				var action *findings.Reclaim
				if img.State == "Ready" {
					action = &findings.Reclaim{Kind: "simulator-runtime", Target: img.Identifier, Cmd: "xcrun simctl runtime delete " + img.Identifier}
				}
				rep.Findings = append(rep.Findings, findings.Finding{ID: findings.IDSlug("sim-runtime", img.Identifier), Path: imagePath, Bytes: imageBytes, ReclaimBytes: reclaimBytes, BytesApparent: apparent, SizeBasis: basis, Category: "simulator-runtime", Risk: risk, Why: why + " disk image=" + img.Identifier + " state=" + img.State, LastUsed: img.LastUsedAt, Reclaim: action, SummaryOnly: action == nil})
			}
			continue
		}
		rep.Findings = append(rep.Findings, findings.Finding{
			ID:          findings.IDSlug("sim-runtime", rt.Identifier),
			Path:        p,
			Bytes:       bytes,
			Category:    "simulator-runtime",
			Risk:        risk,
			Why:         why,
			SummaryOnly: true,
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
