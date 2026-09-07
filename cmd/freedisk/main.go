package main

import (
	"fmt"
	"os"

	"github.com/CiprianSpiridon/free-disk-space/internal/cli"
)

func main() {
	err := cli.Execute(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(cli.ExitCode(err))
}
