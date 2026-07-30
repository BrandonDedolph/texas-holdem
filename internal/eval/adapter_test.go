package eval

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

func TestStandardSatisfiesEngineEvaluator(t *testing.T) {
	// The compile-time assertion is the point of this test; the body checks the
	// values survive the uint32 round trip the interface forces.
	var e engine.Evaluator = Standard{}

	board := engine.MustCards("Ks 7d 2c 9s 4h")
	strong := e.EvalHoldem(engine.Holes("Kh Kd"), board) // trip kings
	weak := e.EvalHoldem(engine.Holes("Ah 3c"), board)   // ace high

	if strong <= weak {
		t.Fatalf("trip kings (%d) should outrank ace high (%d)", strong, weak)
	}
	if got := Rank(strong).Category(); got != ThreeOfAKind {
		t.Errorf("Rank(strong).Category() = %v, want ThreeOfAKind", got)
	}
	if got, want := Rank(strong).Describe(), EvalHoldem(engine.Holes("Kh Kd"), board).Describe(); got != want {
		t.Errorf("round trip changed the description: %q vs %q", got, want)
	}
}

func TestEvaluatorVarIsUsable(t *testing.T) {
	board := engine.MustCards("As Ks Qs Js Ts")
	got := Rank(Evaluator.EvalHoldem(engine.Holes("2c 3d"), board))
	if want := "Royal Flush"; got.Describe() != want {
		t.Errorf("Describe() = %q, want %q", got.Describe(), want)
	}
}
