package ui

import "testing"

func TestFocusTabSelectsTheAskedTab(t *testing.T) {
	h := &browserHost{tabs: []*browserTab{{id: "tab1"}, {id: "tab2"}}}
	if err := h.focusTab("tab2"); err != nil {
		t.Fatal(err)
	}
	if got := h.activeTab(); got == nil || got.id != "tab2" {
		t.Fatalf("active %+v", got)
	}
	if err := h.focusTab(""); err != nil {
		t.Fatal(err)
	}
	if got := h.activeTab(); got == nil || got.id != "tab2" {
		t.Fatalf("empty id changed the tab: %+v", got)
	}
	if err := h.focusTab("missing"); err == nil {
		t.Fatal("missing tab was accepted")
	}
}
