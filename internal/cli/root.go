package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CiprianSpiridon/free-disk-space/internal/version"
)

// ErrUsage is exit 2.
var ErrUsage = errors.New("usage")

// Global flags.
type Global struct {
	JSON    bool
	Catalog string
	Config  string
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Args    []string
	IsTTY   bool
}

type command struct {
	name  string
	run   func(g *Global, args []string) error
	long  string
}

var commands []command

// AddCommand registers a subcommand.
func AddCommand(name string, long string, run func(g *Global, args []string) error) {
	commands = append(commands, command{name: name, long: long, run: run})
}

func lookup(name string) *command {
	for i := range commands {
		if commands[i].name == name {
			return &commands[i]
		}
	}
	return nil
}

func CommandLong(name string) string {
	c := lookup(name)
	if c == nil {
		return ""
	}
	return c.long
}

func CommandNames() []string {
	var s []string
	for _, c := range commands {
		s = append(s, c.name)
	}
	return s
}

func parseGlobal(args []string) (*Global, []string, error) {
	g := &Global{Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin}
	if fi, err := os.Stdout.Stat(); err == nil {
		g.IsTTY = fi.Mode()&os.ModeCharDevice != 0
	}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			g.JSON = true
		case a == "--catalog" && i+1 < len(args):
			i++
			g.Catalog = args[i]
		case strings.HasPrefix(a, "--catalog="):
			g.Catalog = strings.TrimPrefix(a, "--catalog=")
		case a == "--config" && i+1 < len(args):
			i++
			g.Config = args[i]
		case strings.HasPrefix(a, "--config="):
			g.Config = strings.TrimPrefix(a, "--config=")
		case a == "--version" || a == "version":
			rest = append([]string{"version"}, args[i+1:]...)
			return g, rest, nil
		case a == "--help" || a == "-h":
			rest = append([]string{"help"}, args[i+1:]...)
			return g, rest, nil
		default:
			rest = append(rest, args[i:]...)
			return g, rest, nil
		}
	}
	return g, rest, nil
}

func wantJSON(g *Global) bool {
	return g.JSON || !g.IsTTY
}

// Execute is the CLI entrypoint.
func Execute(args []string) error {
	if len(args) == 0 {
		args = []string{"help"}
	}
	g, rest, err := parseGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return runHelp(g, nil)
	}
	name, rest := rest[0], rest[1:]
	if name == "version" {
		fmt.Fprintln(g.Stdout, version.Version)
		return nil
	}
	c := lookup(name)
	if c == nil {
		fmt.Fprintf(g.Stderr, "unknown command %q\n", name)
		return fmt.Errorf("%w: unknown command %s", ErrUsage, name)
	}
	g.Args = rest
	return c.run(g, rest)
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrUsage) {
		return 2
	}
	if errors.Is(err, errUnknownID) {
		return 3
	}
	return 1
}

var errUnknownID = errors.New("unknown id")
