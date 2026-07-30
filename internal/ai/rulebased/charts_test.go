package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// TestChartsParse pins that every chart term is valid range grammar and
// expands to a nonempty, duplicate-free combo list. Charts are package
// data; a typo should fail the build's tests, not a live table.
func TestChartsParse(t *testing.T) {
	for _, c := range allCharts() {
		if len(c.Terms) == 0 || len(c.combos) == 0 {
			t.Errorf("chart %q is empty", c.Name)
		}
		seen := make(map[equity.Combo]bool, len(c.combos))
		for _, cb := range c.combos {
			if seen[cb] {
				t.Errorf("chart %q repeats combo %v", c.Name, cb)
			}
			seen[cb] = true
		}
		if got := c.Range().CountCombos(); int(got) != len(c.combos) {
			t.Errorf("chart %q Range has %.0f combos, combo list has %d", c.Name, got, len(c.combos))
		}
	}
}

// TestChartTargets pins the RFI percentage bands from design-learning.md
// §4.3 (UTG ~15%, HJ ~18%, CO ~26%, BTN ~42%, SB ~40%).
func TestChartTargets(t *testing.T) {
	bands := []struct {
		pos    engine.Position
		lo, hi float64
	}{
		{engine.PosUTG, 13, 17},
		{engine.PosHJ, 16, 20},
		{engine.PosCO, 23, 29},
		{engine.PosBTN, 38, 46},
		{engine.PosSB, 36, 44},
	}
	for _, b := range bands {
		pct := RFI[b.pos].Range().Percent()
		if pct < b.lo || pct > b.hi {
			t.Errorf("%s RFI = %.1f%%, want %.0f–%.0f%%", b.pos, pct, b.lo, b.hi)
		}
	}
}

// TestChartSpots pins the individual hands the design calls out: AA opens
// everywhere, 72o opens nowhere, A5s opens on the button but not UTG.
func TestChartSpots(t *testing.T) {
	aa := engine.Holes("As Ah")
	trash := engine.Holes("7s 2h")
	a5s := engine.Holes("As 5s")

	for pos, c := range RFI {
		r := c.Range()
		if !r.Contains(aa) {
			t.Errorf("%s RFI missing AA", pos)
		}
		if r.Contains(trash) {
			t.Errorf("%s RFI contains 72o", pos)
		}
	}
	if RFI[engine.PosUTG].Range().Contains(a5s) {
		t.Error("UTG RFI contains A5s; it should not")
	}
	if !RFI[engine.PosBTN].Range().Contains(a5s) {
		t.Error("BTN RFI missing A5s")
	}
	for _, c := range []*Chart{threeBetVsEarly, threeBetVsLate, fourBet} {
		if !c.Range().Contains(aa) {
			t.Errorf("chart %q missing AA", c.Name)
		}
	}
}

// TestScaledNesting pins the property RangeScale rests on: for any chart,
// a smaller scale yields a strict subset of a larger one — so the nit's
// raising range is strictly inside TAG's, and the maniac's strictly
// outside, for every chart in the game.
func TestScaledNesting(t *testing.T) {
	scales := []struct {
		key   string
		scale float64
	}{
		{"nit", ai.Archetypes["nit"].RangeScale},
		{"tag", ai.Archetypes["tag"].RangeScale},
		{"lag", ai.Archetypes["lag"].RangeScale},
		{"maniac", ai.Archetypes["maniac"].RangeScale},
	}
	for _, c := range allCharts() {
		prev := c.Scaled(scales[0].scale)
		for i := 1; i < len(scales); i++ {
			cur := c.Scaled(scales[i].scale)
			strict := false
			for cb := 0; cb < equity.NumCombos; cb++ {
				if prev.W[cb] > 0 && cur.W[cb] == 0 {
					t.Fatalf("chart %q: %s range not a subset of %s range",
						c.Name, scales[i-1].key, scales[i].key)
				}
				if cur.W[cb] > 0 && prev.W[cb] == 0 {
					strict = true
				}
			}
			if !strict {
				t.Errorf("chart %q: %s range equals %s range; scaling is decoration",
					c.Name, scales[i-1].key, scales[i].key)
			}
			prev = cur
		}
	}
}

// TestScaledKeepsStrongestFirst pins that narrowing keeps the top of the
// chart: the nit still plays AA in every chart that contains it. (The
// flat-call charts deliberately exclude AA — it 3-bets instead — so the
// check is conditional on the full chart.)
func TestScaledKeepsStrongestFirst(t *testing.T) {
	aa := engine.Holes("As Ah")
	nit := ai.Archetypes["nit"].RangeScale
	for _, c := range allCharts() {
		if c.Range().Contains(aa) && !c.Scaled(nit).Contains(aa) {
			t.Errorf("chart %q scaled to nit lost AA", c.Name)
		}
	}
}

// TestChenOrder pins the widening fallback: a strict, complete, duplicate-
// free ordering with the premium hands at the front and 72o at the back.
func TestChenOrder(t *testing.T) {
	if len(chenOrder) != equity.NumCombos {
		t.Fatalf("chen order has %d combos, want %d", len(chenOrder), equity.NumCombos)
	}
	seen := make(map[equity.Combo]bool, equity.NumCombos)
	for _, cb := range chenOrder {
		if seen[cb] {
			t.Fatalf("chen order repeats combo %v", cb)
		}
		seen[cb] = true
	}
	aa := equity.MakeCombo(engine.MustCard("As"), engine.MustCard("Ah"))
	if chenPct[aa] > 0.01 {
		t.Errorf("AA chen percentile %.3f, want ~0", chenPct[aa])
	}
	trash := equity.MakeCombo(engine.MustCard("7s"), engine.MustCard("2h"))
	if chenPct[trash] < 0.95 {
		t.Errorf("72o chen percentile %.3f, want ~1", chenPct[trash])
	}
}
