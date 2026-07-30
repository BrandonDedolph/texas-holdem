package equity

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// exactOpts forces exact enumeration even preflop-vs-range (~12-23M evals),
// so the oracle tests below compare against enumeration, not sampling.
var exactOpts = Options{MaxExactEvals: 60_000_000}

// TestKnownEquityOracles pins the canonical published numbers. These bands
// come from the standard references (combo-averaged, all-in preflop unless
// a board is given); if a result lands outside its band the equity engine
// is wrong — the band is never adjusted to fit.
func TestKnownEquityOracles(t *testing.T) {
	cases := []struct {
		name    string
		hero    string
		villain string // range spec
		board   string
		lo, hi  float64
	}{
		// AA vs KK ≈ 81-82% (published 81.9%).
		{"AA vs KK", "As Ah", "KK", "", 0.81, 0.825},
		// AKs vs QQ ≈ 46% (published 46.0%).
		{"AKs vs QQ", "As Ks", "QQ", "", 0.445, 0.475},
		// AA vs 72o ≈ 87-88% (published 88.2%).
		{"AA vs 72o", "As Ah", "72o", "", 0.865, 0.89},
		// A flopped flush draw vs top pair ≈ 34-38%.
		{"flush draw vs top pair", "Jh 3h", "KsQc", "Kh 8h 2c", 0.34, 0.38},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hero := engine.Holes(tc.hero)
			villain := mustRange(t, tc.villain)
			board := engine.MustCards(tc.board)
			res := HandVsRange(hero, villain, board, exactOpts)
			if !res.Exact {
				t.Fatalf("oracle should be exact, fell back to Monte Carlo")
			}
			if res.Equity < tc.lo || res.Equity > tc.hi {
				t.Fatalf("equity = %.4f, want within [%.3f, %.3f]", res.Equity, tc.lo, tc.hi)
			}
			t.Logf("equity = %.4f (win %.4f tie %.4f lose %.4f)", res.Equity, res.Win, res.Tie, res.Lose)
		})
	}
}

// TestPreflopHandVsHandIsExact pins the widened budget: with the measured
// 32ns evaluator, preflop hand vs hand (C(48,5) boards) fits the default
// exact budget — the design had routed this to a not-yet-generated table.
func TestPreflopHandVsHandIsExact(t *testing.T) {
	res := HandVsHand(engine.Holes("As Ah"), engine.Holes("Ks Kd"), nil, Options{})
	if !res.Exact {
		t.Fatal("preflop hand vs hand should be exact under the default budget")
	}
	if res.Samples != 0 {
		t.Fatalf("exact result reports %d samples, want 0", res.Samples)
	}
	if res.Equity < 0.80 || res.Equity > 0.84 {
		t.Fatalf("AsAh vs KsKd equity = %.4f, want ~0.82", res.Equity)
	}
}

// TestSumAndSymmetryInvariants: Win+Tie+Lose is 1, and equity is symmetric
// (hero vs villain == 1 - villain vs hero) on random exact spots.
func TestSumAndSymmetryInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for i := 0; i < 60; i++ {
		hero, villain, board := randomSpot(rng, []int{3, 4, 5}[i%3])
		a := HandVsHand(hero, villain, board, Options{})
		b := HandVsHand(villain, hero, board, Options{})
		if !a.Exact || !b.Exact {
			t.Fatal("postflop hand vs hand should always be exact")
		}
		if s := a.Win + a.Tie + a.Lose; math.Abs(s-1) > 1e-9 {
			t.Fatalf("Win+Tie+Lose = %v, want 1", s)
		}
		if math.Abs(a.Equity+b.Equity-1) > 1e-9 {
			t.Fatalf("equity not symmetric: %v + %v != 1", a.Equity, b.Equity)
		}
		if a.Win != b.Lose || a.Lose != b.Win || a.Tie != b.Tie {
			t.Fatalf("win/lose not mirrored: %+v vs %+v", a, b)
		}
	}
}

// TestMonteCarloAgreesWithExact: the sampled backend must land within 1.5%
// of enumeration across many random spots — the guarantee that a budget
// fallback never teaches a different number.
func TestMonteCarloAgreesWithExact(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 13))
	mcOpts := Options{MaxExactEvals: 1, MCSamples: 20_000}
	for i := 0; i < 100; i++ {
		hero, villain, board := randomSpot(rng, []int{3, 4, 5}[i%3])
		exact := HandVsHand(hero, villain, board, Options{})
		mc := HandVsHand(hero, villain, board, mcOpts)
		if mc.Exact {
			t.Fatal("MaxExactEvals=1 should force Monte Carlo")
		}
		if d := math.Abs(mc.Equity - exact.Equity); d > 0.015 {
			t.Fatalf("spot %d (%s %s | %s): MC %.4f vs exact %.4f, diff %.4f",
				i, engine.CardsString(hero[:]), engine.CardsString(villain[:]),
				engine.CardsString(board), mc.Equity, exact.Equity, d)
		}
	}
}

// TestDeterminism: same inputs, identical Result — twice, with the seed
// derived from the inputs. This is what lets a replayed hand show the same
// numbers.
func TestDeterminism(t *testing.T) {
	hero := engine.Holes("As Ah")
	villain := mustRange(t, "22+, A2s+, K9s+, QTs+, ATo+, KQo")
	// Preflop hand vs a wide range exceeds the exact budget, so this
	// exercises the Monte Carlo path with a derived seed.
	a := HandVsRange(hero, villain, nil, Options{})
	b := HandVsRange(hero, villain, nil, Options{})
	if a.Exact {
		t.Fatal("expected the Monte Carlo path")
	}
	if a != b {
		t.Fatalf("same inputs gave different results:\n%+v\n%+v", a, b)
	}
	// An explicit seed changes the samples but is itself reproducible.
	c := HandVsRange(hero, villain, nil, Options{Seed: 42})
	d := HandVsRange(hero, villain, nil, Options{Seed: 42})
	if c != d {
		t.Fatalf("seeded runs differ:\n%+v\n%+v", c, d)
	}
}

// TestDeadlineDegradesSamples: a Deadline must cut the sample count via the
// planner's cost model rather than blowing the budget.
func TestDeadlineDegradesSamples(t *testing.T) {
	hero := engine.Holes("As Ah")
	villain := mustRange(t, "22+, A2s+, K9s+, QTs+, ATo+, KQo")
	start := time.Now()
	res := HandVsRange(hero, villain, nil, Options{Deadline: 2 * time.Millisecond})
	elapsed := time.Since(start)
	if res.Exact {
		t.Fatal("expected the Monte Carlo path")
	}
	if res.Samples <= 0 || res.Samples >= DefaultMCSamples {
		t.Fatalf("Samples = %d, want degraded below the %d default", res.Samples, DefaultMCSamples)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("2ms deadline took %v", elapsed)
	}
}

// TestMultiway: adding a second villain must cost equity, ties must still
// account to 1, and small multiway spots enumerate exactly.
func TestMultiway(t *testing.T) {
	hero := engine.Holes("As Ah")
	board := engine.MustCards("2c 7d 9h")
	kk, qq := mustRange(t, "KK"), mustRange(t, "QQ")

	one := HandVsRange(hero, kk, board, Options{})
	two := HandVsRanges(hero, []Range{kk, qq}, board, Options{})
	if !two.Exact {
		t.Fatal("KK+QQ on a flop should enumerate exactly")
	}
	if s := two.Win + two.Tie + two.Lose; math.Abs(s-1) > 1e-9 {
		t.Fatalf("multiway Win+Tie+Lose = %v, want 1", s)
	}
	if two.Equity >= one.Equity {
		t.Fatalf("adding a villain raised equity: %v vs %v", two.Equity, one.Equity)
	}
	// The sampled multiway path must also work and stay in a sane band.
	mc := HandVsRanges(hero, []Range{kk, qq}, board, Options{MaxExactEvals: 1})
	if mc.Exact || mc.Samples == 0 {
		t.Fatalf("expected sampled multiway, got %+v", mc)
	}
	if math.Abs(mc.Equity-two.Equity) > 0.03 {
		t.Fatalf("sampled multiway %.4f far from exact %.4f", mc.Equity, two.Equity)
	}
}

// TestRangeVsRange: a range against itself is a coin flip, and small
// river spots are exact.
func TestRangeVsRange(t *testing.T) {
	board := engine.MustCards("2c 7d 9h Js 3s")
	aa, kk := mustRange(t, "AA"), mustRange(t, "KK")
	res := RangeVsRange(aa, kk, board, Options{})
	if !res.Exact {
		t.Fatal("AA vs KK on a river board should be exact")
	}
	if res.Equity < 0.99 { // no king, no ace: AA always holds
		t.Fatalf("AA vs KK on %s = %.4f, want ~1", engine.CardsString(board), res.Equity)
	}

	chart := mustRange(t, "22+, A2s+, K9s+, QTs+, ATo+, KQo")
	flop := engine.MustCards("Kd 8c 2h")
	mirror := RangeVsRange(chart, chart, flop, Options{})
	if mirror.Exact {
		t.Fatal("wide range vs range on a flop should sample")
	}
	if math.Abs(mirror.Equity-0.5) > 0.03 {
		t.Fatalf("range vs itself = %.4f, want ~0.5", mirror.Equity)
	}
}

// TestFlopHandVsRangeBudget asserts the coach's hottest path — flop hand vs
// a realistic range — is exact and lands well under the 100ms budget
// (design-learning.md §3.3). Measured: ~5ms.
func TestFlopHandVsRangeBudget(t *testing.T) {
	hero := engine.Holes("Ah Kd")
	villain := mustRange(t, "22+, A2s+, K9s+, QTs+, ATo+, KQo")
	board := engine.MustCards("Ks 7h 2c")
	start := time.Now()
	res := HandVsRange(hero, villain, board, Options{})
	elapsed := time.Since(start)
	if !res.Exact {
		t.Fatal("flop hand vs range should be exact under the default budget")
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("flop hand vs range took %v, budget is well under 100ms", elapsed)
	}
	t.Logf("flop hand vs 200-combo range: %v, equity %.4f", elapsed, res.Equity)
}

// randomSpot deals hero, villain, and a board of boardLen cards.
func randomSpot(rng *rand.Rand, boardLen int) (hero, villain [2]Card, board []Card) {
	perm := rng.Perm(engine.NumCards)
	hero = [2]Card{Card(perm[0]), Card(perm[1])}
	villain = [2]Card{Card(perm[2]), Card(perm[3])}
	for i := 0; i < boardLen; i++ {
		board = append(board, Card(perm[4+i]))
	}
	return
}

func BenchmarkFlopHandVsRange(b *testing.B) {
	hero := engine.Holes("Ah Kd")
	villain, _ := ParseRange("22+, A2s+, K9s+, QTs+, ATo+, KQo")
	board := engine.MustCards("Ks 7h 2c")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandVsRange(hero, villain, board, Options{})
	}
}

func BenchmarkTurnHandVsRange(b *testing.B) {
	hero := engine.Holes("Ah Kd")
	villain, _ := ParseRange("22+, A2s+, K9s+, QTs+, ATo+, KQo")
	board := engine.MustCards("Ks 7h 2c 9d")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandVsRange(hero, villain, board, Options{})
	}
}

func BenchmarkPreflopHandVsHand(b *testing.B) {
	hero, villain := engine.Holes("As Ah"), engine.Holes("Ks Kd")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandVsHand(hero, villain, nil, Options{})
	}
}
