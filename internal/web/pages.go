package web

import (
	"net/http"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/dashboard"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The two content pages. They are here rather than in internal/dashboard because a
// view model has no business knowing about HTTP, and a handler has no business
// deciding what a row means.
//
// Both read through seams the Server is given. That is not test-friendliness for its
// own sake: only the daemon's own process can answer "what is the config right now"
// and "who is logged in", and a handler that called those directly could only be
// tested on a machine that happens to have a config file and a desktop session. The CI
// runner has neither, so the tests would be skipped or would be asserting facts about
// the runner -- and a test that passes because of what the host occurs to contain also
// passes when the feature is broken.

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	sessions, err := s.readSessions()
	if err != nil {
		// The dashboard is still worth showing: which child is held and until when
		// does not depend on who is currently logged in. Saying nothing about the
		// missing column would leave a parent reading "locked out" as "the screen is
		// locked right now", which is the one thing that column means.
		s.logf("web: %s session list unavailable: %v", requestID(r), err)
		w.Header().Set("X-Plock-Note", "sessions-unavailable")
	}
	s.render(w, r, "dashboard", dashboard.Build(cfg, sessions, s.now()))
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	accounts, err := s.readAccounts()
	if err != nil {
		s.pageError(w, r, "the account list could not be read", err)
		return
	}
	configured := make([]users.ConfiguredUser, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		configured = append(configured, users.ConfiguredUser{
			Username: u.Username, ProfileID: u.ProfileID, Enabled: u.Enabled,
		})
	}
	// Union is computed once and reused. Calling it twice would be two answers to the
	// same question, and if they disagreed the count above and the table below would
	// be describing different machines.
	rows := users.Union(accounts, configured)
	s.render(w, r, "users", usersPage{
		Rows:   rows,
		Names:  profileNames(cfg),
		Gaps:   users.Unmanaged(rows),
		Manage: cfg.Settings.Enforcement,
	})
}

// usersPage is what users.tmpl is given.
//
// The profile names ride along because the attach control is a dropdown of profiles
// and a template cannot enumerate them: it only ever sees the row it is rendering.
type usersPage struct {
	Rows   []users.Managed
	Names  map[string]string
	Gaps   int
	Manage bool
}

// profileNames maps a profile id to the name the parent typed.
//
// By id rather than a slice, because the template looks one up per row and a linear
// scan inside a row loop is how a four-child household becomes quadratic.
func profileNames(cfg *config.Config) map[string]string {
	if cfg == nil {
		return nil
	}
	out := make(map[string]string, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		name := p.Name
		if name == "" {
			// An unnamed profile is possible in the document, and a dropdown that
			// renders an empty entry is indistinguishable from a broken control.
			name = p.ID
		}
		out[p.ID] = name
	}
	return out
}
