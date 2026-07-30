// Package components provides the reusable rendering pieces of the table
// view: cards, seat blocks, the pot line and the bet-sizing slider readout.
//
// Components are pure render functions over view models the package defines
// itself; the app layer maps engine state onto them. Two invariants hold
// everywhere: every region renders a fixed size even when empty (the layout
// never reflows), and all styling and glyphs come from internal/ui/theme
// (no hex literal, no raw Unicode glyph in this package).
package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// Mini-card geometry: every mini card is exactly MiniCardWidth cells wide and
// MiniCardHeight rows tall, and every board slot occupies the same box
// whether or not a card has been dealt to it.
const (
	MiniCardWidth  = 4
	MiniCardHeight = 3

	// BoardSlots is the number of community-card slots. The board always
	// renders all five so nothing anchored around it ever moves.
	BoardSlots = 5
)

// MiniCard renders a face-up 4x3 card with rounded corners:
//
//	+--+
//	[As]   (Unicode: the frame is rounded box drawing, the suit a glyph)
//	+--+
//
// Ten renders as "T", never "10" - single-character ranks keep every card
// exactly four cells wide and teach standard poker notation.
func MiniCard(c engine.Card) string {
	interior := theme.Current.CardBorder.Render("??")
	if c.Valid() {
		face := string(c.Rank().Letter()) + theme.SuitGlyph(c.Suit())
		interior = theme.SuitStyle(c.Suit()).Render(face)
	}
	return miniCardFrame(interior)
}

// MiniCardBack renders a face-down 4x3 card.
func MiniCardBack() string {
	return miniCardFrame(theme.Current.CardBack.Render(theme.G.FaceDown))
}

// miniCardFrame wraps a pre-styled two-cell interior in the mini-card frame.
func miniCardFrame(interior string) string {
	g := theme.G
	border := theme.Current.CardBorder
	top := border.Render(g.CardTL + g.CardH + g.CardH + g.CardTR)
	mid := border.Render(g.CardVL) + interior + border.Render(g.CardVR)
	bottom := border.Render(g.CardBL + g.CardH + g.CardH + g.CardBR)
	return top + "\n" + mid + "\n" + bottom
}

// boardPlaceholder renders an undealt board slot: a dim placeholder pip
// occupying the exact box a mini card would, so the board region - and
// everything anchored around it - never moves when a street is dealt.
func boardPlaceholder() string {
	blank := strings.Repeat(" ", MiniCardWidth)
	pip := theme.Current.BoardPlaceholder.Render(theme.G.BoardSlot)
	return blank + "\n" + pip + "\n" + blank
}

// InlineCard renders the compact two-cell form used wherever prose mentions
// a card: rank and suit glyph in the suit's ink, bold. Cards look identical
// inline and on the table because both resolve through theme.SuitStyle.
func InlineCard(c engine.Card) string {
	if !c.Valid() {
		return theme.Current.CardBorder.Render("??")
	}
	face := string(c.Rank().Letter()) + theme.SuitGlyph(c.Suit())
	return theme.SuitStyle(c.Suit()).Render(face)
}

// BoardRow renders the community board as five fixed slots with one-column
// gaps: dealt cards fill left to right, undealt slots render as dim
// placeholder pips. The result is always exactly 3 rows tall and the same
// width for 0, 3, 4 or 5 dealt cards. Cards beyond the fifth are ignored.
func BoardRow(dealt []engine.Card) string {
	slots := make([]string, BoardSlots)
	for i := range slots {
		if i < len(dealt) {
			slots[i] = MiniCard(dealt[i])
		} else {
			slots[i] = boardPlaceholder()
		}
	}
	return joinSlots(slots, " ")
}

// joinSlots joins equal-height multi-line blocks horizontally with a gap.
func joinSlots(blocks []string, gap string) string {
	split := make([][]string, len(blocks))
	rows := 0
	for i, b := range blocks {
		split[i] = strings.Split(b, "\n")
		if len(split[i]) > rows {
			rows = len(split[i])
		}
	}
	lines := make([]string, rows)
	for r := 0; r < rows; r++ {
		parts := make([]string, len(blocks))
		for i := range blocks {
			if r < len(split[i]) {
				parts[i] = split[i][r]
			}
		}
		lines[r] = strings.Join(parts, gap)
	}
	return strings.Join(lines, "\n")
}
