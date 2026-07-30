package eval

import "github.com/BrandonDedolph/texas-holdem/internal/engine"

// Standard is the production engine.Evaluator, backed by Eval7.
//
// The engine cannot import this package — the dependency edge is eval → engine
// (for the Card type), so importing back would be a cycle. The engine therefore
// declares an Evaluator interface and takes an implementation by injection;
// this adapter is that implementation, and it lives here because this is the
// side of the edge that can see both types.
//
// The interface traffics in uint32 rather than HandRank for the same reason:
// the engine has no way to name HandRank. The values are HandRanks, and the
// engine only ever compares them, so widening to uint32 loses nothing.
type Standard struct{}

// EvalHoldem ranks hole cards against a board. It satisfies engine.Evaluator.
func (Standard) EvalHoldem(hole [2]engine.Card, board []engine.Card) uint32 {
	return uint32(EvalHoldem(hole, board))
}

// Rank converts a raw rank from an engine event or result back into a
// HandRank, so callers can call Describe() on it.
func Rank(raw uint32) HandRank { return HandRank(raw) }

// Evaluator is the package-level instance to inject:
//
//	table := engine.NewTable(cfg, eval.Evaluator)
var Evaluator engine.Evaluator = Standard{}
