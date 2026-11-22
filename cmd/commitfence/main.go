// Command commitfence checks local PL/SQL transaction behavior.
package main

import (
	"os"

	"commitfence/internal/cli"
)

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
