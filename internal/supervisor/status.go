package supervisor

import (
	"fmt"
	"strings"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/state"
)

const readinessWindow = 30 * time.Second

// SiteStatus is the shared health result for a registered site.
type SiteStatus struct {
	Kind   Status
	Reason string
}

// Evaluate inspects the launch configuration and the latest runtime attempt.
func Evaluate(site config.Site, settings config.Settings, st *state.State) SiteStatus {
	if _, err := launcher.Resolve(site, settings); err != nil {
		return SiteStatus{Kind: Error, Reason: oneLine(err.Error())}
	}

	entry, ok := st.Get(site.Slug)
	if !ok {
		return SiteStatus{Kind: Stopped}
	}
	if entry.Failure != "" {
		return SiteStatus{Kind: Error, Reason: oneLine(entry.Failure)}
	}
	if !state.EntryAlive(entry) {
		reason := "process exited"
		if entry.Log != "" {
			reason = fmt.Sprintf("%s (see %s)", reason, entry.Log)
		}
		return SiteStatus{Kind: Error, Reason: reason}
	}
	if netcheck.PortAccepting("127.0.0.1", site.Port) {
		return SiteStatus{Kind: Running}
	}
	if time.Since(entry.StartedAt) < readinessWindow {
		return SiteStatus{Kind: Starting}
	}
	return SiteStatus{Kind: Error, Reason: fmt.Sprintf("not accepting connections on :%d after %s", site.Port, readinessWindow)}
}

func oneLine(reason string) string {
	return strings.TrimSpace(strings.SplitN(reason, "\n", 2)[0])
}
