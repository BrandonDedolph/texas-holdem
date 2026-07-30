package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// benchView builds the worst-case common spot — a flop decision facing a
// bet, which pays for perception, an exact-or-sampled equity query, and an
// outs report — without the *testing.T helpers.
func benchView() *engine.PlayerView {
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: sb, BigBlind: bb, Seats: 6},
		Button: 0,
		Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000},
		Holes:  map[engine.Seat][2]engine.Card{0: engine.Holes("9s 8s")},
		Board:  engine.MustCards("Ks 7s 2h"),
		Seed:   1,
		Eval:   eval.Standard{},
	})
	if err != nil {
		panic(err)
	}
	for _, a := range []engine.Action{
		engine.Raise{S: 0, To: 25},
		engine.Call{S: 1},
		engine.Bet{S: 1, Amount: 25},
	} {
		if err := h.Apply(a); err != nil {
			panic(err)
		}
	}
	return h.View(0)
}

// BenchmarkDecideFlopFacingBet measures full decision latency in the spot
// the design budgets against (~50ms worst case, usually ~5ms —
// design-learning.md §4.1).
func BenchmarkDecideFlopFacingBet(b *testing.B) {
	v := benchView()
	strat := NewStrategy(ai.Baseline(), 7)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strat.Decide(v)
	}
}

// BenchmarkDecidePreflop measures the chart-only path — the latency every
// single bot turn pays.
func BenchmarkDecidePreflop(b *testing.B) {
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: sb, BigBlind: bb, Seats: 6},
		Button: 0,
		Stacks: func() map[engine.Seat]engine.Chips {
			m := map[engine.Seat]engine.Chips{}
			for s := engine.Seat(0); s < 6; s++ {
				m[s] = 1000
			}
			return m
		}(),
		Holes: map[engine.Seat][2]engine.Card{3: engine.Holes("Ad Jd")},
		Seed:  2,
		Eval:  eval.Standard{},
	})
	if err != nil {
		b.Fatal(err)
	}
	v := h.View(3)
	strat := NewStrategy(ai.Baseline(), 7)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strat.Decide(v)
	}
}
