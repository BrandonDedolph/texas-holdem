package rulebased

import (
	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// preflopSpot summarizes the visible preflop action.
type preflopSpot struct {
	raises     int         // voluntary raises so far (opens are raises over the BB)
	limpers    int         // callers before the first raise
	opener     engine.Seat // first raiser, NoSeat if unopened
	lastRaiser engine.Seat // most recent raiser, NoSeat if unopened
	currentBet engine.Chips
}

func readPreflop(v *engine.PlayerView) preflopSpot {
	sp := preflopSpot{opener: engine.NoSeat, lastRaiser: engine.NoSeat}
	for _, e := range v.History {
		if e.Kind != engine.EvAction || e.Street != engine.Preflop {
			continue
		}
		switch e.Action {
		case engine.ActionRaise, engine.ActionBet:
			if sp.raises == 0 {
				sp.opener = e.Seat
			}
			sp.raises++
			sp.lastRaiser = e.Seat
		case engine.ActionCall:
			if sp.raises == 0 {
				sp.limpers++
			}
		}
	}
	sp.currentBet = v.ToCall + v.Committed[v.Seat]
	return sp
}

// decidePreflop is the chart-driven preflop game (design-learning.md §4.3).
// Charts decide; personality dials scale them. The baseline NEVER
// open-limps — limping exists only below the aggression floor (the
// station), so the coach can never see a limp recommended.
func (s *Strategy) decidePreflop(c *ctx) ai.Decision {
	v := c.v
	sp := readPreflop(v)

	c.add(ai.PositionFact{Pos: c.heroPos, InPosition: c.inPositionOverField()})

	switch {
	case sp.raises == 0 && v.ToCall == 0:
		return s.bbOption(c, sp)
	case sp.raises == 0:
		return s.openSpot(c, sp)
	case sp.raises == 1:
		return s.facingOpen(c, sp)
	case sp.raises == 2:
		return s.facingThreeBet(c, sp)
	default:
		return s.facingFourBetPlus(c, sp)
	}
}

// bbOption: the big blind checking its option or raising over limps. There
// is no RFI chart for the BB; the raise range is the late-position 3-bet
// chart because both answer "which hands want a bigger pot with no
// initiative information yet".
func (s *Strategy) bbOption(c *ctx, sp preflopSpot) ai.Decision {
	v := c.v
	tiers := func(t SizeTier) engine.Chips { return openTo(t, v.Blinds.Big, sp.limpers) }
	cands := c.buildCandidates(preflopEquityProxy(v.Hole), ai.Baseline(), tiers)

	raiseRange := ThreeBet(engine.PosBTN).Scaled(s.p.RangeScale)
	in := raiseRange.Contains(v.Hole)
	c.add(ai.ChartFact{Chart: "BB option raise", InRange: in})
	if in {
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addOpenSizing(a, "value")
			return c.decision(a, cands)
		}
	}
	return c.decision(engine.Check{S: v.Seat}, cands)
}

// openSpot: nobody has raised and the hero owes the blind price — the
// raise-first-in decision (plus isolation raises over limpers, same chart,
// bigger size). Raise or fold at baseline; the sub-0.7 aggression floor
// (station) limps the weaker chart hands instead.
func (s *Strategy) openSpot(c *ctx, sp preflopSpot) ai.Decision {
	v := c.v
	chart, ok := RFI[c.heroPos]
	if !ok {
		// Unreachable for legal spots (the BB never owes unopened); be
		// conservative rather than crash a live table.
		chart = rfiUTG
	}
	tiers := func(t SizeTier) engine.Chips { return openTo(t, v.Blinds.Big, sp.limpers) }
	cands := c.buildCandidates(preflopEquityProxy(v.Hole), ai.Baseline(), tiers)

	scaled := chart.Scaled(s.p.RangeScale)
	in := scaled.Contains(v.Hole)
	c.add(ai.ChartFact{Chart: chart.Name, InRange: in})

	passive := s.p.Aggression < aggressionFloor
	if in {
		if passive && !chart.Scaled(s.p.RangeScale*0.5).Contains(v.Hole) {
			// The station's limp: chart hand, but below the raising half.
			c.add(ai.ArchetypeFact{Seat: v.Seat, Key: s.p.Key, Note: "passive: limps where the baseline raises"})
			return c.decision(engine.Call{S: v.Seat}, cands)
		}
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addOpenSizing(a, "open")
			return c.decision(a, cands)
		}
	}
	if !in && passive && chart.Scaled(s.p.RangeScale*1.5).Contains(v.Hole) {
		c.add(ai.ArchetypeFact{Seat: v.Seat, Key: s.p.Key, Note: "passive: limps hands the baseline folds"})
		return c.decision(engine.Call{S: v.Seat}, cands)
	}
	return c.decision(engine.Fold{S: v.Seat}, cands)
}

// aggressionFloor is where "raise it or fold it" gives way to limping.
// Only the station (0.6) sits below it; the baseline and every aggressive
// archetype stay raise-or-fold.
const aggressionFloor = 0.7

// facingOpen: exactly one raise in front — 3-bet, flat, or fold.
func (s *Strategy) facingOpen(c *ctx, sp preflopSpot) ai.Decision {
	v := c.v
	openerPos := c.pos[sp.opener]
	ip := postflopOrder(c.heroPos) > postflopOrder(openerPos)
	tiers := func(t SizeTier) engine.Chips { return threeBetTo(t, sp.currentBet, ip) }
	villainRead := s.Read(sp.opener)
	cands := c.buildCandidates(preflopEquityProxy(v.Hole), villainRead, tiers)

	c.add(ai.PotOddsFact{ToCall: v.ToCall, Pot: v.Pot, Required: equity.PotOdds(v.ToCall, v.Pot)})
	r, summary := perceive(v, sp.opener, villainRead)
	c.add(ai.RangeFact{Seat: sp.opener, Range: r, Summary: summary})

	tb := ThreeBet(openerPos)
	if tb.Scaled(s.p.RangeScale).Contains(v.Hole) {
		c.add(ai.ChartFact{Chart: tb.Name, InRange: true})
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addOpenSizing(a, "3-bet")
			return c.decision(a, cands)
		}
	}

	defend := DefendCall(c.heroPos)
	if defend.Scaled(callScale(s.p)).Contains(v.Hole) {
		c.add(ai.ChartFact{Chart: defend.Name, InRange: true})
		return c.decision(engine.Call{S: v.Seat}, cands)
	}

	c.add(ai.ChartFact{Chart: defend.Name, InRange: false})
	return c.decision(engine.Fold{S: v.Seat}, cands)
}

// facingThreeBet: hero opened (or called) and a 3-bet arrived — 4-bet the
// premiums, call the continue range, fold the rest.
func (s *Strategy) facingThreeBet(c *ctx, sp preflopSpot) ai.Decision {
	v := c.v
	tiers := func(t SizeTier) engine.Chips { return fourBetTo(t, sp.currentBet) }
	villainRead := s.Read(sp.lastRaiser)
	cands := c.buildCandidates(preflopEquityProxy(v.Hole), villainRead, tiers)

	c.add(ai.PotOddsFact{ToCall: v.ToCall, Pot: v.Pot, Required: equity.PotOdds(v.ToCall, v.Pot)})
	r, summary := perceive(v, sp.lastRaiser, villainRead)
	c.add(ai.RangeFact{Seat: sp.lastRaiser, Range: r, Summary: summary})

	if FourBet().Scaled(s.p.RangeScale).Contains(v.Hole) {
		c.add(ai.ChartFact{Chart: FourBet().Name, InRange: true})
		if a, ok := c.aggressiveAt(tiers(preferredTier(s.p.Patience))); ok {
			c.addOpenSizing(a, "4-bet")
			return c.decision(a, cands)
		}
	}
	cont := VsThreeBetCall()
	if cont.Scaled(callScale(s.p)).Contains(v.Hole) {
		c.add(ai.ChartFact{Chart: cont.Name, InRange: true})
		return c.decision(engine.Call{S: v.Seat}, cands)
	}
	c.add(ai.ChartFact{Chart: cont.Name, InRange: false})
	return c.decision(engine.Fold{S: v.Seat}, cands)
}

// facingFourBetPlus: the pot has been raised three or more times. Premium
// pairs keep raising; the rest of the 4-bet chart calls; everything else
// folds. Deliberately blunt — a learner in this spot needs "you are
// usually against aces", not nuance.
func (s *Strategy) facingFourBetPlus(c *ctx, sp preflopSpot) ai.Decision {
	v := c.v
	tiers := func(t SizeTier) engine.Chips { return fourBetTo(t, sp.currentBet) }
	villainRead := s.Read(sp.lastRaiser)
	cands := c.buildCandidates(preflopEquityProxy(v.Hole), villainRead, tiers)

	c.add(ai.PotOddsFact{ToCall: v.ToCall, Pot: v.Pot, Required: equity.PotOdds(v.ToCall, v.Pot)})

	in := FourBet().Scaled(s.p.RangeScale).Contains(v.Hole)
	c.add(ai.ChartFact{Chart: FourBet().Name, InRange: in})
	if in {
		pocket := v.Hole[0].Rank() == v.Hole[1].Rank()
		if pocket && v.Hole[0].Rank() >= engine.King {
			if a, ok := c.aggressiveAt(tiers(SizeLarge)); ok {
				c.addOpenSizing(a, "value")
				return c.decision(a, cands)
			}
		}
		return c.decision(engine.Call{S: v.Seat}, cands)
	}
	return c.decision(engine.Fold{S: v.Seat}, cands)
}

// callScale widens a calling chart by the personality: RangeScale sets the
// base, CallDown discounts the price — a station (CallDown 0.6) flats far
// outside the printed chart, a nit (1.1) inside it.
func callScale(p ai.Personality) float64 {
	cd := p.CallDown
	if cd <= 0 {
		cd = 1
	}
	return p.RangeScale / cd
}

// aggressiveAt materializes a legal bet/raise at the desired street-total
// amount, clamped legal. ok is false when no aggressive option exists
// (raising rights closed); callers fall back to call/check.
func (c *ctx) aggressiveAt(want engine.Chips) (engine.Action, bool) {
	if opt, ok := c.v.Legal.Find(engine.ActionRaise); ok {
		return engine.Raise{S: c.v.Seat, To: clampSize(want, opt, c.v.Blinds.Big)}, true
	}
	if opt, ok := c.v.Legal.Find(engine.ActionBet); ok {
		return engine.Bet{S: c.v.Seat, Amount: clampSize(want, opt, c.v.Blinds.Big)}, true
	}
	return nil, false
}

// addOpenSizing records why the chosen aggressive action is its size.
func (c *ctx) addOpenSizing(a engine.Action, purpose string) {
	amount, _ := amountOf(a)
	pot := float64(c.v.Pot)
	if pot <= 0 {
		pot = float64(c.v.Blinds.Big)
	}
	c.add(ai.SizingFact{FractionOfPot: float64(amount) / pot, Purpose: purpose})
}

// inPositionOverField reports whether the hero closes the postflop action
// against every opponent still in the hand.
func (c *ctx) inPositionOverField() bool {
	ho := postflopOrder(c.heroPos)
	for _, seat := range c.v.InHand.Seats() {
		if seat == c.v.Seat {
			continue
		}
		if postflopOrder(c.pos[seat]) > ho {
			return false
		}
	}
	return true
}
