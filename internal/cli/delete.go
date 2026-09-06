package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/reclaim"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

func init() {
	AddCommand("delete", deleteLong, runDelete)
}

const deleteLong = `delete <id> [<id>...] [--yes]

ONLY mutate path. Explicit finding ids from the last scan. No --all.
TTY confirms each id (type yes). Non-TTY requires --yes. Never automatic.
keep/never, git-tracked, busy package managers, tmp roots, and system paths are refused.
`

func runDelete(g *Global, args []string) error {
	yes := false
	var ids []string
	for _, a := range args {
		switch a {
		case "--yes":
			yes = true
		case "--all":
			return fmt.Errorf("%w: delete --all is not allowed", ErrUsage)
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("%w: unknown flag %s", ErrUsage, a)
			}
			ids = append(ids, a)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: delete requires finding ids", ErrUsage)
	}
	if !g.IsTTY && !yes {
		return fmt.Errorf("%w: non-TTY delete requires --yes", ErrUsage)
	}
	r, err := scan.ReadLastScan(scan.LastScanPath())
	if err != nil {
		return err
	}
	in := bufio.NewReader(g.Stdin)
	for _, id := range ids {
		f, ok := reclaim.Lookup(r, id)
		if !ok {
			return fmt.Errorf("%w: %s", errUnknownID, id)
		}
		if g.IsTTY && !yes {
			fmt.Fprintf(g.Stdout, "Type yes to delete %s %s (%d bytes): ", f.ID, f.Path, f.Bytes)
			line, err := in.ReadString('\n')
			if err != nil {
				return fmt.Errorf("delete aborted: %w", err)
			}
			if strings.TrimSpace(line) != "yes" {
				return fmt.Errorf("delete aborted")
			}
		}
		if err := reclaim.ApplyOne(f); err != nil {
			return err
		}
		fmt.Fprintf(g.Stdout, "deleted %s %s (%d bytes)\n", f.ID, f.Path, f.Bytes)
	}
	return nil
}
