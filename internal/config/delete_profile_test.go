package config

import (
	"strings"
	"testing"
)

// A profile is shared by definition -- that is what makes it a profile rather than a
// per-account setting -- so the refusal that protects it has to name everyone holding it.
//
// Naming only the first match makes the operator reassign alice, retry, and be told about
// bob. That reads as the tool failing twice, and the second failure arrives after the
// operator already did the work the first one asked for.

func TestDeleteProfileNamesEveryAccountHoldingIt(t *testing.T) {
	c := valid()
	c.UpsertUser(User{Username: "alice", ProfileID: "schoolnight", Enabled: true})
	c.UpsertUser(User{Username: "bob", ProfileID: "schoolnight", Enabled: true})
	c.UpsertUser(User{Username: "carol", ProfileID: "schoolnight", Enabled: true})
	// An account on a different profile must not be dragged into the message.
	c.UpsertUser(User{Username: "dave", ProfileID: c.Profiles[0].ID, Enabled: true})
	if len(c.Profiles) < 2 {
		c.UpsertProfile(Profile{ID: "weekend", Name: "weekend", Timezone: "UTC"})
		c.UpsertUser(User{Username: "dave", ProfileID: "weekend", Enabled: true})
	}

	err := c.DeleteProfile("schoolnight")
	if err == nil {
		t.Fatal("DeleteProfile removed a profile still assigned to three accounts")
	}
	msg := err.Error()
	for _, want := range []string{"alice", "bob", "carol"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal omits %q, so the operator has to discover it on the next "+
				"attempt; message: %s", want, msg)
		}
	}
	if strings.Contains(msg, "dave") {
		t.Errorf("the refusal names an account that is not holding this profile; message: %s", msg)
	}
	// Plural grammar: the message is read by a person at a kitchen table.
	if !strings.Contains(msg, "them") {
		t.Errorf("three accounts is plural; message: %s", msg)
	}
}

func TestDeleteProfileSingularGrammarForOneAccount(t *testing.T) {
	c := valid()
	err := c.DeleteProfile("schoolnight")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	msg := err.Error()
	if !strings.Contains(msg, "that account") {
		t.Errorf("one account should read as one account; message: %s", msg)
	}
	if strings.Contains(msg, "them") {
		t.Errorf("one account must not read as several; message: %s", msg)
	}
}

// Account names are whatever passwd holds. A name with a space would otherwise run into its
// neighbour in the list an operator is reading off to decide which child to reassign.
func TestBlockingNamesAreQuoted(t *testing.T) {
	c := valid()
	c.UpsertUser(User{Username: "mary jane", ProfileID: "schoolnight", Enabled: true})
	c.UpsertUser(User{Username: "paul", ProfileID: "schoolnight", Enabled: true})

	err := c.DeleteProfile("schoolnight")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"mary jane"`) {
		t.Errorf("a name containing a space is not quoted, so it reads as two names; message: %s", msg)
	}
}
