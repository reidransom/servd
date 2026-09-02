//go:build !windows

package proxy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
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

func TestSudoBindArgsPreserveCallerGroups(t *testing.T) {
	const worker = "/tmp/servd"
	args := sudoBindArgs(worker, config.Settings{}, []int{992, 998})
	for index, arg := range args {
		if arg != worker {
			continue
		}
		if index == 0 || args[index-1] != "-P" {
			t.Fatalf("sudo args = %q, want -P immediately before worker", args)
		}
		return
	}
	t.Fatalf("sudo args = %q, missing worker", args)
}
