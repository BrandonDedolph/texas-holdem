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

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// Mini-card geometry: every mini card is exactly MiniCardWidth cells wide and
// MiniCardHeight rows tall, and every board slot occupies the same box
// whether or not a card has been dealt to it. The interior is three cells,
// sized for the two-character "10" rank (the owner's decision: a ten shows
// as "10" everywhere, never "T"), so single-character ranks pad with a
// trailing space.
const (
	MiniCardWidth  = 5
	MiniCardHeight = 3

	// miniInterior is the face width inside the frame: "10" plus a suit.
	miniInterior = MiniCardWidth - 2

	// BoardSlots is the number of community-card slots. The board always
	// renders all five so nothing anchored around it ever moves.
	BoardSlots = 5
)

// MiniCard renders a face-up 5x3 card with rounded corners:
//
//	+---+
//	[As ]  or  [10s]   (Unicode: rounded box drawing, the suit a glyph)
//	+---+
func MiniCard(c engine.Card) string {
	interior := theme.Current.CardBorder.Render(padCell("??", miniInterior))
	if c.Valid() {
		interior = theme.SuitStyle(c.Suit()).Render(padCell(CardFace(c), miniInterior))
	}
	return miniCardFrame(interior)
}

// CardFace is the bare face text of a card: rank symbol plus the active
// suit glyph ("As", "10d" in the ASCII set). Two cells wide, three for
// tens. Callers that need a fixed slot pad to InlineCardWidth.
func CardFace(c engine.Card) string {
	return c.Rank().Symbol() + theme.SuitGlyph(c.Suit())
}

// padCell pads plain text with trailing spaces to exactly w display cells.
func padCell(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// MiniCardBack renders a face-down 5x3 card.
func MiniCardBack() string {
	return miniCardFrame(theme.Current.CardBack.Render(theme.G.FaceDown))
}

// miniCardFrame wraps a pre-styled three-cell interior in the mini-card frame.
func miniCardFrame(interior string) string {
	g := theme.G
	border := theme.Current.CardBorder
	rail := strings.Repeat(g.CardH, miniInterior)
	top := border.Render(g.CardTL + rail + g.CardTR)
	mid := border.Render(g.CardVL) + interior + border.Render(g.CardVR)
	bottom := border.Render(g.CardBL + rail + g.CardBR)
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

// InlineCardWidth is the widest inline card (a ten, three cells). Renderers
// that mix inline cards with undealt placeholders in a grid pad every slot
// to this width (InlineCardSlot) so nothing moves when a card lands.
const InlineCardWidth = 3

// InlineCard renders the compact form used wherever prose mentions a card:
// rank symbol and suit glyph in the suit's ink. Natural width (two cells,
// three for tens) so sentences stay tight; grid contexts that need a fixed
// column use InlineCardSlot instead. Cards look identical inline and on the
// table because both resolve through theme.SuitStyle.
func InlineCard(c engine.Card) string {
	if !c.Valid() {
		return theme.Current.CardBorder.Render("??")
	}
	return theme.SuitStyle(c.Suit()).Render(CardFace(c))
}

// InlineCardSlot is InlineCard padded to exactly InlineCardWidth cells:
// the fixed-slot form for board rows, where a placeholder pip must occupy
// the same box a future "10" will.
func InlineCardSlot(c engine.Card) string {
	card := InlineCard(c)
	if gap := InlineCardWidth - lipgloss.Width(card); gap > 0 {
		return card + strings.Repeat(" ", gap)
	}
	return card
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
