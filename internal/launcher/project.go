package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/reidransom/servd/internal/config"
)

const projectConfigName = ".servd.toml"

// readProjectCmd reads only the registered root's repository command.
func readProjectCmd(dir string) (string, bool, error) {
	path := filepath.Join(dir, projectConfigName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cannot read repository command file %s: %w", path, err)
	}

	var values map[string]any
	if err := toml.Unmarshal(data, &values); err != nil {
		return "", false, fmt.Errorf("invalid repository command file %s: %w", path, err)
	}
	value, ok := values["cmd"]
	if !ok {
		return "", false, fmt.Errorf("repository command file %s has no cmd", path)
	}
	cmd, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("repository command file %s has a non-string cmd", path)
	}
	if strings.TrimSpace(cmd) == "" {
		return "", false, fmt.Errorf("repository command file %s has a blank cmd", path)
	}
	return cmd, true, nil
}

func missingCommandError(site config.Site) error {
	return fmt.Errorf("no command configured for %s\ncreate %s with a nonblank cmd, or run:\n  servd rm %s\n  servd add %s -- <command>", site.Path, filepath.Join(site.Path, projectConfigName), site.Slug, site.Path)
}
