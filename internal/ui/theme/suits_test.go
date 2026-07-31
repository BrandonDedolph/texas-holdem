package theme

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/charmbracelet/lipgloss"
)

// setDeckMode swaps the deck color mode for one test.
func setDeckMode(t *testing.T, m DeckMode) {
	t.Helper()
	old := CurrentDeckMode()
	SetDeckMode(m)
	t.Cleanup(func() { SetDeckMode(old) })
}

func suitInk(t *testing.T, s engine.Suit) lipgloss.TerminalColor {
	t.Helper()
	return SuitStyle(s).GetForeground()
}

func TestDefaultDeckModeIsTwoColor(t *testing.T) {
	// A real deck is red and black, and so is every casino table, book and
	// poker client the player will meet outside this app. Four-color is a
	// genuine aid against flush blindness, but as a default it contradicts
	// everything else the learner will ever see - so it lives one switch
	// away in Settings rather than out of the box.
	if CurrentDeckMode() != TwoColor {
		t.Errorf("CurrentDeckMode() = %v, want TwoColor", CurrentDeckMode())
	}
}

func TestSuitStyleFourColor(t *testing.T) {
	setDeckMode(t, FourColor)
	suits := []engine.Suit{engine.Spades, engine.Hearts, engine.Diamonds, engine.Clubs}
	inks := map[lipgloss.TerminalColor]engine.Suit{}
	for _, s := range suits {
		ink := suitInk(t, s)
		if prev, dup := inks[ink]; dup {
			t.Errorf("four-color: %v and %v share ink %v", prev, s, ink)
		}
		inks[ink] = s
	}
	if got := suitInk(t, engine.Spades); got != lipgloss.TerminalColor(ColSuitSpade) {
		t.Errorf("four-color spade ink = %v, want ColSuitSpade", got)
	}
	if got := suitInk(t, engine.Diamonds); got != lipgloss.TerminalColor(ColSuitDiamond) {
		t.Errorf("four-color diamond ink = %v, want ColSuitDiamond", got)
	}
}

func TestSuitStyleTwoColor(t *testing.T) {
	setDeckMode(t, TwoColor)
	if suitInk(t, engine.Hearts) != suitInk(t, engine.Diamonds) {
		t.Error("two-color: hearts and diamonds should share the red ink")
	}
	if suitInk(t, engine.Spades) != suitInk(t, engine.Clubs) {
		t.Error("two-color: spades and clubs should share the black ink")
	}
	if suitInk(t, engine.Hearts) == suitInk(t, engine.Spades) {
		t.Error("two-color: red and black suits should differ")
	}
	if got := suitInk(t, engine.Clubs); got != lipgloss.TerminalColor(ColSuitSpade) {
		t.Errorf("two-color club ink = %v, want ColSuitSpade", got)
	}
}

func TestSuitStyleAdaptiveInks(t *testing.T) {
	// Chrome and suit inks must work on both light and dark terminals;
	// spade/club ink in particular flips with the background.
	if ColSuitSpade.Light == ColSuitSpade.Dark {
		t.Error("spade ink must flip with the terminal background")
	}
	setDeckMode(t, FourColor)
	for _, s := range []engine.Suit{engine.Spades, engine.Hearts, engine.Diamonds, engine.Clubs} {
		if _, ok := suitInk(t, s).(lipgloss.AdaptiveColor); !ok {
			t.Errorf("SuitStyle(%v) foreground is %T, want lipgloss.AdaptiveColor", s, suitInk(t, s))
		}
	}
}
