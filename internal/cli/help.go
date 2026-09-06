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

Modes (--quick is volume+known+tmp, NOT cargo-target-only):
  freedisk scan --quick --json
  freedisk scan --dev --json
  freedisk scan --mode=rust --json

Catalog (optional extras):
  freedisk catalog add PATH --scans quick,dev
  freedisk catalog unassign PATH --from quick
  freedisk catalog disable PATH
  freedisk catalog add '/tmp/proj-*' --glob --scans dev

Scan types:
  freedisk scans list
  freedisk scans disable artifacts
  freedisk scans add rust --types artifacts,worktrees

Delete (explicit ids only; never --all; never automatic):
  freedisk delete <id> [--yes]
  TTY types yes per id. Non-TTY requires --yes. Agents must not run delete unless
  the human named those ids. Refused: keep/never, git-tracked, busy
  npm/pnpm/yarn/cargo/uv, tmp roots, /System, ~/.cargo as a whole, /, $HOME.

JSON is default when stdout is not a TTY. Prefer --json when piping.

Exit codes: 0 report, 2 usage, 3 unknown id.

node_modules / vendor / target: listed; idle ≥ 30 days high-confidence.
/tmp /var/tmp $TMPDIR: children only, never the root.
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
