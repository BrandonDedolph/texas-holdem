package theme

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/charmbracelet/lipgloss"
)

// DeckMode selects how suits are colored.
type DeckMode int

// Deck color modes. FourColor is the default: distinguishing suits fast is a
// real beginner skill (flush blindness is the classic leak), so each suit
// gets its own ink. TwoColor is the live-card convention: red hearts and
// diamonds, black spades and clubs.
const (
	FourColor DeckMode = iota
	TwoColor
)

// deckMode is the active mode. Suit color is resolved through the one
// function SuitStyle, so the Settings toggle is a single switch.
var deckMode = FourColor

// SetDeckMode selects the active deck color mode.
func SetDeckMode(m DeckMode) { deckMode = m }

// CurrentDeckMode reports the active deck color mode.
func CurrentDeckMode() DeckMode { return deckMode }

// SuitStyle returns the ink style for a suit under the active deck mode.
// The inks are adaptive (see palette.go): spade and club ink flip with the
// terminal background, which is why cards have no painted background. Suits
// stay distinguishable by glyph alone, so color is never the only cue.
func SuitStyle(s engine.Suit) lipgloss.Style {
	var ink lipgloss.AdaptiveColor
	if deckMode == TwoColor {
		switch s {
		case engine.Hearts, engine.Diamonds:
			ink = ColSuitHeart
		default:
			ink = ColSuitSpade
		}
	} else {
		switch s {
		case engine.Spades:
			ink = ColSuitSpade
		case engine.Hearts:
			ink = ColSuitHeart
		case engine.Diamonds:
			ink = ColSuitDiamond
		default:
			ink = ColSuitClub
		}
	}
	return lipgloss.NewStyle().Foreground(ink).Bold(true)
}
