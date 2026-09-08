package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/report"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func init() {
	AddCommand("history", historyLong, runHistory)
}

const historyLong = `history [list] [--json] [--type MODE]
history show <id|latest|N> [--json]

Each scan writes a timestamped copy under
$XDG_CACHE_HOME/freedisk/history/YYYYMMDDTHHMMSSZ-MODE.json
(or ~/.cache/freedisk/history/). Files are 0600.

list   newest first. --type quick|dev|full (or a user mode) filters.
show   reprint one archived report as markdown or JSON.
       id is the filename stem, latest, a unique prefix, or 1-based index
       from list (1 = newest).

History is display-only. why and delete still use last-scan.json (24h).
Never deletes user files.
`

func runHistory(g *Global, args []string) error {
	if len(args) == 0 {
		return historyList(g, nil)
	}
	switch args[0] {
	case "list":
		return historyList(g, args[1:])
	case "show":
		return historyShow(g, args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return historyList(g, args)
		}
		return fmt.Errorf("%w: unknown history subcommand %s", ErrUsage, args[0])
	}
}

func historyList(g *Global, args []string) error {
	typ := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			g.JSON = true
		case args[i] == "--type" && i+1 < len(args):
			typ = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--type="):
			typ = strings.TrimPrefix(args[i], "--type=")
		case args[i] == "--mode" && i+1 < len(args):
			typ = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--mode="):
			typ = strings.TrimPrefix(args[i], "--mode=")
		default:
			return fmt.Errorf("%w: unknown flag %s", ErrUsage, args[i])
		}
	}
	list, err := scan.ListHistoryDir(scan.HistoryDir())
	if err != nil {
		return err
	}
	if typ != "" {
		var filtered []scan.HistoryEntry
		for _, e := range list {
			if e.Mode == typ {
				filtered = append(filtered, e)
			}
		}
		list = filtered
	}
	if len(list) == 0 {
		if typ != "" {
			fmt.Fprintf(g.Stdout, "no historic %s reports; run scan first\n", typ)
		} else {
			fmt.Fprintln(g.Stdout, "no historic reports; run scan first")
		}
		return nil
	}
	if wantJSON(g) {
		return json.NewEncoder(g.Stdout).Encode(list)
	}
	for _, e := range list {
		fmt.Fprintf(g.Stdout, "%s\t%s\t%s\t%d findings\t%s in use\t%s free\n",
			e.ID, e.Mode, e.GeneratedAt, e.Findings, histSize(e.InUseBytes), histSize(e.FreeBytes))
	}
	return nil
}

func historyShow(g *Global, args []string) error {
	spec := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			g.JSON = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("%w: unknown flag %s", ErrUsage, args[i])
		default:
			if spec != "" {
				return fmt.Errorf("%w: history show takes one id", ErrUsage)
			}
			spec = args[i]
		}
	}
	if spec == "" {
		return fmt.Errorf("%w: history show <id|latest|N>", ErrUsage)
	}
	ent, err := scan.LookupHistory(scan.HistoryDir(), spec)
	if err != nil {
		return err
	}
	r, err := scan.ReadReportFile(ent.Path)
	if err != nil {
		return err
	}
	if wantJSON(g) {
		return report.JSON(g.Stdout, r)
	}
	return report.Markdown(g.Stdout, r, report.Options{})
}

func histSize(n int64) string {
	const k = 1024.0
	f := float64(n)
	switch {
	case f >= k*k*k:
		return fmt.Sprintf("%.1f GiB", f/(k*k*k))
	case f >= k*k:
		return fmt.Sprintf("%.1f MiB", f/(k*k))
	case f >= k:
		return fmt.Sprintf("%.1f KiB", f/k)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
