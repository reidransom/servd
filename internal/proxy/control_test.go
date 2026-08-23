package proxy

import (
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestWorkerArgsPassLANFlagAndPort(t *testing.T) {
	settings := config.DefaultSettings()
	settings.EnableLAN()
	settings.Hostnames.HTTPPort = 80

	got := workerArgs(settings.Hostnames.HTTPPort, settings.Hostnames.LAN)
	want := []string{"__proxy-worker", "--port", "80", "--lan"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("worker args = %q, want %q", got, want)
	}
}
func TestStartWithPortPolicy(t *testing.T) {
	permissionDenied := &net.OpError{Err: os.ErrPermission}
	occupied := &net.OpError{Err: syscall.EADDRINUSE}
	canceled := errors.New("sudo authentication canceled")

	tests := []struct {
		name          string
		configPresent bool
		configured    int
		direct        map[int]error
		elevated      error
		wantPort      int
		wantElevate   bool
		wantFallback  bool
		wantErr       string
	}{
		{"first run binds 80 directly", false, 8080, map[int]error{}, nil, 80, false, false, ""},
		{"first run elevates 80", false, 8080, map[int]error{80: permissionDenied}, nil, 80, true, false, ""},
		{"first run falls back after canceled elevation", false, 8080, map[int]error{80: permissionDenied}, canceled, 8080, true, true, ""},
		{"first run falls back when 80 is occupied", false, 8080, map[int]error{80: occupied}, nil, 8080, false, true, ""},
		{"first run reports both failed ports", false, 8080, map[int]error{80: occupied, 8080: errors.New("fallback occupied")}, nil, 0, false, false, "80"},
		{"configured 80 does not fall back", true, 80, map[int]error{80: permissionDenied}, canceled, 0, true, false, "configured proxy port 80"},
		{"configured 8080 avoids elevation", true, 8080, map[int]error{}, nil, 8080, false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var elevationCalls int
			direct := func(port int) error { return tt.direct[port] }
			elevate := func(int) error {
				elevationCalls++
				return tt.elevated
			}
			var result StartResult
			var err error
			if tt.configPresent {
				result, err = startConfiguredPort(tt.configured, direct, elevate)
			} else {
				result, err = startFirstRunPort(direct, elevate)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Port != tt.wantPort || result.UsedFallback != tt.wantFallback {
				t.Fatalf("result = %+v, want port %d fallback %t", result, tt.wantPort, tt.wantFallback)
			}
			if got := elevationCalls > 0; got != tt.wantElevate {
				t.Fatalf("elevation called = %t, want %t", got, tt.wantElevate)
			}
		})
	}
}
