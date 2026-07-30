package app

import tea "github.com/charmbracelet/bubbletea"

// This file is the single source of truth for key bindings. Screens dispatch
// key messages through their KeyMap, the help overlay renders the same
// KeyMap, and keybind_legend_test.go probes both — so a key that is handled
// but undocumented, or documented but unhandled, cannot exist without a test
// failing (docs/design-tui.md §8.2; euchre let its legend and handlers drift,
// this is the fix).

// KeyAction is what a key press means, independent of which physical key
// produced it. Screens switch on actions, never on key strings.
type KeyAction int

// Key actions shared across the shell screens.
const (
	ActUp KeyAction = iota
	ActDown
	ActLeft
	ActRight
	ActSelect
	ActBack
	ActHelp
	ActQuit
	ActTab1
	ActTab2
	ActTab3
	ActTab4

	// Table-screen actions (docs/design-tui.md §4.2). Poker actions are
	// KeyActions like any other: the update loop switches on these, the
	// action bar renders keycaps for the legal subset, and the legend tests
	// hold the two together.
	ActFold
	ActCheck
	ActCall
	ActBet
	ActRaise
	ActAllIn
	// Sizing-mode presets. Each digit is its own action because dispatch
	// hands screens a KeyAction, not the key that produced it — and in
	// sizing mode the digit's value is the meaning.
	ActPreset1
	ActPreset2
	ActPreset3
	ActPreset4
	ActPreset5
	ActDigit0
	ActDigit6
	ActDigit7
	ActDigit8
	ActDigit9
	ActBackspace
	ActContinue   // dismiss a pacing pause / deal the next hand
	ActCoachCycle // cycle coach verbosity Full -> Mistakes -> Off
	ActSpeedUp
	ActSpeedDown
	ActReview // open the hand review (between hands)
)

// Binding ties one action to the keys that trigger it and the legend text
// that documents it. Keys hold exact tea.KeyMsg.String() values.
type Binding struct {
	Action KeyAction
	Keys   []string
	Label  string // keycap text in the help overlay ("up/k")
	Help   string // what the key does ("move up")
}

// KeyMap is the complete set of keys one screen state accepts.
type KeyMap []Binding

// Lookup resolves a pressed key to its action.
func (km KeyMap) Lookup(key string) (KeyAction, bool) {
	for _, b := range km {
		for _, k := range b.Keys {
			if k == key {
				return b.Action, true
			}
		}
	}
	return 0, false
}

// Contains reports whether the keymap binds the given key.
func (km KeyMap) Contains(key string) bool {
	_, ok := km.Lookup(key)
	return ok
}

// keyScreen is what every shell screen implements so the keybind-legend test
// can drive dispatch and legend rendering from one source. keymap returns
// the bindings active in the screen's current state; handleAction performs
// one and reports whether the action is actually implemented.
type keyScreen interface {
	tea.Model
	keymap() KeyMap
	handleAction(a KeyAction) (tea.Cmd, bool)
}

// Shared binding fragments. Building keymaps from these keeps labels and
// vim aliases identical everywhere (euchre convention: h/l and j/k exist
// wherever arrows do).
var (
	bindUp     = Binding{ActUp, []string{"up", "k"}, "up/k", "move up"}
	bindDown   = Binding{ActDown, []string{"down", "j"}, "down/j", "move down"}
	bindLeft   = Binding{ActLeft, []string{"left", "h"}, "left/h", "previous value"}
	bindRight  = Binding{ActRight, []string{"right", "l"}, "right/l", "next value"}
	bindSelect = Binding{ActSelect, []string{"enter", " "}, "enter/space", "select"}
	bindHelp   = Binding{ActHelp, []string{"?"}, "?", "help"}
)

// Shared help strings for the sizing digit family (see tableSizingKeys).
const (
	presetHelp = "sizing presets: 1/3, 1/2, 2/3, pot, all-in"
	digitHelp  = "type an exact amount (0 starts typing)"
)

// backBinding returns an esc/q binding with screen-appropriate help text
// ("back to menu" vs "quit").
func backBinding(help string) Binding {
	return Binding{ActBack, []string{"esc", "q"}, "esc/q", help}
}

// Per-screen keymaps. Each is the *entire* vocabulary of its screen: the
// legend test asserts that keys outside a screen's keymap are ignored.
var (
	mainMenuKeys = KeyMap{
		bindUp, bindDown, bindSelect,
		backBinding("quit"),
		bindHelp,
	}

	gameSetupKeys = KeyMap{
		bindUp, bindDown, bindLeft, bindRight,
		Binding{ActSelect, []string{"enter", " "}, "enter/space", "select / cycle value"},
		backBinding("back to menu"),
		bindHelp,
	}

	settingsKeys = KeyMap{
		bindUp, bindDown, bindLeft, bindRight,
		Binding{ActSelect, []string{"enter", " "}, "enter/space", "cycle value"},
		backBinding("back to menu"),
		bindHelp,
	}

	quickRefKeys = KeyMap{
		Binding{ActLeft, []string{"left", "h"}, "left/h", "previous tab"},
		Binding{ActRight, []string{"right", "l"}, "right/l", "next tab"},
		Binding{ActTab1, []string{"1"}, "1", "hand rankings"},
		Binding{ActTab2, []string{"2"}, "2", "positions"},
		Binding{ActTab3, []string{"3"}, "3", "pot odds"},
		Binding{ActTab4, []string{"4"}, "4", "glossary"},
		backBinding("back to menu"),
		bindHelp,
	}

	comingSoonKeys = KeyMap{
		backBinding("back to menu"),
		bindHelp,
	}

	// The hand-review screen: step between hero decisions with the arrows
	// (the step past the last decision is the result frame), esc returns to
	// wherever the review was opened from (ReviewRequest.ReturnTo).
	handReviewKeys = KeyMap{
		Binding{ActLeft, []string{"left", "h"}, "left/h", "previous decision"},
		Binding{ActRight, []string{"right", "l"}, "right/l", "next decision / result"},
		backBinding("back"),
		bindHelp,
	}
)

// Table-screen keymaps, one per action-bar/pacing state (docs/design-tui.md
// §4.2). The table's keymap() picks among these, so the help overlay and the
// legend tests always see exactly the vocabulary of the current state.
//
// The choosing map is a superset of what any single spot allows: whether f,
// x, c, b or r actually does anything is decided by the engine's
// LegalActions, the same slice the action bar renders keycaps from — so
// "keys that act" and "keycaps on screen" cannot drift (the table legend
// test pins the correspondence per legality fixture).
var (
	// tableSharedKeys are live in every table state except sizing (where esc
	// must mean "cancel", not "leave").
	tableSharedKeys = KeyMap{
		Binding{ActCoachCycle, []string{"tab"}, "tab", "cycle coach (Full/Mistakes/Off)"},
		Binding{ActSpeedUp, []string{"+", "="}, "+", "speed up"},
		Binding{ActSpeedDown, []string{"-"}, "-", "slow down"},
		bindHelp,
		Binding{ActBack, []string{"esc"}, "esc", "to menu (session kept)"},
	}

	tableWaitingKeys = tableSharedKeys

	tablePauseKeys = append(KeyMap{
		Binding{ActContinue, []string{" ", "enter"}, "space", "continue"},
	}, tableSharedKeys...)

	tableChoosingKeys = append(KeyMap{
		Binding{ActFold, []string{"f"}, "f", "fold"},
		Binding{ActCheck, []string{"x"}, "x", "check (c is call)"},
		Binding{ActCall, []string{"c"}, "c", "call"},
		Binding{ActBet, []string{"b"}, "b", "bet (opens sizing)"},
		Binding{ActRaise, []string{"r"}, "r", "raise (opens sizing)"},
		Binding{ActAllIn, []string{"a"}, "a", "all-in (confirm with enter)"},
	}, tableSharedKeys...)

	// The preset and digit bindings share legend labels ("1-5", "0/6-9") so
	// the help sheet shows the family once; the help renderer dedupes rows
	// with identical labels. Each key is still its own action, because
	// dispatch hands screens the action, not the key.
	tableSizingKeys = KeyMap{
		Binding{ActPreset1, []string{"1"}, "1-5", presetHelp},
		Binding{ActPreset2, []string{"2"}, "1-5", presetHelp},
		Binding{ActPreset3, []string{"3"}, "1-5", presetHelp},
		Binding{ActPreset4, []string{"4"}, "1-5", presetHelp},
		Binding{ActPreset5, []string{"5"}, "1-5", presetHelp},
		Binding{ActDigit0, []string{"0"}, "0/6-9", digitHelp},
		Binding{ActDigit6, []string{"6"}, "0/6-9", digitHelp},
		Binding{ActDigit7, []string{"7"}, "0/6-9", digitHelp},
		Binding{ActDigit8, []string{"8"}, "0/6-9", digitHelp},
		Binding{ActDigit9, []string{"9"}, "0/6-9", digitHelp},
		Binding{ActBackspace, []string{"backspace"}, "backspace", "delete typed digit"},
		Binding{ActLeft, []string{"left", "h"}, "left/h", "nudge down one big blind"},
		Binding{ActRight, []string{"right", "l"}, "right/l", "nudge up one big blind"},
		Binding{ActSelect, []string{"enter"}, "enter", "confirm bet/raise"},
		Binding{ActBack, []string{"esc"}, "esc", "cancel sizing"},
		bindHelp,
	}

	tableHandDoneKeys = append(KeyMap{
		Binding{ActContinue, []string{" ", "enter"}, "space", "deal the next hand"},
		Binding{ActReview, []string{"v"}, "v", "review last hand"},
	}, tableSharedKeys...)
)

// globalKeys are handled by the App root before any screen sees the key.
// They render in every help overlay.
var globalKeys = KeyMap{
	{ActQuit, []string{"ctrl+c"}, "ctrl+c", "quit immediately"},
}
