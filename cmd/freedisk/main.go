package main

import (
	"os"

	"github.com/CiprianSpiridon/free-disk-space/internal/cli"
)

func main() {
	err := cli.Execute(os.Args[1:])
	os.Exit(cli.ExitCode(err))
}
