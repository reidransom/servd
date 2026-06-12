// Command servd runs and manages many local dev servers at once.
package main

import (
	"fmt"
	"os"

	"github.com/reidransom/servd/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "servd: "+err.Error())
		os.Exit(1)
	}
}
