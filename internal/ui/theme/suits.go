package theme

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/charmbracelet/lipgloss"
)

// DeckMode selects how suits are colored.
type DeckMode int

// Deck color modes. TwoColor is the default - a real deck is red and black,
// and so is every casino table, book and poker client the player will meet
// elsewhere; a training aid that contradicts all of them trains the wrong
// reflex. FourColor stays one switch away in Settings for anyone who wants
// it: distinguishing suits fast is a
// real beginner skill (flush blindness is the classic leak), so each suit
// gets its own ink. TwoColor is the live-card convention: red hearts and
// diamonds, black spades and clubs.
const (
	FourColor DeckMode = iota
	TwoColor
)

// deckMode is the active mode. Suit color is resolved through the one
// function SuitStyle, so the Settings toggle is a single switch.
var deckMode = TwoColor

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

// FaceStyle is the ink for a suit printed on a white card face. It is fixed
// rather than adaptive: the face is always white, so the ink is always the
// dark-on-white variant regardless of the terminal's background. SuitStyle
// is its counterpart for cards drawn as bare glyphs on the terminal.
//
// The four-color/two-color toggle applies here too - the deck mode is about
// which suits share an ink, not about the background they sit on.
func FaceStyle(s engine.Suit) lipgloss.Style {
	var ink lipgloss.Color
	if deckMode == TwoColor {
		switch s {
		case engine.Hearts, engine.Diamonds:
			ink = ColFaceHeart
		default:
			ink = ColFaceSpade
		}
	} else {
		switch s {
		case engine.Hearts:
			ink = ColFaceHeart
		case engine.Diamonds:
			ink = ColFaceDiamond
		case engine.Clubs:
			ink = ColFaceClub
		default:
			ink = ColFaceSpade
		}
	}
	return lipgloss.NewStyle().Background(ColCardFace).Foreground(ink)
}

// FaceBlank is the blank white interior of a card face, for padding cells
// that carry no ink but must still be part of the card.
func FaceBlank() lipgloss.Style {
	return lipgloss.NewStyle().Background(ColCardFace)
}
