// Package buildinfo resolves and formats servd's build metadata.
package buildinfo

import (
	"fmt"
	"runtime/debug"
)

const (
	developmentVersion = "dev"
	unknownValue       = "unknown"
)

// These values are replaced by release builds through linker flags.
var (
	Version = developmentVersion
	Commit  = unknownValue
	Date    = unknownValue
)

// String returns servd's version, commit, and build date on one stable line.
func String() string {
	info, ok := debug.ReadBuildInfo()
	return stringFor(Version, Commit, Date, info, ok)
}

func stringFor(version, commit, date string, info *debug.BuildInfo, ok bool) string {
	if version == "" {
		version = developmentVersion
	}
	if commit == "" {
		commit = unknownValue
	}
	if date == "" {
		date = unknownValue
	}

	if ok && info != nil {
		if version == developmentVersion && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == unknownValue && setting.Value != "" {
					commit = setting.Value
				}
			case "vcs.time":
				if date == unknownValue && setting.Value != "" {
					date = setting.Value
				}
			}
		}
	}

	return fmt.Sprintf("servd version=%s commit=%s date=%s", version, commit, date)
}
