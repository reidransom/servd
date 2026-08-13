//go:build windows

package launcher

import "testing"

func TestShellJoin(t *testing.T) {
	got := ShellJoin([]string{`C:\Program Files\servd.exe`, "__static", "a&b", "%TEMP%"})
	want := `"C:\Program Files\servd.exe" "__static" "a&b" "%%TEMP%%"`
	if got != want {
		t.Fatalf("ShellJoin() = %q, want %q", got, want)
	}
}

func TestShellQuoteEscapesTrailingBackslashes(t *testing.T) {
	got := ShellQuote(`C:\sites\`)
	want := `"C:\sites\\"`
	if got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}
