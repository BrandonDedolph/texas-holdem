package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// The keybind-legend invariant (docs/design-tui.md §8.2): the keys a
// screen's update loop accepts are exactly the keys its help overlay
// documents, both driven off keymap.go. These tests probe both directions —
// a key handled but undocumented, or documented but unhandled, fails here.

// legendScreen pairs a fresh screen with its name for error messages.
type legendScreen struct {
	name  string
	model keyScreen
}

// legendScreens builds one of every shell screen, sized to the baseline.
func legendScreens(t *testing.T) []legendScreen {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	p := profile.NewProfile()
	prefs := DefaultPrefs()

	screens := []legendScreen{
		{"MainMenu", NewMainMenu()},
		{"GameSetup", NewGameSetup(p, prefs)},
		{"Settings", NewSettings(p, prefs)},
		{"QuickReference", NewQuickReference()},
		{"ComingSoon", newComingSoon(ScreenLessons)},
	}
	for _, s := range screens {
		s.model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	}
	return screens
}

// TestKeymapKeysAreUnique guards the dispatch premise: within one keymap a
// key resolves to exactly one action.
func TestKeymapKeysAreUnique(t *testing.T) {
	maps := map[string]KeyMap{
		"mainMenuKeys":   mainMenuKeys,
		"gameSetupKeys":  gameSetupKeys,
		"settingsKeys":   settingsKeys,
		"quickRefKeys":   quickRefKeys,
		"comingSoonKeys": comingSoonKeys,
		"globalKeys":     globalKeys,
	}
	for name, km := range maps {
		seen := map[string]bool{}
		for _, b := range km {
			for _, k := range b.Keys {
				if seen[k] {
					t.Errorf("%s: key %q bound twice", name, k)
				}
				seen[k] = true
			}
		}
	}
}

// TestEveryDocumentedActionIsHandled is the "documented but unhandled"
// direction: every action a screen's keymap advertises must be implemented
// by its handleAction. ActHelp is exempt — the shared dispatch in help.go
// consumes it before handleAction ever sees it (verified separately below).
func TestEveryDocumentedActionIsHandled(t *testing.T) {
	for _, s := range legendScreens(t) {
		for _, b := range s.model.keymap() {
			if b.Action == ActHelp {
				continue
			}
			if _, handled := s.model.handleAction(b.Action); !handled {
				t.Errorf("%s: keymap documents %q (%s) but handleAction ignores it",
					s.name, b.Label, b.Help)
			}
		}
	}
}

// TestUndocumentedKeysAreIgnored is the "handled but undocumented"
// direction: a key absent from the screen's keymap must change nothing —
// same view, no command. The probe set is every key any screen binds plus a
// spread of plausible strays, so a handler quietly added outside keymap.go
// gets caught by the union of everyone else's keys.
func TestUndocumentedKeysAreIgnored(t *testing.T) {
	probe := map[string]bool{
		"z": true, "x": true, "f": true, "r": true, "b": true, "c": true,
		"a": true, "v": true, "+": true, "-": true, "tab": true,
		"pgdown": true, "f1": true, "ctrl+r": true, "backspace": true,
	}
	for _, km := range []KeyMap{mainMenuKeys, gameSetupKeys, settingsKeys, quickRefKeys, comingSoonKeys} {
		for _, b := range km {
			for _, k := range b.Keys {
				probe[k] = true
			}
		}
	}

	for _, s := range legendScreens(t) {
		km := s.model.keymap()
		for k := range probe {
			if km.Contains(k) {
				continue // documented; exercised elsewhere
			}
			before := s.model.View()
			_, cmd := s.model.Update(key(k))
			if cmd != nil {
				t.Errorf("%s: undocumented key %q produced a command", s.name, k)
			}
			if after := s.model.View(); after != before {
				t.Errorf("%s: undocumented key %q changed the view", s.name, k)
			}
		}
	}
}

// TestHelpOverlayDocumentsKeymap: the "?" sheet must render every binding's
// keycap and description — the legend and the handlers share keymap.go, and
// this pins the rendering half of that contract. The global bindings and the
// close instruction must be present too.
func TestHelpOverlayDocumentsKeymap(t *testing.T) {
	for _, s := range legendScreens(t) {
		s.model.Update(key("?"))
		view := s.model.View()

		if !strings.Contains(view, "Controls") {
			t.Errorf("%s: help overlay missing title", s.name)
			continue
		}
		for _, b := range s.model.keymap() {
			if !strings.Contains(view, b.Label) {
				t.Errorf("%s: help overlay missing keycap %q", s.name, b.Label)
			}
			if !strings.Contains(view, b.Help) {
				t.Errorf("%s: help overlay missing description %q", s.name, b.Help)
			}
		}
		for _, b := range globalKeys {
			if !strings.Contains(view, b.Label) {
				t.Errorf("%s: help overlay missing global keycap %q", s.name, b.Label)
			}
		}
		if !strings.Contains(view, "Press any key to close") {
			t.Errorf("%s: help overlay must say how to close", s.name)
		}
	}
}

// TestHelpOverlayOpensAndAnyKeyCloses verifies the shared "?" protocol:
// opens on "?", any key closes, and the screen underneath is untouched.
func TestHelpOverlayOpensAndAnyKeyCloses(t *testing.T) {
	for _, s := range legendScreens(t) {
		base := s.model.View()

		s.model.Update(key("?"))
		if s.model.View() == base {
			t.Errorf("%s: \"?\" should open the help overlay", s.name)
			continue
		}

		// Any key closes — including one the screen doesn't bind.
		s.model.Update(key("z"))
		if got := s.model.View(); got != base {
			t.Errorf("%s: closing help must restore the exact previous view", s.name)
		}

		// While open, keys must not leak to the screen underneath.
		s.model.Update(key("?"))
		_, cmd := s.model.Update(key("esc")) // esc would navigate if it leaked
		if cmd != nil {
			t.Errorf("%s: key pressed on the open overlay leaked a command", s.name)
		}
		if got := s.model.View(); got != base {
			t.Errorf("%s: overlay close-by-esc must restore the previous view", s.name)
		}
	}
}
