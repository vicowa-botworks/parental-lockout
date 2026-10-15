package web

import (
	"errors"
	"net/http"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// mutateConfig is the only way anything on this server changes the config document.
//
// It exists because the document has no concurrency control of its own. config.Save is
// atomic -- temp file, rename -- so a reader never sees half a write, but atomicity is not
// the hazard here. Two load-mutate-save cycles that overlap both start from the same
// document and the second rename wins, silently deleting the first change. A parent who
// adds a rule at the same moment they toggle the master switch ends up with one of those
// two edits and no indication anything was lost. The window is milliseconds, which is
// exactly the kind of window that survives every test and reports as a bug report three
// weeks later.
//
// The lock lives here rather than inside the seam for one reason: a seam is supplied by the
// caller, and a caller who forgets the mutex produces a server that loses edits under
// concurrent requests. Holding it in the layer that owns the route means forgetting is not
// available. The cost is that a mutation serializes behind every other mutation on the
// process, which is the correct behaviour for a single-daemon admin interface and is not a
// throughput concern at the number of writes a parent makes in an evening.
//
// The seam still does the load and the save, because only the caller knows the path and the
// policy around it. What it cannot do is run unsynchronized.
func (s *Server) mutateConfig(fn func(*config.Config) error) error {
	if s.Mutate == nil {
		return errors.New("no configuration write path is wired")
	}
	if fn == nil {
		return errors.New("no mutation was supplied")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	// Validation is folded into the caller's mutation rather than left to the seam, for
	// the same reason the mutex is: the seam is supplied by the caller, and a seam that
	// forgets to check produces a server that writes documents the daemon refuses to load.
	//
	// That is not a hypothetical shape. plock serve exits 1 when Validate fails at startup,
	// so a document written here that fails validation does not merely lose the edit -- it
	// means the next restart does not come up, and enforcement stops until a human notices.
	// config.Validate's own doc says refusing to write is "the responsibility of whoever
	// produced the change"; this is that whoever.
	//
	// It runs inside the lock, on the same document the seam is about to save, so a
	// concurrent mutation cannot slip an unchecked value in between the check and the rename.
	return s.Mutate(func(c *config.Config) error {
		if err := fn(c); err != nil {
			return err
		}
		return c.Validate()
	})
}

// mutateRefused reports a failed mutation on the right side of the 4xx/5xx line.
//
// A document the daemon would refuse to load is the operator's request being wrong -- the
// same category as "an override needs a reason" -- so it answers 422 and repeats what
// Validate objected to, because that text already names the field. Anything else is the
// server failing, and says so. Collapsing the two into 503 would tell a parent to retry a
// submission that will be refused identically forever.
func (s *Server) mutateRefused(w http.ResponseWriter, r *http.Request, why string, err error) {
	if errors.Is(err, config.ErrInvalid) {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			// The sentinel's own text already says the configuration is not valid, so the
			// wrapper only adds the consequence. Saying it twice reads like two errors.
			"refusing to save: "+err.Error())
		return
	}
	s.pageError(w, r, why, err)
}

// pageRefuse answers a request the server is declining on purpose.
//
// It is separate from pageError because the two mean opposite things and a caller has to be
// able to tell them apart. pageError is 503: "I could not look, try again". pageRefuse is a
// 4xx: "what you asked for is not what this daemon does, and retrying will not change that".
// Collapsing them would teach an operator to retry a request that was correctly refused, and
// would hide a real outage behind a validation message.
//
// The message is the whole answer. The reason is chosen by the caller and is safe to show --
// it names the rule that refused, never an internal error -- because a parent acting on this
// page needs to know what to change, and "invalid request" sends them to a terminal instead.
func (s *Server) pageRefuse(w http.ResponseWriter, _ *http.Request, status int, why string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, why, status)
}
