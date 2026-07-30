package equity

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

func TestComboEncodingRoundTrip(t *testing.T) {
	seen := make(map[Combo]bool)
	for a := Card(0); a < engine.NumCards; a++ {
		for b := a + 1; b < engine.NumCards; b++ {
			c := MakeCombo(a, b)
			if MakeCombo(b, a) != c {
				t.Fatalf("MakeCombo not order-insensitive for %s %s", a.Code(), b.Code())
			}
			if int(c) >= NumCombos {
				t.Fatalf("combo index %d out of range", c)
			}
			if seen[c] {
				t.Fatalf("combo index %d assigned twice", c)
			}
			seen[c] = true
			cards := c.Cards()
			if cards[0] != b || cards[1] != a {
				t.Fatalf("Cards() = %v, want [%s %s]", cards, b.Code(), a.Code())
			}
		}
	}
	if len(seen) != NumCombos {
		t.Fatalf("got %d combos, want %d", len(seen), NumCombos)
	}
}

// TestWithoutCardRemoval is the reason Range is 1326 weights: holding an
// ace must cut AA from 6 combos to 3.
func TestWithoutCardRemoval(t *testing.T) {
	aa := mustRange(t, "AA")
	if got := aa.CountCombos(); got != 6 {
		t.Fatalf("AA = %v combos, want 6", got)
	}
	if got := aa.Without(engine.MustCard("As")).CountCombos(); got != 3 {
		t.Fatalf("AA without As = %v combos, want 3", got)
	}

	var full Range
	for i := range full.W {
		full.W[i] = 1
	}
	// Removing one card kills the 51 combos that contain it.
	if got := full.Without(engine.MustCard("7d")).CountCombos(); got != NumCombos-51 {
		t.Fatalf("full range without one card = %v combos, want %d", got, NumCombos-51)
	}
}

func TestContainsAndAdd(t *testing.T) {
	r := mustRange(t, "KQs")
	if !r.Contains(engine.Holes("Ks Qs")) {
		t.Fatal("KQs should contain KsQs")
	}
	if r.Contains(engine.Holes("Ks Qd")) {
		t.Fatal("KQs should not contain KsQd")
	}
	if err := r.Add("KQo", 0.5); err != nil {
		t.Fatal(err)
	}
	if !r.Contains(engine.Holes("Ks Qd")) {
		t.Fatal("after Add(KQo), range should contain KsQd")
	}
	if w := r.W[MakeCombo(engine.MustCard("Ks"), engine.MustCard("Qd"))]; w != 0.5 {
		t.Fatalf("Add weight = %v, want 0.5", w)
	}
}

func TestGridView(t *testing.T) {
	r := mustRange(t, "AA, AKs, AKo, [50]KQs")
	g := r.Grid()
	if g[0][0] != 1 {
		t.Fatalf("grid AA cell = %v, want 1", g[0][0])
	}
	if g[0][1] != 1 { // row A, col K, upper right = suited
		t.Fatalf("grid AKs cell = %v, want 1", g[0][1])
	}
	if g[1][0] != 1 { // row K, col A, lower left = offsuit
		t.Fatalf("grid AKo cell = %v, want 1", g[1][0])
	}
	if g[1][2] != 0.5 { // row K, col Q = KQs at half weight
		t.Fatalf("grid KQs cell = %v, want 0.5", g[1][2])
	}
	if g[2][1] != 0 { // KQo not in range
		t.Fatalf("grid KQo cell = %v, want 0", g[2][1])
	}
	// Card removal shows up as a partial cell: AA minus the As is 3 of 6.
	g = r.Without(engine.MustCard("As")).Grid()
	if g[0][0] != 0.5 {
		t.Fatalf("grid AA cell after removing As = %v, want 0.5", g[0][0])
	}
}

func mustRange(t *testing.T, s string) Range {
	t.Helper()
	r, err := ParseRange(s)
	if err != nil {
		t.Fatalf("ParseRange(%q): %v", s, err)
	}
	return r
}
