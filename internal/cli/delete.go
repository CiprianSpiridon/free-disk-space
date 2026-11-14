package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/findings"
	"github.com/CiprianSpiridon/free-disk-space/internal/reclaim"
	"github.com/CiprianSpiridon/free-disk-space/internal/scan"
)

var measureDeleteVolume = scan.ReadVolume

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
	before, beforeErr := measureDeleteVolume()
	var failures []error
	var requestedBytes, completedBytes int64
	defer func() {
		after, afterErr := measureDeleteVolume()
		if beforeErr == nil && afterErr == nil && before.ContainerID == after.ContainerID {
			fmt.Fprintf(g.Stdout, "Unallocated container space before: %d bytes; after: %d bytes; measured change: %+d bytes; requested row sizes: %d bytes; completed row sizes: %d bytes\n", before.FreeBytes, after.FreeBytes, after.FreeBytes-before.FreeBytes, requestedBytes, completedBytes)
			fmt.Fprintln(g.Stdout, "Measured container change includes concurrent disk activity; row sizes may share blocks and are not reclaimed space.")
		} else {
			fmt.Fprintf(g.Stdout, "Unallocated container space comparison unavailable (before: %v; after: %v); completed row sizes: %d bytes\n", beforeErr, afterErr, completedBytes)
		}
	}()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		f, ok := reclaim.Lookup(r, id)
		if !ok {
			failures = append(failures, fmt.Errorf("%w: %s", errUnknownID, id))
			continue
		}
		requestedBytes += f.Bytes
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
			failures = append(failures, fmt.Errorf("%s: %w", id, err))
			fmt.Fprintf(g.Stdout, "failed %s: %v\n", id, err)
			continue
		}
		completedBytes += f.Bytes
		var keep []findings.Finding
		for _, x := range r.Findings {
			if x.ID != id {
				keep = append(keep, x)
			}
		}
		r.Findings = keep
		if err := scan.WriteLastScan(scan.LastScanPath(), r); err != nil {
			failures = append(failures, fmt.Errorf("save completed deletion %s: %w", id, err))
		}
		fmt.Fprintf(g.Stdout, "deleted %s %s (%d bytes)\n", f.ID, f.Path, f.Bytes)
	}

	return errors.Join(failures...)
}
