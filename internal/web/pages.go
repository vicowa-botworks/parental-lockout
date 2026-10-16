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

// csrfCarrier is the seam between render and a page that posts something back.
//
// render already knows the token -- it looked it up to put the meta tag in the shell -- so a
// page body should not have to ask for it a second time, and a page that forgets to would
// render a form that is refused on every submit with no clue why.
type csrfCarrier interface {
	setCSRF(token string)
}

// dashboardPage is the dashboard view plus the fields only a rendering needs.
//
// The view model itself stays in internal/dashboard and knows nothing about sessions or
// tokens; this is the web layer's own wrapper around it. Embedding is what lets the template
// keep saying .Rows and .Banner -- promoted fields resolve through it -- so adding the token
// did not mean rewriting the page.
type dashboardPage struct {
	dashboard.View
	CSRF string
}

func (p *dashboardPage) setCSRF(token string) { p.CSRF = token }

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	sessions, err := s.readSessions()
	view := dashboard.Build(cfg, sessions, s.Self, s.now())
	if err != nil {
		// The dashboard is still worth showing: which child is held and until when
		// does not depend on who is currently logged in. But the session column has to
		// say it could not look. Rendering this as an empty list would tell a parent
		// "no desktop session" -- a positive claim the failed read did not establish --
		// and would make a broken session backend look exactly like a quiet house.
		s.logf("web: %s session list unavailable: %v", requestID(r), err)
		view.SessionsKnown = false
		w.Header().Set("X-Plock-Note", "sessions-unavailable")
	}
	// The pointer, not the value: render fills the token through setCSRF, and a copy would
	// fill a temporary the template never sees.
	s.render(w, r, "dashboard", &dashboardPage{View: view})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	listing, err := s.readAccounts()
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
	rows := users.Union(listing.Accounts, configured)
	s.render(w, r, "users", usersPage{
		Rows:   rows,
		Names:  profileNames(cfg),
		Gaps:   users.Unmanaged(rows),
		Manage: cfg.Settings.Enforcement,
		// The listing's notes are the difference between a page that shows a child
		// missing and one that says why. They are the package's own words, so they are
		// passed through rather than paraphrased here.
		Notes:  listing.Notes,
		UIDMin: listing.UIDMin,
		UIDMax: listing.UIDMax,
	})
}

// usersPage is what users.tmpl is given.
//
// The profile names ride along because a row carries only a profile id and the
// template only ever sees the row it is rendering, so it cannot look a name up.
// Showing the raw id would also hide the case the template cares about: an id with no
// profile behind it is a dangling assignment, and it is rendered as a problem rather
// than as a name.
type usersPage struct {
	Rows   []users.Managed
	Names  map[string]string
	Gaps   int
	Manage bool

	// Notes, UIDMin and UIDMax come straight off the Listing. They are here because a
	// page that shows an empty table and no explanation is the exact failure internal/
	// users was written to prevent -- and the notes only count as "rendered" if a
	// template actually renders them, which is what the seam was widened for.
	Notes  []string
	UIDMin uint32
	UIDMax uint32
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
