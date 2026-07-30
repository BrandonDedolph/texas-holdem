package rulebased

import (
	"fmt"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// PerceivedRange reconstructs a seat's range from its visible betting line —
// deliberately coarse in v1, and honest about it (design-learning.md §4.4).
//
// Preflop, the range is the chart for the line taken, scaled by the assumed
// personality: an UTG open is RFI[UTG] × RangeScale, a cold call is the
// defend chart widened by 1/CallDown (a station's calls mean less), a limp
// or a blind check is a Chen-order prefix because no chart was consulted.
// Each postflop bet or raise then filters the range to combos that connect
// with the board at that street (made ≥ top pair or draw ≥ OESD); a check
// caps it by removing TripsPlus. Combos conflicting with the observer's own
// cards and the board are removed last.
//
// assumed is the personality the observer attributes to the seat: the coach
// perceives bots with their actual personalities (the read is the lesson);
// bots perceive everyone as the TAG baseline.
func PerceivedRange(v *engine.PlayerView, seat engine.Seat, assumed ai.Personality) equity.Range {
	r, _ := perceive(v, seat, assumed)
	return r
}

// perceive is PerceivedRange plus the pre-rendered summary line the
// RangeFact carries.
func perceive(v *engine.PlayerView, seat engine.Seat, assumed ai.Personality) (equity.Range, string) {
	pos := positionsFromView(v)
	r, line := preflopLineRange(v, seat, pos, assumed)

	// Postflop filtering, street by street, against the board as that
	// street saw it. If a filter empties the range the line makes no sense
	// under our model — keep the previous street's range rather than
	// pretending to certainty we don't have.
	for st := engine.Flop; st <= v.Street && st <= engine.River; st++ {
		board := boardAt(v.Board, st)
		if len(board) == 0 {
			break
		}
		for _, e := range v.History {
			if e.Kind != engine.EvAction || e.Street != st || e.Seat != seat {
				continue
			}
			switch e.Action {
			case engine.ActionBet, engine.ActionRaise:
				if f := filterConnected(r, board); f.CountCombos() > 0 {
					r = f
				}
			case engine.ActionCheck:
				if f := filterCapped(r, board); f.CountCombos() > 0 {
					r = f
				}
			}
		}
	}

	dead := append([]engine.Card{v.Hole[0], v.Hole[1]}, v.Board...)
	r = r.Without(dead...)
	return r, fmt.Sprintf("≈%.0f%% range (%s)", r.Percent(), line)
}

// preflopLineRange assigns the preflop range for the seat's last preflop
// action, with the chart name for the summary.
func preflopLineRange(v *engine.PlayerView, seat engine.Seat, pos map[engine.Seat]engine.Position, assumed ai.Personality) (equity.Range, string) {
	seatPos := pos[seat]
	raisesBefore := 0
	lastKind := engine.ActionCheck // sentinel: "no voluntary action seen"
	lastRaiseNum := 0
	sawAction := false

	for _, e := range v.History {
		if e.Kind != engine.EvAction || e.Street != engine.Preflop {
			continue
		}
		if e.Seat == seat {
			sawAction = true
			lastKind = e.Action
			if e.Action == engine.ActionRaise || e.Action == engine.ActionBet {
				lastRaiseNum = raisesBefore + 1
			}
		}
		if e.Action == engine.ActionRaise || e.Action == engine.ActionBet {
			raisesBefore++
		}
	}

	scale := assumed.RangeScale
	switch {
	case sawAction && (lastKind == engine.ActionRaise || lastKind == engine.ActionBet):
		if lastRaiseNum >= 2 {
			return ThreeBet(seatPos).Scaled(scale), "3-bet preflop"
		}
		if c, ok := RFI[seatPos]; ok {
			return c.Scaled(scale), "opened from " + seatPos.String()
		}
		// A BB raise over limps has no RFI chart; it is a value line.
		return ThreeBet(engine.PosBTN).Scaled(scale), "raised from the BB"

	case sawAction && lastKind == engine.ActionCall:
		if lastRaiseNum >= 1 {
			// Raised earlier, then called a re-raise: the continue range.
			return VsThreeBetCall().Scaled(scale), "called a 3-bet"
		}
		if !sawRaiseBeforeCall(v, seat) {
			// Limped: no chart consulted, so no chart to cite. Wide.
			return chenPrefix(clamp01x(0.45*scale, 0.95)), "limped"
		}
		callScale := scale
		if assumed.CallDown > 0 {
			callScale /= assumed.CallDown
		}
		return DefendCall(seatPos).Scaled(callScale), "called an open"

	default:
		// Checked the big blind option, or no voluntary action yet: nearly
		// anything minus the hands that would have raised.
		return chenPrefix(0.85), "no preflop action"
	}
}

// sawRaiseBeforeCall reports whether the seat's last preflop call came
// after a raise (a cold call) rather than before any (a limp).
func sawRaiseBeforeCall(v *engine.PlayerView, seat engine.Seat) bool {
	raised := false
	calledAfterRaise := false
	for _, e := range v.History {
		if e.Kind != engine.EvAction || e.Street != engine.Preflop {
			continue
		}
		switch e.Action {
		case engine.ActionRaise, engine.ActionBet:
			raised = true
		case engine.ActionCall:
			if e.Seat == seat {
				calledAfterRaise = raised
			}
		}
	}
	return calledAfterRaise
}

// filterConnected keeps combos whose hand connects with the board: made
// top pair or better, or drawing to an OESD or better. This is the v1
// model of "a bet means something".
func filterConnected(r equity.Range, board []engine.Card) equity.Range {
	var out equity.Range
	for _, wc := range r.Combos() {
		cc := wc.Combo.Cards()
		hole := [2]engine.Card{cc[0], cc[1]}
		if conflicts(hole, board) {
			continue
		}
		cl := Classify(hole, board)
		if cl.Made >= ai.TopPair || cl.Draw >= ai.OESD {
			out.W[wc.Combo] = wc.Weight
		}
	}
	return out
}

// filterCapped removes TripsPlus: a check caps the range at two pair in
// the v1 model (baseline never slowplays, so its checks are honest).
func filterCapped(r equity.Range, board []engine.Card) equity.Range {
	var out equity.Range
	for _, wc := range r.Combos() {
		cc := wc.Combo.Cards()
		hole := [2]engine.Card{cc[0], cc[1]}
		if conflicts(hole, board) {
			continue
		}
		if Classify(hole, board).Made < ai.TripsPlus {
			out.W[wc.Combo] = wc.Weight
		}
	}
	return out
}

func conflicts(hole [2]engine.Card, board []engine.Card) bool {
	for _, b := range board {
		if b == hole[0] || b == hole[1] {
			return true
		}
	}
	return false
}

// boardAt truncates the full visible board to what a street saw.
func boardAt(board []engine.Card, st engine.Street) []engine.Card {
	n := 0
	switch st {
	case engine.Flop:
		n = 3
	case engine.Turn:
		n = 4
	case engine.River:
		n = 5
	}
	if n > len(board) {
		n = len(board)
	}
	return board[:n]
}

// positionsFromView recovers the position map from the visible history:
// the dealt-in set is exactly the seats with an EvDealHole event (the cards
// are blanked for opponents, the events are not).
func positionsFromView(v *engine.PlayerView) map[engine.Seat]engine.Position {
	var dealtIn engine.SeatSet
	for _, e := range v.History {
		if e.Kind == engine.EvDealHole {
			dealtIn = dealtIn.Add(e.Seat)
		}
	}
	return engine.Positions(dealtIn, v.Button)
}

func clamp01x(v, hi float64) float64 {
	if v > hi {
		return hi
	}
	if v < 0.05 {
		return 0.05
	}
	return v
}
