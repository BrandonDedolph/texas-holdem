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

func TestOddsToOneMatchesTheDesignMockups(t *testing.T) {
	// docs/design-tui.md mockup A: hero faces a raise to 30 with 45 in the pot
	// and 20 to call. The doc originally mis-stated this as 3.2:1 (24%); the
	// arithmetic is 45/20 = 2.25 and 20/65 = 30.8%.
	if got, want := OddsToOne(20, 45), "2.2:1"; got != want {
		t.Errorf("OddsToOne(20, 45) = %q, want %q", got, want)
	}
	if got, want := PotOddsText(20, 45), "2.2:1 (31%)"; got != want {
		t.Errorf("PotOddsText(20, 45) = %q, want %q", got, want)
	}
	// Mockup B: hero bets 120 into 185, villain calls 120 into 305.
	if got, want := PotOddsText(120, 305), "2.5:1 (28%)"; got != want {
		t.Errorf("PotOddsText(120, 305) = %q, want %q", got, want)
	}
	// Mockup C: 640 to call into 1,610.
	if got, want := OddsToOne(640, 1610), "2.5:1"; got != want {
		t.Errorf("OddsToOne(640, 1610) = %q, want %q", got, want)
	}
}

func TestOddsToOneWithNothingToCall(t *testing.T) {
	if got := OddsToOne(0, 100); got != "—" {
		t.Errorf("OddsToOne(0, 100) = %q, want an em dash", got)
	}
	if got := PotOddsText(0, 100); got != "no bet to call" {
		t.Errorf("PotOddsText(0, 100) = %q", got)
	}
}
