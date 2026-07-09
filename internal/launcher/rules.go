package launcher

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/reidransom/servd/internal/config"
)

// Rule is one declarative launcher: predicates that recognize a project
// directory, paired with the command that serves it. Rules live in TOML —
// the built-in defaults are embedded from defaults.toml, and users can extend
// or override them in ~/.config/servd/launchers.toml (same format).
type Rule struct {
	// Name identifies the rule. A user rule with the same name as a built-in
	// replaces that built-in.
	Name string `toml:"name"`
	// Kind is recorded as the site's launcher kind and defaults to Name. It
	// lets several rules (e.g. a fallback variant) present as one kind.
	Kind string `toml:"kind,omitempty"`
	// Disabled turns the rule off. A disabled user rule named after a
	// built-in removes that built-in.
	Disabled bool `toml:"disabled,omitempty"`

	// Predicates. All present keys must hold (AND); list values match any-of.
	// At least one directory-inspecting predicate (exists, dirs, recipe,
	// matches, npm_script) is required — bin alone would claim every project.
	Exists    []string      `toml:"exists,omitempty"`     // files, globs allowed
	Dirs      []string      `toml:"dirs,omitempty"`       // directories, globs allowed
	Bin       string        `toml:"bin,omitempty"`        // binary resolvable on PATH
	Recipe    *RecipeMatch  `toml:"recipe,omitempty"`     // make/just target declaration
	Matches   *ContentMatch `toml:"matches,omitempty"`    // file content regex
	NpmScript []string      `toml:"npm_script,omitempty"` // package.json script names

	// Cmd is the launch command template. {port} and {host} are substituted
	// at launch time, {script} is the matched npm_script entry, and {self} is
	// the shell-quoted path to the servd binary itself.
	Cmd string `toml:"cmd"`
	// PortFlag is appended to Cmd when any of PortFlagDeps appears in
	// package.json dependencies/devDependencies — for dev servers that ignore
	// $PORT and need an explicit flag. Only meaningful with npm_script.
	PortFlag     string   `toml:"port_flag,omitempty"`
	PortFlagDeps []string `toml:"port_flag_deps,omitempty"`
}

// RecipeMatch matches a make/just-style file declaring a target.
type RecipeMatch struct {
	Files  []string `toml:"files"`  // candidate files, any-of
	Target string   `toml:"target"` // target/recipe name that must be declared
}

// ContentMatch matches a file whose content matches a regex.
type ContentMatch struct {
	File  string `toml:"file"`
	Regex string `toml:"regex"`
}

// valid reports whether the rule is well-formed enough to try.
func (r Rule) valid() bool {
	if r.Name == "" || r.Cmd == "" {
		return false
	}
	return len(r.Exists) > 0 || len(r.Dirs) > 0 || r.Recipe != nil ||
		r.Matches != nil || len(r.NpmScript) > 0
}

// match reports whether the rule claims dir, returning the command (still
// containing {port}/{host} placeholders) and the launcher kind.
func (r Rule) match(dir string) (cmd, kind string, ok bool) {
	if r.Disabled || !r.valid() {
		return "", "", false
	}
	if r.Bin != "" && !onPath(r.Bin) {
		return "", "", false
	}
	if len(r.Exists) > 0 && !anyGlob(dir, r.Exists, false) {
		return "", "", false
	}
	if len(r.Dirs) > 0 && !anyGlob(dir, r.Dirs, true) {
		return "", "", false
	}
	if r.Recipe != nil && !r.Recipe.match(dir) {
		return "", "", false
	}
	if r.Matches != nil && !r.Matches.match(dir) {
		return "", "", false
	}
	cmd, kind = r.Cmd, r.Kind
	if kind == "" {
		kind = r.Name
	}
	if len(r.NpmScript) > 0 {
		pkg, err := readPackageJSON(dir)
		if err != nil {
			return "", "", false
		}
		script := ""
		for _, s := range r.NpmScript {
			if _, has := pkg.Scripts[s]; has {
				script = s
				break
			}
		}
		if script == "" {
			return "", "", false
		}
		cmd = strings.ReplaceAll(cmd, "{script}", script)
		kind += ":" + script
		if r.PortFlag != "" && hasAnyDep(pkg, r.PortFlagDeps) {
			cmd += r.PortFlag
		}
	}
	cmd = strings.ReplaceAll(cmd, "{self}", selfExe())
	return cmd, kind, true
}

func (m *RecipeMatch) match(dir string) bool {
	if m.Target == "" {
		return false
	}
	for _, f := range m.Files {
		if hasRecipe(filepath.Join(dir, f), m.Target) {
			return true
		}
	}
	return false
}

func (m *ContentMatch) match(dir string) bool {
	if m.File == "" || m.Regex == "" {
		return false
	}
	re, err := regexp.Compile(m.Regex)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, m.File))
	if err != nil {
		return false
	}
	return re.Match(data)
}

// matchRules tries rules in order; the first that claims dir wins.
func matchRules(dir string, rules []Rule) (cmd, kind string, ok bool) {
	for _, r := range rules {
		if cmd, kind, ok := r.match(dir); ok {
			return cmd, kind, true
		}
	}
	return "", "", false
}

// rulesFile is the on-disk shape shared by defaults.toml and launchers.toml.
type rulesFile struct {
	Launchers []Rule `toml:"launcher"`
}

//go:embed defaults.toml
var defaultsTOML []byte

var builtinRules = sync.OnceValue(func() []Rule {
	var f rulesFile
	if err := toml.Unmarshal(defaultsTOML, &f); err != nil {
		panic("servd: embedded defaults.toml is invalid: " + err.Error())
	}
	return f.Launchers
})

// userRulesPath is ~/.config/servd/launchers.toml.
func userRulesPath() string { return filepath.Join(config.ConfigDir(), "launchers.toml") }

var userRulesWarn sync.Once

// loadUserRules reads launchers.toml; a missing file yields nil. A malformed
// file is skipped with a one-time warning rather than failing resolution.
func loadUserRules() []Rule {
	data, err := os.ReadFile(userRulesPath())
	if err != nil {
		return nil
	}
	var f rulesFile
	if err := toml.Unmarshal(data, &f); err != nil {
		userRulesWarn.Do(func() {
			fmt.Fprintf(os.Stderr, "servd: ignoring %s: %v\n", userRulesPath(), err)
		})
		return nil
	}
	return f.Launchers
}

// EffectiveRules returns the launcher rules in the order they are tried:
// user rules from launchers.toml first, then the built-in defaults minus any
// the user overrode by name — so user rules always win. Disabled rules stay
// in the list (match rejects them) so `servd launchers` can show them.
func EffectiveRules() []Rule {
	user := loadUserRules()
	overridden := map[string]bool{}
	for _, r := range user {
		overridden[r.Name] = true
	}
	out := slices.Clone(user)
	for _, r := range builtinRules() {
		if !overridden[r.Name] {
			out = append(out, r)
		}
	}
	return out
}

// MarshalRules renders rules in launchers.toml format (used by `servd
// launchers` so the output can be pasted straight into the user file).
func MarshalRules(rules []Rule) ([]byte, error) {
	return toml.Marshal(rulesFile{Launchers: rules})
}

// plainWord matches a bare executable name (no shell syntax, no placeholder).
var plainWord = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Tools returns the unique executables the rules depend on — bin requirements
// plus plain leading command words — in first-seen order (used by doctor).
func Tools(rules []Rule) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && plainWord.MatchString(name) && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, r := range rules {
		add(r.Bin)
		if fields := strings.Fields(r.Cmd); len(fields) > 0 {
			add(fields[0])
		}
	}
	return out
}
