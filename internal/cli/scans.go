package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func init() {
	AddCommand("scans", scansLong, runScans)
}

const scansLong = `scans list [--json]
scans disable TYPE
scans enable TYPE
scans add NAME --types TYPE[,TYPE]
scans remove NAME
`

var builtinPhases = []string{"volume", "known", "tmp", "drill", "toolchains", "artifacts", "worktrees", "apple-sim", "android-sim", "apis"}

func runScans(g *Global, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: scans needs a subcommand", ErrUsage)
	}
	path := catalog.OverlayPath(g.Config)
	ov, err := catalog.LoadOverlay(path)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		disabled := map[string]struct{}{}
		for _, d := range ov.DisableScans {
			disabled[d] = struct{}{}
		}
		type row struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		var rows []row
		for _, n := range builtinPhases {
			_, off := disabled[n]
			rows = append(rows, row{Name: n, Enabled: !off})
		}
		if wantJSON(g) || (len(args) > 1 && args[1] == "--json") {
			return json.NewEncoder(g.Stdout).Encode(rows)
		}
		for _, r := range rows {
			fmt.Fprintf(g.Stdout, "%s\tenabled=%v\n", r.Name, r.Enabled)
		}
		return nil
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("%w: scans disable TYPE", ErrUsage)
		}
		if args[1] == "volume" {
			return fmt.Errorf("%w: cannot disable volume", ErrUsage)
		}
		if !knownType(args[1]) {
			return fmt.Errorf("%w: unknown type %s", ErrUsage, args[1])
		}
		ov.DisableScans = append(ov.DisableScans, args[1])
		return catalog.SaveOverlay(path, ov)
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("%w: scans enable TYPE", ErrUsage)
		}
		var next []string
		for _, d := range ov.DisableScans {
			if d != args[1] {
				next = append(next, d)
			}
		}
		ov.DisableScans = next
		return catalog.SaveOverlay(path, ov)
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("%w: scans add NAME --types ...", ErrUsage)
		}
		name := args[1]
		if name == "quick" || name == "dev" || name == "full" || knownType(name) {
			return fmt.Errorf("%w: name collides with built-in", ErrUsage)
		}
		types := []string{}
		for i := 2; i < len(args); i++ {
			if args[i] == "--types" && i+1 < len(args) {
				types = strings.Split(args[i+1], ",")
				i++
			}
		}
		if len(types) == 0 {
			return fmt.Errorf("%w: --types required", ErrUsage)
		}
		if ov.Modes == nil {
			ov.Modes = map[string]catalog.UserMode{}
		}
		ov.Modes[name] = catalog.UserMode{Types: types}
		return catalog.SaveOverlay(path, ov)
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("%w: scans remove NAME", ErrUsage)
		}
		name := args[1]
		if name == "quick" || name == "dev" || name == "full" || name == "volume" {
			return fmt.Errorf("%w: cannot remove built-in", ErrUsage)
		}
		delete(ov.Modes, name)
		return catalog.SaveOverlay(path, ov)
	default:
		return fmt.Errorf("%w: unknown scans subcommand", ErrUsage)
	}
}

func knownType(n string) bool {
	for _, p := range builtinPhases {
		if p == n {
			return true
		}
	}
	for _, p := range scan.PhasesForTest() {
		if p.Name == n {
			return true
		}
	}
	return false
}
