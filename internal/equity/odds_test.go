package equity

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

func TestPotOdds(t *testing.T) {
	cases := []struct {
		toCall, pot int64
		want        float64
	}{
		{50, 150, 0.25},       // the design's worked example
		{20, 45, 20.0 / 65.0}, // the DESIGN.md §4 errata spot: ~31%
		{100, 100, 0.5},       // pot-sized bet
		{0, 100, 0},           // nothing to call
		{25, 0, 1},            // a bet into an empty pot needs certainty
	}
	for _, tc := range cases {
		if got := PotOdds(chips(tc.toCall), chips(tc.pot)); !approx(got, tc.want, 1e-12) {
			t.Errorf("PotOdds(%d, %d) = %v, want %v", tc.toCall, tc.pot, got, tc.want)
		}
	}
}

func TestOddsRatio(t *testing.T) {
	cases := []struct {
		toCall, pot  int64
		wantA, wantB int
	}{
		{50, 150, 3, 1},
		{20, 45, 9, 4}, // ≈ 2.2:1; display may round further
		{100, 100, 1, 1},
		{0, 100, 0, 0},
	}
	for _, tc := range cases {
		a, b := OddsRatio(chips(tc.toCall), chips(tc.pot))
		if a != tc.wantA || b != tc.wantB {
			t.Errorf("OddsRatio(%d, %d) = %d:%d, want %d:%d", tc.toCall, tc.pot, a, b, tc.wantA, tc.wantB)
		}
	}
}

func TestRequiredEquityText(t *testing.T) {
	if got, want := RequiredEquityText(50, 150), "risk 50 to win 150 → need 25%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// The corrected mockup-A spot: call 20 into 45 needs 31%, not 24%.
	if got, want := RequiredEquityText(20, 45), "risk 20 to win 45 → need 31%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Chips render with thousands separators.
	if got, want := RequiredEquityText(1000, 3000), "risk 1,000 to win 3,000 → need 25%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func chips(n int64) engine.Chips { return engine.Chips(n) }
