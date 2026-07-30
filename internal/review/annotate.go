package review

// This file is the hindsight layer: what was actually true at each decision,
// computed with the revealed cards. Everything here is display data for a
// visually distinct "hindsight" region — it must never touch a FrozenGrade,
// and nothing here is an input to any grade. The separation is the point:
// the player has to be able to see, side by side, "what you knew" (frozen)
// and "what was true" (this file), and watch them disagree without the
// grade moving.

import (
	"fmt"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// Hindsight is the perfect-information view of one decision, computed at
// review time from the revealed cards. It never rewrites the frozen grade;
// the UI renders it dimmed and tagged "hindsight", spatially apart.
type Hindsight struct {
	// VillainHands are the actual holdings of every opponent still live at
	// this decision. Folded seats are excluded — they no longer contested
	// the pot the hero was pricing.
	VillainHands map[engine.Seat][2]engine.Card
	// TrueEquity is the hero's exact equity against those actual hands on
	// the board then visible. HasEquity is false when there was nothing to
	// compute against (unknown cards, no live opponents).
	TrueEquity float64
	HasEquity  bool
	// Note is a short factual hindsight sentence ("the price (25%) was right
	// against the real hand"). Facts about the truth, never a re-grade.
	Note string
}

// annotate fills every decision's Hindsight. Postflop heads-up queries are
// exact; multiway falls back to seeded Monte Carlo whose seed derives from
// the inputs, so a replayed hand always shows identical numbers.
func annotate(m *ReplayModel) {
	hero, heroKnown := m.Holes[m.HeroSeat]
	for i := range m.Decisions {
		d := &m.Decisions[i]
		h := Hindsight{VillainHands: map[engine.Seat][2]engine.Card{}}
		live := (m.DealtIn &^ d.Folded).Remove(m.HeroSeat)
		for _, s := range live.Seats() {
			if hole, ok := m.Holes[s]; ok {
				h.VillainHands[s] = hole
			}
		}
		if heroKnown && len(h.VillainHands) > 0 {
			h.TrueEquity, h.HasEquity = trueEquity(hero, h.VillainHands, d.Board)
		}
		if h.HasEquity {
			h.Note = hindsightNote(*d, h.TrueEquity)
		}
		d.Hindsight = h
	}
}

// trueEquity computes hero equity against the villains' ACTUAL hands.
// One villain goes through HandVsHand; multiway wraps each known hand in a
// single-combo Range for HandVsRanges. Seats are visited in ascending order
// so the query — and therefore any derived Monte Carlo seed — is
// deterministic despite the map.
func trueEquity(hero [2]engine.Card, villains map[engine.Seat][2]engine.Card, board []engine.Card) (float64, bool) {
	holes := make([][2]engine.Card, 0, len(villains))
	for s := engine.Seat(0); s.Valid(); s++ {
		if h, ok := villains[s]; ok {
			holes = append(holes, h)
		}
	}
	if len(holes) == 1 {
		return equity.HandVsHand(hero, holes[0], board, equity.Options{}).Equity, true
	}
	ranges := make([]equity.Range, len(holes))
	for i, h := range holes {
		var r equity.Range
		if err := r.Add(h[0].Code()+" "+h[1].Code(), 1); err != nil {
			return 0, false
		}
		ranges[i] = r
	}
	return equity.HandVsRanges(hero, ranges, board, equity.Options{}).Equity, true
}

// hindsightNote states one fact about the truth of the spot. Facing a bet it
// compares the price actually paid against the true equity — reconstruction
// arithmetic from the event log, not a grade; unpressured spots just say
// which side of the field the hero was on.
func hindsightNote(d DecisionFrame, trueEq float64) string {
	if d.ToCall > 0 {
		req := int(equity.PotOdds(d.ToCall, d.PotBefore)*100 + 0.5)
		if trueEq*100+0.5 >= float64(req) {
			return fmt.Sprintf("the price (%d%%) was right against the real hand", req)
		}
		return fmt.Sprintf("against the real hand the price (%d%%) was not there", req)
	}
	if trueEq >= 0.5 {
		return "you were ahead of the field here"
	}
	return "you were behind the field here"
}

// GoodBand reports whether a frozen band names a good decision. The review
// only ever CLASSIFIES the coach's spelling of the band ("Best"/"Good" vs
// the rest) for template selection — it never assigns a band itself.
func GoodBand(band string) bool {
	switch strings.ToLower(band) {
	case "best", "good":
		return true
	}
	return false
}

// DecisionOutcomeNote renders the decision-vs-outcome teaching line for one
// graded decision — the four quadrants of (good/bad decision) x (won/lost
// hand). This juxtaposition is the app's hardest lesson made explicit: two
// of the quadrants feel wrong on purpose, and those two carry the message.
//
// The decision axis comes from the FROZEN band only; the outcome axis from
// the hand's net result. Neither input ever modifies the other.
func DecisionOutcomeNote(g FrozenGrade, heroNetBB float64) string {
	good, won := GoodBand(g.Band), heroNetBB >= 0
	switch {
	case good && won:
		return "Right play, good result. Bank the pot, but keep score by the decision."
	case good && !won:
		return "Right play, bad result. Over time this play prints money."
	case !good && won:
		return fmt.Sprintf(
			"You got there and won %s this time; the play still loses money. Luck paid you for a mistake.",
			bbText(heroNetBB))
	default:
		return "Bad price, bad result. This time the cards agreed with the math."
	}
}

// bbText renders a big-blind amount for prose: "4.5bb".
func bbText(bb float64) string {
	if bb < 0 {
		bb = -bb
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", bb), ".0") + "bb"
}
