//go:build windows

package launcher

import "strings"

// ShellQuote quotes one argument for cmd.exe with delayed expansion disabled.
// It combines cmd's percent escaping with the Windows argv quoting rules used
// by CommandLineToArgvW-compatible programs.
func ShellQuote(value string) string {
	value = strings.ReplaceAll(value, "%", "%%")

	var quoted strings.Builder
	quoted.Grow(len(value) + 2)
	quoted.WriteByte('"')
	backslashes := 0
	for _, character := range value {
		switch character {
		case '\\':
			backslashes++
		case '"':
			quoted.WriteString(strings.Repeat("\\", backslashes*2+1))
			quoted.WriteRune(character)
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat("\\", backslashes))
			quoted.WriteRune(character)
			backslashes = 0
		}
	}
	quoted.WriteString(strings.Repeat("\\", backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}

// ShellJoin renders argv as one command line for cmd.exe /c.
func ShellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for index, argument := range argv {
		parts[index] = ShellQuote(argument)
	}
	return strings.Join(parts, " ")
}
