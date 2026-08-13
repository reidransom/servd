package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestStringForDevelopmentBuild(t *testing.T) {
	got := stringFor(developmentVersion, unknownValue, unknownValue, nil, false)
	want := "servd version=dev commit=unknown date=unknown"
	if got != want {
		t.Fatalf("stringFor() = %q, want %q", got, want)
	}
}

func TestStringForUsesGoBuildInfoFallbacks(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
			{Key: "vcs.time", Value: "2026-08-12T10:11:12Z"},
		},
	}

	got := stringFor(developmentVersion, unknownValue, unknownValue, info, true)
	want := "servd version=v1.2.3 commit=0123456789abcdef date=2026-08-12T10:11:12Z"
	if got != want {
		t.Fatalf("stringFor() = %q, want %q", got, want)
	}
}

func TestStringForPrefersInjectedMetadata(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "source-commit"},
			{Key: "vcs.time", Value: "2026-01-01T00:00:00Z"},
		},
	}

	got := stringFor("v2.0.0", "release-commit", "2026-08-12T10:11:12Z", info, true)
	want := "servd version=v2.0.0 commit=release-commit date=2026-08-12T10:11:12Z"
	if got != want {
		t.Fatalf("stringFor() = %q, want %q", got, want)
	}
}
