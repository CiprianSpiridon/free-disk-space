package scan

import (
	"encoding/json"
	"os/exec"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
)

func init() {
	Register(Phase{Name: "apple-sim", Quick: false, Dev: false, Run: runAppleSim})
}

// SimctlJSON is injected in tests.
var SimctlJSON = func() ([]byte, error) {
	return exec.Command("xcrun", "simctl", "list", "devices", "-j").Output()
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
	ParseSimctlDevices(b, ctx.Report)
	return nil
}

// ParseSimctlDevices joins on runtimeIdentifier, not display name.
func ParseSimctlDevices(raw []byte, rep *findings.Report) {
	var doc simctlDevices
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	for group, devs := range doc.Devices {
		for _, d := range devs {
			risk := findings.RiskAsk
			if d.LastBootedAt == "" {
				risk = findings.RiskUnusedRuntime
			}
			why := "runtimeIdentifier=" + d.RuntimeIdentifier + " group=" + group
			rep.Findings = append(rep.Findings, findings.Finding{
				ID: "sim-" + d.UDID, Path: d.UDID, Bytes: 0,
				Category: "simulator-device", Risk: risk,
				LastUsed: d.LastBootedAt, Why: why,
				Reclaim: &findings.Reclaim{Cmd: "xcrun simctl delete " + d.UDID},
			})
		}
	}
}
