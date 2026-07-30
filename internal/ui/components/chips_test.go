package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// chipCount counts the chip glyphs in a rendered pile.
func chipCount(pile string) int {
	return strings.Count(stripANSI(pile), theme.G.Chip)
}

func TestChipScalePairs(t *testing.T) {
	cases := []struct {
		pot, call engine.Chips
		wantScale engine.Chips
		wantPot   int // chips in the pot pile
		wantCall  int // chips in the call pile
	}{
		// The pitch's canonical example: pot 45, call 30, one chip per 15,
		// so "call 2 to win 3" is drawn before it is arithmetic.
		{45, 30, 15, 3, 2},
		// The same ratio at 20x the stakes draws the same picture.
		{900, 600, 300, 3, 2},
		// Coprime amounts fall back to a bounded pile.
		{45, 31, 9, 5, 4},
		{7, 3, 2, 4, 2},
	}
	glyphSets(t, func(t *testing.T) {
		for _, c := range cases {
			scale := ChipScale(c.pot, c.call)
			if scale != c.wantScale {
				t.Errorf("ChipScale(%d, %d) = %d, want %d", c.pot, c.call, scale, c.wantScale)
			}
			if got := chipCount(ChipPile(c.pot, scale)); got != c.wantPot {
				t.Errorf("pot pile %d@%d: %d chips, want %d", c.pot, scale, got, c.wantPot)
			}
			if got := chipCount(ChipPile(c.call, scale)); got != c.wantCall {
				t.Errorf("call pile %d@%d: %d chips, want %d", c.call, scale, got, c.wantCall)
			}
		}
	})
}

func TestChipScaleBoundsPiles(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		amounts := [][]engine.Chips{
			{45, 30}, {900, 600}, {1, 1000000}, {13, 17, 19}, {5, 10},
		}
		for _, as := range amounts {
			scale := ChipScale(as...)
			if scale < 1 {
				t.Fatalf("ChipScale(%v) = %d, want >= 1", as, scale)
			}
			for _, a := range as {
				n := chipCount(ChipPile(a, scale))
				if n < 1 {
					t.Errorf("pile %d@%d has no chips", a, scale)
				}
				if n > MaxPileChips {
					t.Errorf("pile %d@%d has %d chips, want <= %d", a, scale, n, MaxPileChips)
				}
			}
		}
	})
}

func TestChipPileProportional(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		scale := ChipScale(900, 600)
		if pot, call := chipCount(ChipPile(900, scale)), chipCount(ChipPile(600, scale)); pot <= call {
			t.Errorf("bigger amount must draw the taller pile: pot %d chips, call %d chips", pot, call)
		}
	})
}

func TestChipPileCarriesAmount(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		got := stripANSI(ChipPile(1045, 300))
		if !strings.Contains(got, "1,045") {
			t.Errorf("pile should carry the amount: %q", got)
		}
	})
}

func TestChipPileDegenerateInputs(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		if got := ChipPile(0, 15); got != "" {
			t.Errorf("zero amount should render nothing, got %q", got)
		}
		if got := ChipPile(-5, 15); got != "" {
			t.Errorf("negative amount should render nothing, got %q", got)
		}
		// A degenerate caller-supplied scale must not flood the row.
		if n := chipCount(ChipPile(1000000, 1)); n > chipPileHardCap {
			t.Errorf("hostile scale drew %d chips, want <= %d", n, chipPileHardCap)
		}
		// Scale zero is treated as one, never a divide-by-zero.
		if got := ChipPile(3, 0); chipCount(got) != 3 {
			t.Errorf("scale 0 should behave as 1: %q", got)
		}
	})
}
