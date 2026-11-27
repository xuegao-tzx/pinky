package policy

import (
	"strings"
	"testing"

	"github.com/DIYgod/pinky/internal/screen"
)

func TestUnrestrictedAllowsEverything(t *testing.T) {
	e := Default()
	bank := screen.App{Name: "Bank", ID: "com.bank.app"}
	pw := &screen.Element{Role: screen.RoleInput, Label: "Password"}
	in := Input{Op: OpType, App: bank, Target: pw, Text: "hunter2"}
	if v := e.Evaluate(in); v.Decision == Allow {
		t.Fatal("the default engine allows a password in a banking app")
	}
	if _, blocked := e.Blocked(bank); !blocked && e.Categories(bank) != nil {
		// The bundled table may not know this id. A finance category is
		// added below so the deny is real either way.
	}
	e.AddApps("finance", CategoryEntries{IDs: []string{"com.bank.app"}, Names: []string{"Bank"}})
	if v := e.Evaluate(Input{Op: OpLaunch, Launch: bank}); v.Decision != Deny {
		t.Fatalf("finance launch: %+v", v)
	}
	if v := e.Evaluate(in); v.Decision == Allow || v.Rule != RuleCredentials {
		t.Fatalf("password: %+v", v)
	}

	e.SetUnrestricted(true)
	if !e.Unrestricted() {
		t.Fatal("not unrestricted")
	}
	if v := e.Evaluate(in); v.Decision != Allow || v.Rule != "" {
		t.Fatalf("unrestricted type: %+v", v)
	}
	if v := e.Evaluate(Input{Op: OpTap, App: bank, Target: &screen.Element{Role: screen.RoleButton, Label: "Pay $12"}}); v.Decision != Allow {
		t.Fatalf("unrestricted pay: %+v", v)
	}
	if v := e.Evaluate(Input{Op: OpLaunch, Launch: bank}); v.Decision != Allow {
		t.Fatalf("unrestricted launch: %+v", v)
	}
	if rule, blocked := e.Blocked(bank); blocked || rule != "" {
		t.Fatalf("unrestricted blocked: %q %v", rule, blocked)
	}
	if !e.CredentialTypingAllowed(false) || !e.CredentialTypingAllowed(true) {
		t.Fatal("credential typing stays refused")
	}
	e.SetReadOnly("", true)
	if v := e.Evaluate(Input{Op: OpType, App: bank, Target: pw, Text: "x"}); v.Decision != Allow {
		t.Fatalf("read-only still refuses: %+v", v)
	}
	e.SetUnrestricted(false)
	if v := e.Evaluate(in); v.Decision == Allow || !strings.Contains(v.Rule, RuleCredentials) && v.Rule == "" {
		t.Fatalf("gates did not come back: %+v", v)
	}
}
