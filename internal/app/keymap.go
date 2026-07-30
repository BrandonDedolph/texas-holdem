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
)

// globalKeys are handled by the App root before any screen sees the key.
// They render in every help overlay.
var globalKeys = KeyMap{
	{ActQuit, []string{"ctrl+c"}, "ctrl+c", "quit immediately"},
}
