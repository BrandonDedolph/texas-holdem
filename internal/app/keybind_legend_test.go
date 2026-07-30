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
		// The review without a model: the empty state, its most static form.
		// The populated screen's legend behaviour is probed by the
		// TestHandReview* tests in hand_review_test.go.
		{"HandReview", NewHandReview(nil, ScreenMainMenu)},
		// The table without Init: no hand dealt, waiting state — its most
		// static keymap. The state-dependent maps are probed by the
		// TestTable* legend tests below.
		{"Table", NewTableScreen(TableConfig{SmallBlind: 5, BigBlind: 10,
			Stack: 1000, Speed: SpeedInstant}, prefs, profile.NewProfile())},
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
		"mainMenuKeys":      mainMenuKeys,
		"gameSetupKeys":     gameSetupKeys,
		"settingsKeys":      settingsKeys,
		"quickRefKeys":      quickRefKeys,
		"comingSoonKeys":    comingSoonKeys,
		"handReviewKeys":    handReviewKeys,
		"globalKeys":        globalKeys,
		"tableWaitingKeys":  tableWaitingKeys,
		"tablePauseKeys":    tablePauseKeys,
		"tableChoosingKeys": tableChoosingKeys,
		"tableSizingKeys":   tableSizingKeys,
		"tableHandDoneKeys": tableHandDoneKeys,
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

// TestTableKeymapTracksState: the table's keymap() must follow its
// interaction state, so the help overlay and the legend always describe the
// keys that are actually live (§8.2).
func TestTableKeymapTracksState(t *testing.T) {
	same := func(a, b KeyMap) bool { return len(a) == len(b) && &a[0] == &b[0] }

	m := buildTable(t, scenarioPreflop(), 80, 24)
	if !same(m.keymap(), tableChoosingKeys) {
		t.Error("hero to act: keymap must be the choosing set")
	}

	m = buildTable(t, scenarioFlopSizing(), 80, 24)
	if !same(m.keymap(), tableSizingKeys) {
		t.Error("sizing open: keymap must be the sizing set")
	}

	m = buildTable(t, scenarioShowdown(), 80, 24)
	if !same(m.keymap(), tableHandDoneKeys) {
		t.Error("between hands: keymap must be the hand-done set")
	}

	sc := scenarioFlopChoosing()
	sc.speed = SpeedLearn
	m = buildTable(t, sc, 80, 24)
	if !m.paused {
		t.Fatal("Learn speed should be paused after the flop here")
	}
	if !same(m.keymap(), tablePauseKeys) {
		t.Error("street pause: keymap must be the pause set")
	}
}

// TestTableChoosingKeysMatchRenderedKeycaps drives the full invariant
// through the screen: in a given legality state, a poker key changes the
// game if and only if its keycap is on screen.
func TestTableChoosingKeysMatchRenderedKeycaps(t *testing.T) {
	cases := []struct {
		name     string
		scenario func() tableScenario
		acts     map[string]string // key -> keycap text that must be rendered
		ignored  []string          // keys whose keycap is absent and press inert
	}{
		{
			name:     "preflop facing the blind",
			scenario: scenarioPreflop,
			acts:     map[string]string{"f": "f fold", "c": "c call", "r": "r raise"},
			ignored:  []string{"x", "b"},
		},
		{
			name:     "flop checked to hero",
			scenario: scenarioFlopChoosing,
			acts:     map[string]string{"x": "x check", "b": "b bet"},
			ignored:  []string{"f", "c"},
		},
	}
	for _, tc := range cases {
		view := stripANSI(buildTable(t, tc.scenario(), 80, 24).View())
		for k, cap := range tc.acts {
			if !strings.Contains(view, cap) {
				t.Errorf("%s: keycap %q not rendered", tc.name, cap)
			}
			m := buildTable(t, tc.scenario(), 80, 24) // fresh: keys mutate
			before := m.hand.Events()
			m.Update(key(k))
			if len(m.hand.Events()) == len(before) && m.bar.State() == ActionBarChoosing {
				t.Errorf("%s: rendered key %q did nothing", tc.name, k)
			}
		}
		for _, k := range tc.ignored {
			m := buildTable(t, tc.scenario(), 80, 24)
			before := m.View()
			m.Update(key(k))
			if m.View() != before {
				t.Errorf("%s: key %q has no keycap but changed the view", tc.name, k)
			}
		}
	}
}

// TestTableHelpOverlayPerState: every state's help sheet documents exactly
// its keymap.
func TestTableHelpOverlayPerState(t *testing.T) {
	states := map[string]func() *TableScreen{
		"choosing": func() *TableScreen { return buildTable(t, scenarioPreflop(), 80, 24) },
		"sizing":   func() *TableScreen { return buildTable(t, scenarioFlopSizing(), 80, 24) },
		"handDone": func() *TableScreen { return buildTable(t, scenarioShowdown(), 80, 24) },
	}
	for name, build := range states {
		m := build()
		km := m.keymap()
		m.Update(key("?"))
		view := m.View()
		for _, b := range km {
			if !strings.Contains(view, b.Label) {
				t.Errorf("%s help: missing keycap %q", name, b.Label)
			}
			if !strings.Contains(view, b.Help) {
				t.Errorf("%s help: missing description %q", name, b.Help)
			}
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
