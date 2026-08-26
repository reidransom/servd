// Package launcher resolves registered sites into concrete commands.
package launcher

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/reidransom/servd/internal/config"
)

// Resolved is a fully-resolved command specification.
type Resolved struct {
	Cmd    string // shell command line with {port}/{host} substituted
	Source string // explicit or .servd.toml
	Dir    string // working directory (the project path)
}

// Resolve produces the next command for a site.
func Resolve(site config.Site, settings config.Settings) (Resolved, error) {
	dir := site.Path
	info, err := os.Stat(dir)
	if err != nil {
		return Resolved{}, fmt.Errorf("project path %s is unavailable: %w", dir, err)
	}
	if !info.IsDir() {
		return Resolved{}, fmt.Errorf("project path %s is not a directory", dir)
	}

	if strings.TrimSpace(site.Cmd) != "" {
		return Resolved{Cmd: subst(site.Cmd, settings.BindHost, site.Port), Source: "explicit", Dir: dir}, nil
	}

	cmd, found, err := readProjectCmd(dir)
	if err != nil {
		return Resolved{}, err
	}
	if found {
		return Resolved{Cmd: subst(cmd, settings.BindHost, site.Port), Source: ".servd.toml", Dir: dir}, nil
	}
	return Resolved{}, missingCommandError(site)
}

// subst replaces {port} and {host} placeholders. Procfile $PORT/$HOST are left
// untouched here and resolved from the environment by the shell at run time.
func subst(cmd, host string, port int) string {
	cmd = strings.ReplaceAll(cmd, "{port}", strconv.Itoa(port))
	cmd = strings.ReplaceAll(cmd, "{host}", host)
	return cmd
}
