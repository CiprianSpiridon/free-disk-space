package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/catalog"
)

func init() {
	AddCommand("catalog", catalogLong, runCatalog)
}

const catalogLong = `catalog add PATH [--scans quick,dev,full] [--risk ask --category user] [--glob]
catalog unassign PATH --from MODE[,MODE]
catalog disable PATH
catalog list [--json]
catalog path
`

func runCatalog(g *Global, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: catalog needs a subcommand", ErrUsage)
	}
	home, _ := os.UserHomeDir()
	path := catalog.OverlayPath(g.Config)
	ov, err := catalog.LoadOverlay(path)
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(g.Stdout, path)
		return nil
	case "list":
		b, _, err := catalog.LoadDefault(g.Catalog)
		if err != nil {
			return err
		}
		m := catalog.Merge(b, ov, home)
		if wantJSON(g) || (len(args) > 1 && args[1] == "--json") {
			paths := make([]string, 0)
			for _, e := range m.AllEntries() {
				paths = append(paths, e.Path)
			}
			return json.NewEncoder(g.Stdout).Encode(paths)
		}
		for _, e := range m.AllEntries() {
			fmt.Fprintf(g.Stdout, "%s\t%s\t%s\n", e.Path, e.Category, strings.Join(e.Scans, ","))
		}
		return nil
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("%w: catalog add PATH", ErrUsage)
		}
		e := catalog.Entry{Path: args[1], Risk: "ask", Category: "user"}
		for i := 2; i < len(args); i++ {
			switch {
			case args[i] == "--glob":
				e.Glob = true
			case strings.HasPrefix(args[i], "--scans="):
				e.Scans = strings.Split(strings.TrimPrefix(args[i], "--scans="), ",")
			case args[i] == "--scans" && i+1 < len(args):
				i++
				e.Scans = strings.Split(args[i], ",")
			case strings.HasPrefix(args[i], "--risk="):
				e.Risk = strings.TrimPrefix(args[i], "--risk=")
			case args[i] == "--risk" && i+1 < len(args):
				i++
				e.Risk = args[i]
			case strings.HasPrefix(args[i], "--category="):
				e.Category = strings.TrimPrefix(args[i], "--category=")
			case args[i] == "--category" && i+1 < len(args):
				i++
				e.Category = args[i]
			}
		}
		ov.Add = append(ov.Add, e)
		return catalog.SaveOverlay(path, ov)
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("%w: catalog disable PATH", ErrUsage)
		}
		ov.Disable = append(ov.Disable, args[1])
		return catalog.SaveOverlay(path, ov)
	case "unassign":
		from := []string{}
		p := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--from" && i+1 < len(args) {
				from = strings.Split(args[i+1], ",")
				i++
				continue
			}
			if strings.HasPrefix(args[i], "--from=") {
				from = strings.Split(strings.TrimPrefix(args[i], "--from="), ",")
				continue
			}
			p = args[i]
		}
		if p == "" || len(from) == 0 {
			return fmt.Errorf("%w: catalog unassign PATH --from MODE", ErrUsage)
		}
		for _, m := range from {
			if m != "quick" && m != "dev" && m != "full" {
				return fmt.Errorf("%w: unknown mode %s", ErrUsage, m)
			}
		}
		ov.Unassign = append(ov.Unassign, catalog.Unassign{Path: p, From: from})
		return catalog.SaveOverlay(path, ov)
	default:
		return fmt.Errorf("%w: unknown catalog subcommand", ErrUsage)
	}
}
