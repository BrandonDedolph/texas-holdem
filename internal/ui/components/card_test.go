package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// setGlyphs swaps the active glyph set for one test.
func setGlyphs(t *testing.T, g theme.Glyphs) {
	t.Helper()
	old := theme.G
	theme.G = g
	t.Cleanup(func() { theme.G = old })
}

// glyphSets runs a subtest against both the Unicode and ASCII glyph sets:
// every layout invariant must hold identically in the fallback.
func glyphSets(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	for name, set := range map[string]theme.Glyphs{
		"unicode": theme.Unicode(),
		"ascii":   theme.ASCII(),
	} {
		t.Run(name, func(t *testing.T) {
			setGlyphs(t, set)
			fn(t)
		})
	}
}

// allCards returns the full 52-card deck.
func allCards() []engine.Card {
	return engine.FullDeck().Cards()
}

// assertBlock asserts a rendered block is exactly rows tall and every row is
// exactly width cells (lipgloss.Width is display-width aware and strips any
// styling), the invariant that keeps the table layout from ever reflowing.
func assertBlock(t *testing.T, block string, width, rows int) {
	t.Helper()
	lines := strings.Split(block, "\n")
	if len(lines) != rows {
		t.Fatalf("got %d rows, want %d:\n%s", len(lines), rows, block)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("row %d is %d cells, want %d: %q", i, w, width, line)
		}
	}
}

func TestMiniCardDimensionsAll52(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, c := range allCards() {
			assertBlock(t, MiniCard(c), MiniCardWidth, MiniCardHeight)
		}
	})
}

func TestMiniCardBackDimensions(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		assertBlock(t, MiniCardBack(), MiniCardWidth, MiniCardHeight)
	})
}

func TestMiniCardInvalidCard(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		assertBlock(t, MiniCard(engine.Card(52)), MiniCardWidth, MiniCardHeight)
	})
}

func TestMiniCardTenRendersAs10(t *testing.T) {
	// The owner's decision: the ten renders "10" everywhere, never "T".
	glyphSets(t, func(t *testing.T) {
		card := MiniCard(engine.MustCard("Th"))
		want := "10" + theme.SuitGlyph(engine.Hearts)
		if !strings.Contains(card, want) {
			t.Errorf("MiniCard(Th) missing %q:\n%s", want, card)
		}
		if strings.Contains(card, "T"+theme.SuitGlyph(engine.Hearts)) {
			t.Errorf("ten must render as 10, never T:\n%s", card)
		}
	})
}

func TestMiniCardFaceContent(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, c := range allCards() {
			face := c.Rank().Symbol() + theme.SuitGlyph(c.Suit())
			if got := MiniCard(c); !strings.Contains(got, face) {
				t.Errorf("MiniCard(%s) missing face %q:\n%s", c.Code(), face, got)
			}
		}
	})
}

func TestInlineCardWidthsAll52(t *testing.T) {
	// Inline cards are natural width: two cells, three for the wide "10"
	// rank. The fixed-slot form pads everything to InlineCardWidth.
	glyphSets(t, func(t *testing.T) {
		for _, c := range allCards() {
			want := 2
			if c.Rank() == engine.Ten {
				want = 3
			}
			if w := lipgloss.Width(InlineCard(c)); w != want {
				t.Errorf("InlineCard(%s) is %d cells, want %d", c.Code(), w, want)
			}
			if w := lipgloss.Width(InlineCardSlot(c)); w != InlineCardWidth {
				t.Errorf("InlineCardSlot(%s) is %d cells, want %d", c.Code(), w, InlineCardWidth)
			}
		}
		// The invalid-card forms hold their widths too.
		if w := lipgloss.Width(InlineCard(engine.Card(52))); w != 2 {
			t.Errorf("InlineCard(invalid) is %d cells, want 2", w)
		}
		if w := lipgloss.Width(InlineCardSlot(engine.Card(52))); w != InlineCardWidth {
			t.Errorf("InlineCardSlot(invalid) is %d cells, want %d", w, InlineCardWidth)
		}
	})
}

func TestInlineCardWidthMatchesAcrossGlyphSets(t *testing.T) {
	for _, code := range []string{"As", "Th"} {
		setGlyphs(t, theme.Unicode())
		uni := lipgloss.Width(InlineCard(engine.MustCard(code)))
		setGlyphs(t, theme.ASCII())
		asc := lipgloss.Width(InlineCard(engine.MustCard(code)))
		if uni != asc {
			t.Errorf("InlineCard(%s) width differs between glyph sets: unicode %d, ascii %d",
				code, uni, asc)
		}
	}
}

func TestBoardRowStableWidth(t *testing.T) {
	// The load-bearing layout invariant: the board is the same size for 0,
	// 3, 4 and 5 dealt cards, so nothing anchored around it ever moves.
	glyphSets(t, func(t *testing.T) {
		const wantWidth = BoardSlots*MiniCardWidth + (BoardSlots - 1)
		deals := map[string][]engine.Card{
			"preflop": nil,
			"flop":    engine.MustCards("Ks 7d 2c"),
			"turn":    engine.MustCards("Ks 7d 2c 9s"),
			"river":   engine.MustCards("Ks 7d 2c 9s Ah"),
		}
		for street, cards := range deals {
			t.Run(street, func(t *testing.T) {
				assertBlock(t, BoardRow(cards), wantWidth, MiniCardHeight)
			})
		}
	})
}

func TestBoardRowIgnoresExtraCards(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		six := engine.MustCards("Ks 7d 2c 9s Ah 3d")
		five := engine.MustCards("Ks 7d 2c 9s Ah")
		if BoardRow(six) != BoardRow(five) {
			t.Error("BoardRow should ignore cards past the fifth slot")
		}
	})
}

func TestBoardRowPlaceholdersAreNotCards(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		empty := BoardRow(nil)
		if strings.Contains(empty, theme.G.CardTL) {
			t.Errorf("empty board should render placeholder pips, not card frames:\n%s", empty)
		}
		if !strings.Contains(empty, strings.TrimSpace(theme.G.BoardSlot)) {
			t.Errorf("empty board missing placeholder pip %q:\n%s", theme.G.BoardSlot, empty)
		}
	})
}
