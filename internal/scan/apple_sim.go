package scan

import (
	"encoding/json"
	"os/exec"
	"path/filepath"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func init() {
	Register(Phase{Name: "apple-sim", Quick: false, Dev: false, Run: runAppleSim})
}

// SimctlJSON is injected in tests.
var SimctlJSON = func() ([]byte, error) {
	return exec.Command("xcrun", "simctl", "list", "devices", "-j").Output()
}

// SimctlRuntimesJSON is injected in tests.
var SimctlRuntimesJSON = func() ([]byte, error) {
	return exec.Command("xcrun", "simctl", "list", "runtimes", "-j").Output()
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
		Identifier  string `json:"identifier"`
		Name        string `json:"name"`
		IsAvailable bool   `json:"isAvailable"`
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
		ParseSimctlRuntimes(rb, ctx.Report, booted)
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
			if deviceDir != "" && d.UDID != "" {
				p = filepath.Join(deviceDir, d.UDID)
			}
			rep.Findings = append(rep.Findings, findings.Finding{
				ID: "sim-" + d.UDID, Path: p, Bytes: 0,
				Category: "simulator-device", Risk: risk,
				LastUsed: d.LastBootedAt, Why: why,
				Reclaim: &findings.Reclaim{Cmd: "xcrun simctl delete " + d.UDID},
			})
		}
	}
	return booted, nil
}

// ParseSimctlRuntimes marks a runtime unused-runtime only when none of its devices were ever booted.
func ParseSimctlRuntimes(raw []byte, rep *findings.Report, booted map[string]bool) {
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
		rep.Findings = append(rep.Findings, findings.Finding{
			ID:       findings.IDSlug("sim-runtime", rt.Identifier),
			Path:     rt.Identifier,
			Bytes:    0,
			Category: "simulator-runtime",
			Risk:     risk,
			Why:      why,
			Reclaim:  &findings.Reclaim{Cmd: "xcrun simctl runtime delete " + rt.Identifier},
		})
	}
}
