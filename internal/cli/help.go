package cli

import (
	"fmt"
	"strings"
)

func init() {
	AddCommand("help", "help [command]", runHelp)
}

const agentGuide = `freedisk — report what is eating disk on this Mac. Scan never deletes.

Starting point: no overlay required. Bundled catalog covers caches, SDKs, tmp,
Desktop/Downloads, generic work roots, and auto-discovered project folders.

Modes (--quick is volume+known+home depth-1+tmp, NOT cargo-target-only):
  freedisk scan --quick --json
  freedisk scan --dev --json
  freedisk scan --mode=rust --json

Expected time (developer Mac with large work trees / caches; wall clock):
  --quick   1-5 min typical, up to ~10 min if home has a 100GB+ work dir
  --dev     3-10 min (quick + artifacts + worktrees)
  full      5-15 min (--dev + drill budget ~3 min + simctl/android/docker/brew)
A quiet process is not hung: stderr prints "freedisk: phase ..." and
"still walking ...". Do not kill while those lines appear. Small/empty
disks finish in tens of seconds. Prefer --quick first; only run full if
you need simulators or docker/brew.

Catalog overlay (this Mac; not delete). Bundled YAML is generic. After a
scan, add paths the user cares about that were missing, or drop paths they
do not use. Overlay: ~/.config/freedisk/catalog.yaml (see catalog path).
  freedisk catalog add PATH --scans quick,dev --risk ask --category user
  freedisk catalog add '/Users/you/work_*' --glob --scans dev
  freedisk catalog unassign PATH --from quick
  freedisk catalog disable PATH
  freedisk catalog list [--json]
Never edit the bundled catalog. Re-scan after overlay changes.

Scan types:
  freedisk scans list
  freedisk scans disable artifacts
  freedisk scans add rust --types artifacts,worktrees

Skill (install into every local agent CLI):
  freedisk skill
  freedisk skill install
  freedisk skill list

Delete (explicit ids only; never --all; never automatic):
  freedisk delete <id> [--yes]
  TTY types yes per id. Non-TTY requires --yes. Agents must not run delete unless
  the human named those ids. Refused: keep/never, git-tracked, busy
  npm/pnpm/yarn/cargo/uv, tmp roots, /System, ~/.cargo as a whole, /, $HOME.

Markdown tables list id, full path, last used, and a reclaim command
(catalog command, or rm -rf PATH). Never guess a path from a truncated id.
Progress goes to stderr (freedisk: phase ...); --quiet silences it.
freedisk scan (full) time-budgets drill and still prints the report — it
does not abort with "drill timed out".

JSON is default when stdout is not a TTY. Prefer --json when piping.

Exit codes: 0 report, 2 usage, 3 unknown id.

node_modules / vendor / target: listed; idle ≥ 30 days high-confidence.
/tmp /private/tmp /var/tmp $TMPDIR: children only (e.g. /private/tmp/kensi-*), never the root.
Idle ≥ 7 days or ≥ 1 GiB tmp children are high-confidence reclaimable.
`

func ensureHelp() { lookup("help") }

func runHelp(g *Global, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(g.Stdout, agentGuide)
		fmt.Fprintln(g.Stdout)
		fmt.Fprintln(g.Stdout, "commands:", strings.Join(CommandNames(), ", "))
		return nil
	}
	name := args[0]
	long := CommandLong(name)
	if long == "" && lookup(name) == nil {
		return fmt.Errorf("%w: unknown help topic %s", ErrUsage, name)
	}
	if long == "" {
		long = name
	}
	fmt.Fprintln(g.Stdout, long)
	return nil
}
