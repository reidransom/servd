//go:build !windows

package proxy

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmPortlessMode(t *testing.T) {
	tests := []struct {
		answer string
		want   bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"\n", false},
		{"unexpected\n", false},
	}

	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.answer), func(t *testing.T) {
			var output bytes.Buffer
			got, err := confirmPortlessMode(strings.NewReader(tt.answer), &output)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("confirmation = %t, want %t", got, tt.want)
			}
			const prompt = "Use port-less mode (requires root password)? [y/N] "
			if output.String() != prompt {
				t.Fatalf("prompt = %q, want %q", output.String(), prompt)
			}
		})
	}
}
