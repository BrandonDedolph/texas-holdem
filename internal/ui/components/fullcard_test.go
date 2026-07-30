package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// TestFullCardGeometry pins the study card's 7x5 box: exact dimensions for
// both rank widths, the rank in the top-left and bottom-right indices, and
// the same footprint highlighted or not - a highlight that changed the size
// would shift every layout that uses it.
func TestFullCardGeometry(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, code := range []string{"As", "10d"} {
			card := engine.MustCards(code)[0]
			for _, highlight := range []bool{false, true} {
				out := FullCard(card, highlight)
				assertBlock(t, out, FullCardWidth, FullCardHeight)
			}
			plain := stripANSI(FullCard(card, false))
			rank := card.Rank().Symbol()
			rows := strings.Split(plain, "\n")
			if !strings.HasPrefix(rows[1], theme.G.CardVL+rank) {
				t.Errorf("%s: top-left index missing, row %q", code, rows[1])
			}
			if !strings.HasSuffix(rows[3], rank+theme.G.CardVR) {
				t.Errorf("%s: bottom-right index missing, row %q", code, rows[3])
			}
		}
	})
}

// TestFullBoardRowStableWidth: the full-size board occupies the same box for
// 0, 3, 4 and 5 dealt cards, so nothing anchored around it ever moves.
func TestFullBoardRowStableWidth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		full := engine.MustCards("As Kd 10h 4c 2s")
		width := 5*FullCardWidth + 4
		for _, n := range []int{0, 3, 4, 5} {
			assertBlock(t, FullBoardRow(full[:n], 0), width, FullCardHeight)
		}
	})
}

// TestFullBoardRowHighlights: highlighted cards carry the emphasis frame,
// unhighlighted ones the plain frame.
func TestFullBoardRowHighlights(t *testing.T) {
	setGlyphs(t, theme.Unicode())
	board := engine.MustCards("As Kd 10h")
	out := stripANSI(FullBoardRow(board, engine.NewCardSet(board[0])))
	if !strings.Contains(out, theme.G.CardDblTL) {
		t.Error("highlighted card missing the double-border frame")
	}
	if !strings.Contains(out, theme.G.CardTL) {
		t.Error("plain cards missing the single-border frame")
	}
}

// TestFullHandGeometry: two full cards side by side, fixed box.
func TestFullHandGeometry(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		hole := engine.Holes("As 10d")
		assertBlock(t, FullHand(hole, 0), 2*FullCardWidth+1, FullCardHeight)
	})
}

// TestMiniCardsGeometry: a drawn hand of n mini cards is always 3 rows and
// n*(MiniCardWidth+1)-1 cells - the row form the hand ladder and the
// order-the-hands drills lay text next to.
func TestMiniCardsGeometry(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		cards := engine.MustCards("As Ks Qs Js 10s")
		for _, n := range []int{2, 5} {
			assertBlock(t, MiniCards(cards[:n]...), n*(MiniCardWidth+1)-1, MiniCardHeight)
		}
	})
}

// TestMiniCardEmphasisGeometry: the emphasis mini card keeps the exact
// mini-card footprint and uses the double-border frame.
func TestMiniCardEmphasisGeometry(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, code := range []string{"As", "10d"} {
			c := engine.MustCards(code)[0]
			out := MiniCardEmphasis(c)
			assertBlock(t, out, MiniCardWidth, MiniCardHeight)
			if !strings.Contains(stripANSI(out), theme.G.CardDblTL) {
				t.Errorf("%s: emphasis card missing the double-border frame", code)
			}
		}
	})
}
