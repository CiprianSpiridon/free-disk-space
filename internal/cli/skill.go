package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/CiprianSpiridon/free-disk-space/internal/skill"
)

func init() {
	AddCommand("skill", skillLong, runSkill)
}

const skillLong = `skill
skill print
skill list [--json]
skill install [--json]
skill uninstall [--json]

Print or install the bundled agent skill into every available CLI
(Claude, Codex, Cursor, Grok, OpenCode, Kiro, Gemini, Factory, Goose, Continue, Windsurf, shared ~/.agents).
`

func runSkill(g *Global, args []string) error {
	home, _ := os.UserHomeDir()
	sub := "print"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	for _, a := range args {
		if a == "--json" {
			g.JSON = true
		} else if a != "" && a[0] == '-' {
			return fmt.Errorf("%w: unknown flag %s", ErrUsage, a)
		}
	}
	switch sub {
	case "print", "show", "cat":
		_, err := g.Stdout.Write(skill.Markdown())
		return err
	case "list":
		res := skill.List(home, nil)
		return writeSkillResults(g, res)
	case "install":
		res := skill.Install(home, nil)
		return writeSkillResults(g, res)
	case "uninstall":
		res := skill.Uninstall(home, nil)
		return writeSkillResults(g, res)
	default:
		return fmt.Errorf("%w: unknown skill subcommand %s", ErrUsage, sub)
	}
}

func writeSkillResults(g *Global, res []skill.Result) error {
	if wantJSON(g) {
		enc := json.NewEncoder(g.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	n, skip, fail := 0, 0, 0
	for _, r := range res {
		switch {
		case r.Error != "":
			fmt.Fprintf(g.Stdout, "error  %-10s %s: %s\n", r.Tool, r.Path, r.Error)
			fail++
		case r.Skipped:
			fmt.Fprintf(g.Stdout, "skip   %-10s %s\n", r.Tool, r.Reason)
			skip++
		default:
			fmt.Fprintf(g.Stdout, "ok     %-10s %s\n", r.Tool, r.Path)
			n++
		}
	}
	fmt.Fprintf(g.Stdout, "%d ok, %d skipped, %d errors\n", n, skip, fail)
	return nil
}
