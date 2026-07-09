package launcher

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// projectConfigName is the per-project override file. A project that knows
// how to serve itself declares it once, next to its code:
//
//	cmd = "bundle exec middleman serve -p {port}"
//
// It beats Procfile and all launcher rules; only a `servd add --cmd` pin on
// the site ranks higher.
const projectConfigName = ".servd.toml"

// readProjectCmd returns the cmd from dir/.servd.toml, if one is declared.
// A missing file, unparsable file, or empty cmd all fall through to the next
// resolution step.
func readProjectCmd(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, projectConfigName))
	if err != nil {
		return "", false
	}
	var pc struct {
		Cmd string `toml:"cmd"`
	}
	if err := toml.Unmarshal(data, &pc); err != nil || strings.TrimSpace(pc.Cmd) == "" {
		return "", false
	}
	return pc.Cmd, true
}
