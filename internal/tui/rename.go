package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/registration"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// handleRenameKey drives the rename modal and keeps a running site running
// under its new slug.
func (m *model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.status = ""
		return m, nil
	case "enter":
		newSlug := strings.TrimSpace(m.renameInput.Value())
		if newSlug == "" {
			m.status = "ERROR: a slug is required"
			return m, nil
		}

		oldSite := m.reg.Find(m.renameFrom)
		if oldSite == nil {
			m.status = "ERROR: unknown site " + m.renameFrom
			return m, nil
		}
		if newSlug == oldSite.Slug {
			m.mode = modeNormal
			m.status = "slug unchanged"
			return m, nil
		}

		preview := &config.Registry{Sites: append([]config.Site(nil), m.reg.Sites...)}
		if _, err := registration.RenameSite(preview, oldSite.Slug, newSlug); err != nil {
			m.status = "ERROR: " + firstLine(err.Error())
			return m, nil
		}

		site, settings := *oldSite, m.settings
		entry, exists := m.st.Get(site.Slug)
		wasRunning := exists && state.EntryAlive(entry)
		m.mode = modeNormal
		m.cmdCache = map[string]string{}
		m.cmdErrors = map[string]error{}
		return m.action("renaming "+site.Slug+" to "+newSlug+"…", func() actionDoneMsg {
			if wasRunning {
				if err := supervisor.Stop(site.Slug); err != nil {
					return actionDoneMsg{err: err}
				}
			}

			var renamed config.Site
			err := config.MutateRegistry(func(reg *config.Registry) error {
				var err error
				renamed, err = registration.RenameSite(reg, site.Slug, newSlug)
				return err
			})
			if err != nil {
				if wasRunning {
					if restartErr := supervisor.Start(site, settings); restartErr != nil {
						err = fmt.Errorf("%w; could not restart %s: %v", err, site.Slug, restartErr)
					}
				}
				return actionDoneMsg{err: err}
			}
			if wasRunning {
				err = supervisor.Start(renamed, settings)
			}
			return actionDoneMsg{verb: "renamed " + site.Slug + " to", slug: newSlug, err: err}
		})
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m *model) renameView() string {
	title := titleStyle.Render("rename site")
	field := dimStyle.Render("slug  ") + m.renameInput.View()
	hint := helpStyle.Render("enter rename · esc cancel")
	body := title + "\n\n" + field + "\n\n" + hint
	if strings.HasPrefix(m.status, "ERROR:") {
		body += "\n" + errStyle.Render(m.status)
	}
	modal := boxStyle.Width(max(40, m.width/2)).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
