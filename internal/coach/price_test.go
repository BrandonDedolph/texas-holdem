package coach

import (
	"math"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// TestPilePriceEncodesTheRatio is the design's load-bearing claim: the pile
// is the ratio, not the chip count, so 45:30 and 900:600 draw identically.
func TestPilePriceEncodesTheRatio(t *testing.T) {
	small := PilePrice(30, 45)
	big := PilePrice(600, 900)
	for _, p := range []PricePiles{small, big} {
		if p.PotPile != 3 || p.CallPile != 2 {
			t.Errorf("piles = %d/%d, want 3/2", p.PotPile, p.CallPile)
		}
		if p.Phrase != "call 2 to win 3 — need 40%" {
			t.Errorf("phrase = %q", p.Phrase)
		}
		if math.Abs(p.Required-0.4) > 1e-9 {
			t.Errorf("required = %v, want 0.4", p.Required)
		}
	}
}

func TestPilePriceTextbookSpots(t *testing.T) {
	cases := []struct {
		toCall, pot engine.Chips
		potPile     int
		callPile    int
		phrase      string
	}{
		// Pot-sized bet: call 50 into 100 → 2:1, need 33%.
		{50, 100, 2, 1, "call 1 to win 2 — need 33%"},
		// Half-pot: call 25 into 75 → 3:1, need 25%.
		{25, 75, 3, 1, "call 1 to win 3 — need 25%"},
		// Facing a standard open (the golden 7♥7♣ spot): 30 into 45.
		{30, 45, 3, 2, "call 2 to win 3 — need 40%"},
		// Two-thirds pot: call 20 into 50 → 5:2, need 29%.
		{20, 50, 5, 2, "call 2 to win 5 — need 29%"},
		// Overbet, calling more than the pot: 200 into 100 → 1:2, need 67%.
		{200, 100, 1, 2, "call 2 to win 1 — need 67%"},
	}
	for _, c := range cases {
		got := PilePrice(c.toCall, c.pot)
		if got.PotPile != c.potPile || got.CallPile != c.callPile {
			t.Errorf("PilePrice(%d, %d) piles = %d/%d, want %d/%d",
				c.toCall, c.pot, got.PotPile, got.CallPile, c.potPile, c.callPile)
		}
		if got.Phrase != c.phrase {
			t.Errorf("PilePrice(%d, %d) phrase = %q, want %q", c.toCall, c.pot, got.Phrase, c.phrase)
		}
	}
}

// TestPilePriceAwkwardRatio: 173 into 411 does not reduce; the piles
// approximate (larger scaled to 5) and the phrase normalizes to one.
func TestPilePriceAwkwardRatio(t *testing.T) {
	got := PilePrice(173, 411)
	if got.PotPile != 5 || got.CallPile != 2 {
		t.Errorf("piles = %d/%d, want 5/2", got.PotPile, got.CallPile)
	}
	if got.Phrase != "call 1 to win 2.4 — need 30%" {
		t.Errorf("phrase = %q", got.Phrase)
	}
	// The pile ratio must stay close to the true price even when scaled.
	truth := 411.0 / 173.0
	drawn := float64(got.PotPile) / float64(got.CallPile)
	if math.Abs(drawn-truth) > 0.35 {
		t.Errorf("drawn ratio %.2f strays from true %.2f", drawn, truth)
	}
}

func TestPilePriceNoBet(t *testing.T) {
	got := PilePrice(0, 120)
	if got.PotPile != 0 || got.CallPile != 0 || got.Required != 0 {
		t.Errorf("no-bet piles = %+v", got)
	}
	if got.Phrase != "no bet to call" {
		t.Errorf("no-bet phrase = %q", got.Phrase)
	}
}

// TestPilePriceRequiredMatchesPhrase: the struct's Required and the
// percentage inside the phrase are the same number — a renderer may use
// either without the two disagreeing.
func TestPilePriceRequiredMatchesPhrase(t *testing.T) {
	for _, c := range []struct{ toCall, pot engine.Chips }{
		{30, 45}, {25, 75}, {173, 411}, {640, 1060},
	} {
		got := PilePrice(c.toCall, c.pot)
		want := "need " + PctText(got.Required*100)
		if !strings.Contains(got.Phrase, want) {
			t.Errorf("PilePrice(%d, %d) phrase %q does not carry %q", c.toCall, c.pot, got.Phrase, want)
		}
	}
}
