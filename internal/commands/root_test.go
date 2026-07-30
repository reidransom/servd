package commands

import (
	"strings"
	"testing"
)

func TestRootCommandDoesNotSupportScan(t *testing.T) {
	_, _, err := newRootCmd().Find([]string{"scan"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "scan"`) {
		t.Fatalf("Find(scan) error = %v, want unknown command", err)
	}
}
