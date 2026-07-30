package rulebased

import (
	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// Postflop decision skeleton (design-learning.md §4.3). Deliberately
// simple and transparently teachable — each branch maps 1:1 to a lesson:
//
//   - facing a bet     → pot odds vs equity vs the perceived range
//   - hero aggressor   → the c-bet, gated by CBetFreq and board texture
//   - no initiative    → value-bet top pair+ honestly (no slowplay first),
//     semi-bluff strong draws
//   - river            → value or busted-combo-draw bluff, and NEVER a
//     bluff into a calling station (CallDown < 0.8 kills every bluff
//     branch — the "stop bluffing seat 4" lesson made executable)
func (s *Strategy) decidePostflop(c *ctx) ai.Decision {
	v := c.v

	cl := Classify(v.Hole, v.Board)
	aggressor := lastAggressor(v)
	heroAgg := aggressor == v.Seat
	villain := pickVillain(v, aggressor)
	villainRead := s.Read(villain)
	villainPos, hasVillainPos := c.pos[villain]
	ip := hasVillainPos && postflopOrder(c.heroPos) > postflopOrder(villainPos)

	c.add(
		ai.PositionFact{Pos: c.heroPos, InPosition: ip},
		ai.InitiativeFact{HeroIsAggressor: heroAgg},
		ai.ClassFact{Made: cl.Made, Draw: cl.Draw},
	)

	// One equity query per decision, against the primary opponent's
	// perceived range; every candidate score and every branch compare
	// reuses it. Multiway pots are scored against that single range —
	// coarse, and stated as such in the range fact's summary.
	vRange, summary := perceive(v, villain, villainRead)
	c.add(ai.RangeFact{Seat: villain, Range: vRange, Summary: summary})
	res := equity.HandVsRange(v.Hole, vRange, v.Board, s.eqOpt)
	eq := res.Equity
	method := "sampled"
	if res.Exact {
		method = "exact"
	}
	c.add(ai.EquityFact{Equity: eq, Method: method})

	// The outs inventory, against the same range the equity used, whenever
	// the hero is actually on a draw with cards to come.
	if cl.Draw != ai.NoDraw && v.Street != engine.River {
		c.add(ai.OutsFact{Report: equity.Outs(v.Hole, v.Board, &vRange)})
	}

	if v.ToCall > 0 {
		return s.facingBet(c, cl, eq, villain, villainRead)
	}
	return s.unraised(c, cl, eq, heroAgg, villain, villainRead)
}

// facingBet: the pot-odds lesson. Compare the price against equity vs the
// perceived range; raise the top of range for value and the strongest
// draws as semi-bluffs.
func (s *Strategy) facingBet(c *ctx, cl Classification, eq float64, villain engine.Seat, villainRead ai.Personality) ai.Decision {
	v := c.v
	facing := v.ToCall + v.Committed[v.Seat] // the bet's street-total size
	tiers := func(t SizeTier) engine.Chips { return raiseTo(t, facing) }
	cands := c.buildCandidates(eq, villainRead, tiers)

	required := equity.PotOdds(v.ToCall, v.Pot)
	c.add(ai.PotOddsFact{ToCall: v.ToCall, Pot: v.Pot, Required: required})

	// Implied-odds fudge: a draw to the nuts keeps winning bets when it
	// hits, so deep stacks are worth a fixed +4% equity credit — stated in
	// the rationale, never silent (design-learning.md §4.3).
	adjEq := eq
	if cl.Draw >= ai.FlushDraw && effectiveBehind(v, villain) >= 3*v.Pot {
		adjEq += 0.04
		c.add(ai.EquityFact{Equity: adjEq, Method: "implied-odds credit"})
	}

	// Value raise: two pair and better. Low-aggression personalities
	// (station) mostly just call even with it; that IS their leak.
	if cl.Made >= ai.TwoPair && (s.p.Aggression >= 0.8 || c.chance(s.p.Aggression*s.p.Aggression)) {
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addBetSizing(a, "value")
			return c.decision(a, cands)
		}
	}

	// The maniac's raise: high aggression turns strong one-pair hands into
	// raises the baseline never makes.
	if cl.Made >= ai.TopPair && s.p.Aggression >= 1.4 && c.chance(0.4*(s.p.Aggression-1)) {
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addBetSizing(a, "value")
			return c.decision(a, cands)
		}
	}

	// Semi-bluff raise: combo draws only, frequency-gated. A combo draw
	// has the equity to get raised and get called; lone gutshots do not.
	if v.Street != engine.River && cl.Draw == ai.ComboDraw && c.chance(0.7*s.p.BluffFreq) {
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addBetSizing(a, "semi-bluff")
			return c.decision(a, cands)
		}
	}

	if adjEq >= required*s.p.CallDown {
		return c.decision(engine.Call{S: v.Seat}, cands)
	}
	return c.decision(engine.Fold{S: v.Seat}, cands)
}

// unraised: nobody has bet this street.
func (s *Strategy) unraised(c *ctx, cl Classification, eq float64, heroAgg bool, villain engine.Seat, villainRead ai.Personality) ai.Decision {
	v := c.v
	tiers := func(t SizeTier) engine.Chips {
		return betAmount(betFraction(t, v.Street), v.Pot, v.Blinds.Big)
	}
	cands := c.buildCandidates(eq, villainRead, tiers)
	check := engine.Check{S: v.Seat}
	dry := boardDry(v.Board)
	stationRead := villainRead.CallDown > 0 && villainRead.CallDown < 0.8

	bet := func(tier SizeTier, purpose string) (ai.Decision, bool) {
		if preferredTier(s.p.Patience) == SizeLarge {
			tier = SizeLarge
		}
		if a, ok := c.aggressiveAt(tiers(tier)); ok {
			c.addBetSizing(a, purpose)
			return c.decision(a, cands), true
		}
		return ai.Decision{}, false
	}

	// Value: top pair and better bets — "beginner honesty", the baseline
	// does not slowplay, because the coach shouldn't teach slowplaying
	// first. Dry boards take the small size (nothing to charge), wet
	// boards and the river the fuller one.
	if cl.Made >= ai.TopPair {
		tier := SizeStandard
		if dry && v.Street != engine.River && cl.Made < ai.TwoPair {
			tier = SizeSmall
		}
		if d, ok := bet(tier, "value"); ok {
			return d
		}
		return c.decision(check, cands)
	}

	// River: no draws left to bet. Bluff only busted combo draws — the
	// hands with the best story and zero showdown value — at half the
	// personality's bluffing frequency, and never into a calling station.
	if v.Street == engine.River {
		if cl.Made <= ai.WeakPair && bustedComboDraw(v, cl) {
			if stationRead {
				c.add(ai.ArchetypeFact{Seat: villain, Key: villainRead.Key, Note: "calling station: don't bluff"})
				return c.decision(check, cands)
			}
			if p := 0.5 * s.p.BluffFreq; p >= bluffFloor && c.chance(p) {
				if d, ok := bet(SizeStandard, "bluff"); ok {
					return d
				}
			}
		}
		return c.decision(check, cands)
	}

	// Semi-bluff: strong draws bet for the double chance — fold now or
	// improve later. Frequency-gated by BluffFreq.
	if cl.Draw >= ai.OESD {
		gate := 0.6
		if heroAgg {
			gate = 0.75 // the c-bet halo: the aggressor's draws bet more often
		}
		if c.chance(gate * s.p.BluffFreq) {
			if d, ok := bet(SizeStandard, "semi-bluff"); ok {
				return d
			}
		}
		return c.decision(check, cands)
	}

	// The air c-bet: only as the aggressor, only on dry boards, gated by
	// CBetFreq — and killed by a station read, because a bet that never
	// folds anyone out is a donation.
	if heroAgg {
		if !dry {
			return c.decision(check, cands)
		}
		if stationRead {
			c.add(ai.ArchetypeFact{Seat: villain, Key: villainRead.Key, Note: "calling station: don't bluff"})
			return c.decision(check, cands)
		}
		if c.chance(0.6 * s.p.CBetFreq) {
			if d, ok := bet(SizeSmall, "bluff"); ok {
				return d
			}
		}
	}
	return c.decision(check, cands)
}

// bluffFloor rounds tiny bluffing frequencies down to never: a residual 5%
// river bluff teaches nothing but noise, so a personality whose halved
// BluffFreq lands below the floor (the station's 0.05) simply never
// bluffs the river — which is the testable claim the archetype makes.
const bluffFloor = 0.075

// bustedComboDraw reports whether the hero was on a combo draw on the turn
// that the river failed to complete — the only hands the baseline turns
// into river bluffs.
func bustedComboDraw(v *engine.PlayerView, cl Classification) bool {
	if len(v.Board) != 5 || cl.Made > ai.WeakPair {
		return false
	}
	turn := Classify(v.Hole, v.Board[:4])
	return turn.Draw == ai.ComboDraw
}

// lastAggressor returns the seat of the most recent bet or raise in the
// visible history, NoSeat if the hand has seen none (limped pots).
func lastAggressor(v *engine.PlayerView) engine.Seat {
	last := engine.NoSeat
	for _, e := range v.History {
		if e.Kind == engine.EvAction && (e.Action == engine.ActionBet || e.Action == engine.ActionRaise) {
			last = e.Seat
		}
	}
	return last
}

// pickVillain selects the primary opponent for perception and equity: the
// aggressor when there is one still in the hand, otherwise the first live
// opponent. v1 treats multiway pots as heads-up against this seat.
func pickVillain(v *engine.PlayerView, aggressor engine.Seat) engine.Seat {
	if aggressor != engine.NoSeat && aggressor != v.Seat && v.InHand.Has(aggressor) {
		return aggressor
	}
	for _, s := range v.InHand.Seats() {
		if s != v.Seat {
			return s
		}
	}
	return engine.NoSeat
}

// effectiveBehind is the stack still playable against the villain: the
// smaller of the two remaining stacks. Implied odds only pay if both
// players have chips left to pay them.
func effectiveBehind(v *engine.PlayerView, villain engine.Seat) engine.Chips {
	hero := v.Stacks[v.Seat] - v.ToCall
	if hero < 0 {
		hero = 0
	}
	if !villain.Valid() {
		return hero
	}
	if vs := v.Stacks[villain]; vs < hero {
		return vs
	}
	return hero
}

// addBetSizing records the chosen size as a fraction of the pot.
func (c *ctx) addBetSizing(a engine.Action, purpose string) {
	amount, _ := amountOf(a)
	risk := amount - c.v.Committed[c.v.Seat]
	pot := float64(c.v.Pot)
	if pot <= 0 {
		pot = float64(c.v.Blinds.Big)
	}
	c.add(ai.SizingFact{FractionOfPot: float64(risk) / pot, Purpose: purpose})
}
