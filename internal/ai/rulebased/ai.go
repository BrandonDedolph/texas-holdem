package rulebased

import (
	"math/rand/v2"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// AI is a Personality wired to the one rule-based Strategy: the thing you
// seat at a table. It implements ai.Player.
type AI struct {
	name  string
	strat *Strategy
}

// New builds a Player from a personality. seed makes the bot's mixed
// decisions (bluff-or-not) reproducible: the same seed replays the same
// hand identically, which tests and the review screen rely on.
func New(name string, p ai.Personality, seed int64) *AI {
	return &AI{name: name, strat: NewStrategy(p, seed)}
}

// Name implements ai.Player.
func (a *AI) Name() string { return a.name }

// Act implements ai.Player: decide, then act on the decision. The full
// Decision (candidates, rationale) is available through Strategy for
// callers that need more than the action.
func (a *AI) Act(v *engine.PlayerView) engine.Action {
	return a.strat.Decide(v).Action
}

// Personality returns the parameter block this bot runs.
func (a *AI) Personality() ai.Personality { return a.strat.p }

// Strategy exposes the underlying strategy — the coach advises by calling
// Decide on the same object family the bots run.
func (a *AI) Strategy() *Strategy { return a.strat }

// Strategy is the one rule-based implementation of ai.Strategy. All
// archetypes and the coach are this type with different Personality values.
type Strategy struct {
	p     ai.Personality
	seed  int64
	reads map[engine.Seat]ai.Personality
	eqOpt equity.Options
}

// NewStrategy builds the strategy for a personality. seed drives the mixed
// decisions; see Decide.
func NewStrategy(p ai.Personality, seed int64) *Strategy {
	return &Strategy{
		p:    p,
		seed: seed,
		// Equity budget: narrow ranges enumerate exactly (a flop hand vs a
		// 210-combo open is ~230k evals, ~5ms measured); only the very wide
		// perception fallbacks fall through to seeded Monte Carlo, which is
		// still deterministic per spot. This keeps every Decide comfortably
		// under the ~50ms worst case budgeted in design-learning.md §4.1.
		eqOpt: equity.Options{MaxExactEvals: 300_000, MCSamples: 5_000},
	}
}

// SetRead attributes a personality to a seat. The coach sets the actual
// archetypes here (the app knows them, and the hero is told the read — the
// read is the lesson, not cheating); bots get no reads and assume everyone
// plays the TAG baseline. Reads change perception only, never the cards
// anyone can see.
func (s *Strategy) SetRead(seat engine.Seat, p ai.Personality) {
	if s.reads == nil {
		s.reads = make(map[engine.Seat]ai.Personality)
	}
	s.reads[seat] = p
}

// Read returns the personality assumed for a seat: the set read, or the
// TAG baseline.
func (s *Strategy) Read(seat engine.Seat) ai.Personality {
	if p, ok := s.reads[seat]; ok {
		return p
	}
	return ai.Baseline()
}

// Decide implements ai.Strategy. It must be called on the acting seat's
// view (v.Legal non-empty); deciding for a seat that has no decision is a
// caller bug.
//
// Determinism: every frequency decision ("c-bet this air 60% of the time")
// draws from a PCG seeded by hash(strategy seed, seat, street, hole,
// board). The same view under the same seed therefore produces the same
// Decision, byte for byte — a replayed hand is identical, and tests can
// pin mixed behavior.
func (s *Strategy) Decide(v *engine.PlayerView) ai.Decision {
	if v == nil || len(v.Legal) == 0 {
		panic("rulebased: Decide called without a pending decision")
	}
	c := s.newCtx(v)
	if v.Street == engine.Preflop {
		return s.decidePreflop(c)
	}
	return s.decidePostflop(c)
}

// ctx is one decision's working state: the view, the seeded generator, the
// fact accumulator, and the cached equity query — computed once, consumed
// by the branch logic and every scored candidate.
type ctx struct {
	v   *engine.PlayerView
	p   ai.Personality
	rng *rand.Rand

	pos     map[engine.Seat]engine.Position
	heroPos engine.Position

	rationale ai.Rationale

	// scorer is installed by buildCandidates so decision() can score a
	// chosen action that discretization missed (it never should — the
	// branches pick from the same tier table — but a fallback beats a
	// panic in a released game).
	scorer func(engine.Action) float64
}

func (s *Strategy) newCtx(v *engine.PlayerView) *ctx {
	pos := positionsFromView(v)
	return &ctx{
		v:       v,
		p:       s.p,
		rng:     rand.New(rand.NewPCG(s.mixSeed(v), pcgSalt)),
		pos:     pos,
		heroPos: pos[v.Seat],
	}
}

// pcgSalt mirrors the engine's fixed second PCG word; any constant works,
// it just has to be the same everywhere forever.
const pcgSalt = 0x9e3779b97f4a7c15

// mixSeed hashes (strategy seed, seat, street, hole, board) with FNV-1a.
// Hole+board identify the hand for replay purposes: the engine does not
// expose a hand ID through PlayerView, and the cards are exactly as unique
// and exactly as replayable.
func (s *Strategy) mixSeed(v *engine.PlayerView) uint64 {
	h := uint64(14695981039346656037)
	put := func(b byte) {
		h ^= uint64(b)
		h *= 1099511628211
	}
	for i := 0; i < 8; i++ {
		put(byte(uint64(s.seed) >> (8 * i)))
	}
	put(byte(v.Seat))
	put(byte(v.Street))
	put(byte(v.Hole[0]))
	put(byte(v.Hole[1]))
	for _, c := range v.Board {
		put(byte(c))
	}
	return h
}

// add appends facts to the decision's rationale, in salience order.
func (c *ctx) add(facts ...ai.Fact) { c.rationale.Add(facts...) }

// chance draws one mixed decision at probability p (clamped to [0,1]).
func (c *ctx) chance(p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return c.rng.Float64() < p
}

// decision assembles the final Decision, guaranteeing the chosen action
// appears among the candidates.
func (c *ctx) decision(chosen engine.Action, cands []ai.ScoredAction) ai.Decision {
	found := false
	for _, sc := range cands {
		if sameAction(sc.Action, chosen) {
			found = true
			break
		}
	}
	if !found {
		score := 0.0
		if c.scorer != nil {
			score = c.scorer(chosen)
		}
		cands = append(cands, ai.ScoredAction{Action: chosen, ScoreBB: score})
	}
	return ai.Decision{Action: chosen, Candidates: cands, Rationale: c.rationale}
}

func sameAction(a, b engine.Action) bool {
	if a.Type() != b.Type() || a.Seat() != b.Seat() {
		return false
	}
	am, _ := amountOf(a)
	bm, _ := amountOf(b)
	return am == bm
}

func amountOf(a engine.Action) (engine.Chips, bool) {
	switch v := a.(type) {
	case engine.Bet:
		return v.Amount, true
	case engine.Raise:
		return v.To, true
	}
	return 0, false
}

// --- Candidate scoring -------------------------------------------------------

// buildCandidates scores every legal action class (design-learning.md
// §4.1): fold, check/call, three discretized aggressive sizes from tiers,
// and all-in when the stack is under twice the standard raise.
//
// The scores are the ScoreBB proxy documented on ai.ScoredAction — pot
// share from eq minus chips risked plus a coarse fold-equity term — NOT a
// solved EV. eq is the decision's one cached equity (a chart percentile
// proxy preflop, hand-vs-perceived-range postflop), so scoring five
// candidates costs zero additional equity queries.
//
// villain carries the personality read used for the fold-equity estimate;
// pass the baseline when no single opponent is identified.
func (c *ctx) buildCandidates(eq float64, villain ai.Personality, tiers func(SizeTier) engine.Chips) []ai.ScoredAction {
	v := c.v
	bb := float64(v.Blinds.Big)
	pot := float64(v.Pot)
	if pot < bb {
		pot = bb
	}

	// score computes the proxy for any concrete action; also installed on
	// the ctx for decision()'s fallback path.
	score := func(a engine.Action) float64 {
		switch a.Type() {
		case engine.ActionFold:
			// Convention: scores are chips gained relative to folding now;
			// chips already committed are sunk either way.
			return 0
		case engine.ActionCheck:
			return eq * pot / bb
		case engine.ActionCall:
			call := float64(v.Legal.CallAmount())
			return (eq*(pot+call) - call) / bb
		default:
			amount, _ := amountOf(a)
			risk := float64(amount - v.Committed[v.Seat])
			match := float64(amount) - maxOpponentCommitted(v)
			if match < 0 {
				match = 0
			}
			if match > risk {
				match = risk
			}
			fe := foldEquity(risk/pot, villain)
			// A caller's range is stronger than their checking range;
			// shave the equity we realize when called. Constant shave —
			// consistent and monotone, which is all the proxy promises.
			eqc := eq - 0.06
			if eqc < 0 {
				eqc = 0
			}
			return (fe*pot + (1-fe)*(eqc*(pot+risk+match)-risk)) / bb
		}
	}
	c.scorer = score

	var out []ai.ScoredAction
	seen := make(map[string]bool, 8)
	push := func(a engine.Action) {
		am, _ := amountOf(a)
		key := a.Type().String() + "/" + am.String()
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, ai.ScoredAction{Action: a, ScoreBB: score(a)})
	}

	for _, opt := range v.Legal {
		switch opt.Type {
		case engine.ActionFold:
			push(engine.Fold{S: v.Seat})
		case engine.ActionCheck:
			push(engine.Check{S: v.Seat})
		case engine.ActionCall:
			push(engine.Call{S: v.Seat})
		case engine.ActionBet, engine.ActionRaise:
			std := clampSize(tiers(SizeStandard), opt, v.Blinds.Big)
			for _, tier := range []SizeTier{SizeSmall, SizeStandard, SizeLarge} {
				push(c.aggressive(opt, clampSize(tiers(tier), opt, v.Blinds.Big)))
			}
			// All-in joins the slate when the standard raise commits the
			// stack anyway (design-learning.md §4.1).
			if opt.Max < 2*std {
				push(c.aggressive(opt, opt.Max))
			}
		}
	}
	return out
}

// aggressive materializes a bet-or-raise at a street-total amount.
func (c *ctx) aggressive(opt engine.ActionOption, amount engine.Chips) engine.Action {
	if opt.Type == engine.ActionBet {
		return engine.Bet{S: c.v.Seat, Amount: amount}
	}
	return engine.Raise{S: c.v.Seat, To: amount}
}

// foldEquity is the proxy's coarse "how often does a bet win outright"
// estimate: rising in bet size with diminishing returns, and nearly zero
// against an opponent read as unbluffable (CallDown < 0.8) — the scoring
// side of the never-bluff-a-station rule.
func foldEquity(frac float64, villain ai.Personality) float64 {
	if frac < 0 {
		frac = 0
	}
	fe := 0.18 + 0.5*frac/(frac+1)
	if fe > 0.55 {
		fe = 0.55
	}
	if villain.CallDown > 0 && villain.CallDown < 0.8 {
		fe *= 0.25
	}
	return fe
}

func maxOpponentCommitted(v *engine.PlayerView) float64 {
	var best engine.Chips
	for s := engine.Seat(0); s < engine.MaxSeats; s++ {
		if s != v.Seat && v.Committed[s] > best {
			best = v.Committed[s]
		}
	}
	return float64(best)
}

// preflopEquityProxy maps a hand's Chen-order percentile onto a plausible
// showdown-equity scale (AA ≈ 0.85 vs one random hand, trash ≈ 0.25). It
// exists ONLY to give preflop candidates comparable ScoreBB values without
// a 169×169 equity table; it is never emitted as an EquityFact — preflop
// rationales cite the chart, because the chart is what decided.
func preflopEquityProxy(hole [2]engine.Card) float64 {
	return 0.85 - 0.60*chenPct[equity.MakeCombo(hole[0], hole[1])]
}

// postflopOrder ranks positions by postflop action order: the blinds act
// first, the button last. Higher acts later — is "in position".
func postflopOrder(p engine.Position) int {
	switch p {
	case engine.PosSB:
		return 0
	case engine.PosBB:
		return 1
	case engine.PosUTG:
		return 2
	case engine.PosHJ:
		return 3
	case engine.PosCO:
		return 4
	default:
		return 5 // BTN
	}
}
