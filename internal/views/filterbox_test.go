package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterBoxHandleBackTwoStage(t *testing.T) {
	var f FilterBox
	f.value = ""

	// Stage 2 first: nothing to clear, caller should pop.
	if f.HandleBack() {
		t.Fatalf("HandleBack() with empty value should return false (let caller pop)")
	}

	// A committed filter is present: first Esc should clear it, not pop.
	f.value = "error"
	if !f.HandleBack() {
		t.Fatalf("HandleBack() with a committed filter should return true (clear, don't pop)")
	}
	if f.value != "" {
		t.Fatalf("HandleBack() should clear the filter value, got %q", f.value)
	}

	// Now that it's empty again, the next Esc should let the caller pop.
	if f.HandleBack() {
		t.Fatalf("HandleBack() after clearing should return false on the second press")
	}
}

func TestFilterBoxHandleKeyEscWhileTyping(t *testing.T) {
	f := NewFilterBox()
	f.Start()
	f.input.SetValue("foo")

	handled, _ := f.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled {
		t.Fatalf("HandleKey(esc) while active should be handled")
	}
	if f.Active() {
		t.Fatalf("esc while typing should exit edit mode")
	}
	if f.Value() != "" {
		t.Fatalf("esc while typing should clear the filter, got %q", f.Value())
	}
}

func TestFilterBoxHandleKeyEnterCommits(t *testing.T) {
	f := NewFilterBox()
	f.Start()
	f.input.SetValue("running")

	handled, _ := f.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Fatalf("HandleKey(enter) while active should be handled")
	}
	if f.Active() {
		t.Fatalf("enter should exit edit mode")
	}
	if f.Value() != "running" {
		t.Fatalf("enter should commit the typed value, got %q", f.Value())
	}
}

func TestFilterBoxHandleKeyNotActive(t *testing.T) {
	f := NewFilterBox()
	handled, _ := f.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if handled {
		t.Fatalf("HandleKey should not handle keys when the box isn't active")
	}
}
