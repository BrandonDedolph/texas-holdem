package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// The full-size study card.
//
// Two card sizes exist on purpose. The table screen keeps the 5x3 mini card:
// its seat ring already spends 13 of 24 rows, and 5-row cards would cost
// roughly four more rows between the board and the hero's hand, which that
// layout cannot pay. Lessons, the trainer and the quick reference exist to
// LOOK at cards, have the rows to spare, and so draw this full 7x5 card -
// rank in the top-left index, suit pip centred, rank mirrored bottom-right,
// like a real card (the euchre project's card, which the owner asked for).
// Glanceable on the table, a full card when the card itself is the subject.

// Full-card geometry: every full card is exactly FullCardWidth cells wide
// and FullCardHeight rows tall. The interior is five cells, sized for the
// two-character "10" rank plus breathing room.
const (
	FullCardWidth  = 7
	FullCardHeight = 5

	fullInterior = FullCardWidth - 2
)

// FullCard renders the 7x5 study card: rank top-left, suit centred, rank
// mirrored bottom-right, on a white face. highlight swaps the single frame
// for the double-border emphasis form - "these five play", or the card a
// drill is pointing at.
//
// The face is deliberately white with fixed dark ink rather than adaptive
// terminal colours. A playing card is a physical white object, and drawing
// it as one is most of what makes this read as a card instead of glyphs
// floating in space (design-tui.md section 3.1).
func FullCard(c engine.Card, highlight bool) string {
	g := theme.G
	border := theme.Current.CardBorder
	tl, tr, bl, br, h, v := g.CardTL, g.CardTR, g.CardBL, g.CardBR, g.CardH, g.CardVL
	vr := g.CardVR
	if highlight {
		border = theme.Current.CardWinner
		tl, tr, bl, br, h, v = g.CardDblTL, g.CardDblTR, g.CardDblBL, g.CardDblBR, g.CardDblH, g.CardDblV
		vr = g.CardDblV
	}
	rank := c.Rank().Symbol()
	suit := theme.SuitGlyph(c.Suit())
	ink := theme.FaceStyle(c.Suit())
	pad := strings.Repeat(" ", fullInterior-lipgloss.Width(rank))
	mid := strings.Repeat(" ", (fullInterior-1)/2)
	rows := []string{
		border.Render(tl + strings.Repeat(h, fullInterior) + tr),
		border.Render(v) + ink.Render(rank+pad) + border.Render(vr),
		border.Render(v) + ink.Render(mid+suit+mid) + border.Render(vr),
		border.Render(v) + ink.Render(pad+rank) + border.Render(vr),
		border.Render(bl + strings.Repeat(h, fullInterior) + br),
	}
	return strings.Join(rows, "\n")
}

// fullPlaceholder is an undealt board slot at full-card size: a dim pip in
// the box a future card will fill, so the board's width never changes as
// streets arrive.
func fullPlaceholder() string {
	blank := strings.Repeat(" ", FullCardWidth)
	mid := strings.Repeat(" ", (FullCardWidth-1)/2)
	pip := mid + theme.Current.BoardPlaceholder.Render(theme.G.Dot) + mid
	return strings.Join([]string{blank, blank, pip, blank, blank}, "\n")
}

// FullBoardRow renders the community board as five full-size slots: dealt
// cards fill left to right, undealt slots render as dim placeholder pips.
// highlight marks cards to draw in the emphasis frame; 0 highlights none.
func FullBoardRow(dealt []engine.Card, highlight engine.CardSet) string {
	slots := make([]string, BoardSlots)
	for i := range slots {
		if i < len(dealt) {
			slots[i] = FullCard(dealt[i], highlight.Has(dealt[i]))
		} else {
			slots[i] = fullPlaceholder()
		}
	}
	return joinSlots(slots, " ")
}

// FullHand renders a two-card holding side by side at full size.
func FullHand(hole [2]engine.Card, highlight engine.CardSet) string {
	return joinSlots([]string{
		FullCard(hole[0], highlight.Has(hole[0])),
		FullCard(hole[1], highlight.Has(hole[1])),
	}, " ")
}

// MiniCards renders a row of face-up mini cards with one-column gaps - the
// drawn form for a named hand in rows too tight for the 5-row study card.
// Only the 60x20 compact breakpoint is that tight; everywhere else uses
// FullCards, which is what a named hand should look like.
func MiniCards(cards ...engine.Card) string {
	slots := make([]string, len(cards))
	for i, c := range cards {
		slots[i] = MiniCard(c)
	}
	return joinSlots(slots, " ")
}

// FullCards renders a row of face-up study cards with one-column gaps: the
// drawn form for a named hand ("these five specific cards") wherever the
// rows exist to pay for it - the ten-tier hand ladder, order-the-hands
// drill items, the quick reference's rankings tab.
//
// Five rows per hand is expensive, and the ladder's ten tiers scroll
// because of it. That is the right trade: these screens exist to be looked
// at, and a hand a learner is asked to rank should look like cards on a
// table, not like a compressed index. The mini form is the fallback for
// the compact breakpoint only.
func FullCards(cards ...engine.Card) string {
	slots := make([]string, len(cards))
	for i, c := range cards {
		slots[i] = FullCard(c, false)
	}
	return joinSlots(slots, " ")
}

// FullCardsEmphasis is FullCards with the cards in highlight drawn in the
// double-border frame.
func FullCardsEmphasis(cards []engine.Card, highlight engine.CardSet) string {
	slots := make([]string, len(cards))
	for i, c := range cards {
		slots[i] = FullCard(c, highlight.Has(c))
	}
	return joinSlots(slots, " ")
}
