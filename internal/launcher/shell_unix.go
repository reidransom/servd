//go:build !windows

package launcher

import (
	"regexp"
	"strings"
)

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./{}-]+$`)

// ShellQuote quotes one argument for sh.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// ShellJoin renders argv as one command line for sh -c.
func ShellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for index, argument := range argv {
		if shellSafe.MatchString(argument) {
			parts[index] = argument
		} else {
			parts[index] = ShellQuote(argument)
		}
	}
	return strings.Join(parts, " ")
}
