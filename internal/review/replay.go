// Package review reconstructs a finished hand for the post-hand review
// screen (docs/design-learning.md §7).
//
// This package is the one layer that knows everything: every seat's hole
// cards, the full runout, and the result. That knowledge is exactly why the
// package's core rule exists — grades are FROZEN at decision time by the
// coach, from the hero's information set only, and this package renders them
// verbatim. Replay never recomputes, adjusts, or re-words a grade; hindsight
// (revealed cards, true equities) is carried in a separate Hindsight value
// that the UI renders visually apart from the frozen grade. If the review
// used its perfect information to re-grade, the app's central teaching claim
// — "grade the decision, not the outcome" — would be a lie
// (docs/DESIGN.md §1 principle 1).
package review

import (
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// FrozenGrade is the review's view of one graded hero decision, exactly as
// the coach computed it before the next card was dealt. The review treats
// every field as opaque display data: values in equal values out, byte for
// byte (TestReplayRendersFrozenGradesVerbatim pins this).
//
// TODO(wire-coach): this is a local view model of coach.GradedDecision +
// coach.Advice, defined here because internal/coach is being built in
// parallel. When it lands, the table screen maps its types onto this shape;
// nothing in this package changes.
type FrozenGrade struct {
	// Band is the grade band name as the coach spelled it
	// ("Best", "Good", "Inaccuracy", "Mistake", "Blunder").
	Band string `json:"band"`
	// EVLossBB is the decision's EV leak in big blinds, ≥ 0. The summary
	// ledger sums these — "where EV was lost" means "which decisions
	// leaked", never "which pots were lost".
	EVLossBB float64 `json:"ev_loss_bb"`
	// Body is the coach's frozen explanation — the "Then:" line, citing only
	// what the hero knew ("needed 25%, had ~31% vs his likely range").
	Body string `json:"body"`
	// Recommended is the action the coach recommended at the time ("call").
	Recommended string `json:"recommended"`
}

// HandAnnotations rides alongside a HandRecord: the coach's frozen output,
// keyed by the event index of the hero action it grades
// (docs/design-learning.md §6).
type HandAnnotations struct {
	HandID string `json:"hand_id"`
	// Grades maps the event index of a hero EvAction to its frozen grade.
	// Decisions without an entry render ungraded — the review never fills
	// the gap itself.
	Grades map[int]FrozenGrade `json:"grades,omitempty"`
}

// Archive is the persisted {record, annotations} pair — one JSONL line in
// the profile store's hands/ directory (docs/design-learning.md §6).
type Archive struct {
	Record      engine.HandRecord `json:"record"`
	Annotations HandAnnotations   `json:"annotations"`
}

// NewRecord assembles the persisted HandRecord wrapper around a hand's event
// log. The events are copied verbatim — the engine's log is the single
// source of truth and nothing downstream may edit it. The caller supplies
// identity (ID, time) and the seat roster, which are table-session concerns
// the engine does not know.
func NewRecord(h *engine.Hand, id string, at time.Time, seats []engine.SeatInfo) engine.HandRecord {
	return engine.HandRecord{
		ID:     id,
		Time:   at,
		Blinds: h.Blinds(),
		Seats:  append([]engine.SeatInfo(nil), seats...),
		Events: append([]engine.Event(nil), h.Events()...),
	}
}

// StreetFrame is the reconstructed state of one street at the moment its
// betting closed: the board then visible, the pot, every stack, and who had
// folded. Built purely from the event log — the replay must be able to
// stand on the persisted record alone, with no live Hand in reach.
type StreetFrame struct {
	Street engine.Street
	Board  []engine.Card
	// PotAfter is the pot when the street's betting closed (for the final
	// street: the full pot, before refunds and awards).
	PotAfter engine.Chips
	Stacks   [engine.MaxSeats]engine.Chips
	Folded   engine.SeatSet
}

// DecisionFrame is one hero decision — the review's spine. Everything except
// Frozen and Hindsight is the reconstructed table state at the instant the
// decision was made (before the action's chips moved).
type DecisionFrame struct {
	EventIdx int // index of the hero's EvAction in Record.Events
	Street   engine.Street
	Action   engine.ActionType
	// Amount is the hero's street-total commitment after the action, the
	// engine's convention. Paid is the chips this action actually added.
	Amount engine.Chips
	Paid   engine.Chips
	// ToCall is the price the hero faced; 0 when checking was legal.
	ToCall    engine.Chips
	PotBefore engine.Chips
	Board     []engine.Card
	Stacks    [engine.MaxSeats]engine.Chips
	Committed [engine.MaxSeats]engine.Chips // this street, before the action
	Folded    engine.SeatSet

	// Frozen is the coach's grade exactly as handed in via HandAnnotations,
	// or nil when the decision was never graded. Rendered verbatim; the
	// review computes nothing into it.
	Frozen *FrozenGrade

	// Hindsight is the perfect-information layer, computed here and now WITH
	// the revealed cards. It lives in its own field — never merged into
	// Frozen — so the UI can keep "what you knew" and "what was true"
	// visually separate.
	Hindsight Hindsight
}

// Award is one pot payout, per winner per pot layer.
type Award struct {
	Seat   engine.Seat
	Amount engine.Chips
	Pot    int // pot layer index; 0 is the main pot
}

// Outcome is the hand's ending: the full board, final stacks, and who was
// paid. It is deliberately a separate struct from Summary's grade ledger —
// the outcome is variance's column, the grades are the player's.
type Outcome struct {
	Board    []engine.Card
	Stacks   [engine.MaxSeats]engine.Chips
	Folded   engine.SeatSet
	Awards   []Award
	Showdown bool
	// WinnerSeat is the main pot's (first) winner; NoSeat if no award was
	// recorded. WinnerDesc is the evaluator's English for the winning hand.
	WinnerSeat engine.Seat
	WinnerDesc string
	// WinningFive is the exact five of the winner's seven cards that play,
	// board cards included (eval.Best5) — the "best five of seven" lesson.
	// Nil when the pot was conceded without a showdown.
	WinningFive []engine.Card
}

// Summary is the hand's two scoreboards, kept deliberately unaligned:
// the EV ledger sums frozen decision losses (skill), and the net result
// stands apart (variance). Plotting them diverging is the variance lesson.
type Summary struct {
	HeroNet   engine.Chips
	HeroNetBB float64
	// GradeCounts counts frozen grades by band name; Graded is the number of
	// decisions that carried a grade at all.
	GradeCounts map[string]int
	Graded      int
	// EVLossBB is the sum of the frozen per-decision EV losses — "you leaked
	// 2.5bb this hand" — never a function of the result.
	EVLossBB float64
	// KeyDecision indexes the largest-EV-loss decision in Decisions,
	// -1 when nothing was graded.
	KeyDecision int
}

// ReplayModel is the street-by-street reconstruction of one recorded hand,
// with the hero's frozen grades attached and the hindsight layer computed.
type ReplayModel struct {
	Record   engine.HandRecord
	HeroSeat engine.Seat
	Button   engine.Seat
	DealtIn  engine.SeatSet
	// Holes is every seat's hole cards — the knowledge that makes this the
	// hindsight layer. It exists for display only; nothing here feeds a grade.
	Holes map[engine.Seat][2]engine.Card

	Streets   []StreetFrame
	Decisions []DecisionFrame
	Outcome   Outcome
	Summary   Summary
}

// Replay builds the review model from a persisted record and its frozen
// annotations. It walks the event log once, snapshotting table state at
// every hero decision and street close, then attaches grades verbatim and
// computes the hindsight layer (annotate.go).
func Replay(rec engine.HandRecord, ann HandAnnotations) *ReplayModel {
	m := &ReplayModel{
		Record:   rec,
		HeroSeat: heroSeatOf(rec.Seats),
		Button:   engine.NoSeat,
		Holes:    map[engine.Seat][2]engine.Card{},
	}
	m.Summary.GradeCounts = map[string]int{}
	m.Summary.KeyDecision = -1
	m.Outcome.WinnerSeat = engine.NoSeat

	var (
		stacks    [engine.MaxSeats]engine.Chips
		start     [engine.MaxSeats]engine.Chips
		committed [engine.MaxSeats]engine.Chips
		pot       engine.Chips
		board     []engine.Card
		folded    engine.SeatSet
		street    engine.Street
		settled   bool // the final street's frame has been closed
	)
	for _, si := range rec.Seats {
		if si.Seat.Valid() {
			stacks[si.Seat] = si.StartingStack
			start[si.Seat] = si.StartingStack
		}
	}

	closeStreet := func() {
		m.Streets = append(m.Streets, StreetFrame{
			Street:   street,
			Board:    append([]engine.Card(nil), board...),
			PotAfter: pot,
			Stacks:   stacks,
			Folded:   folded,
		})
	}
	// pay moves chips from a stack into the pot, mirroring the engine's own
	// bookkeeping (street totals include blinds; all-in pays are capped).
	pay := func(s engine.Seat, delta engine.Chips) {
		if delta <= 0 {
			return
		}
		if delta > stacks[s] {
			delta = stacks[s]
		}
		stacks[s] -= delta
		committed[s] += delta
		pot += delta
	}
	// settle closes the final street frame the first time a settlement event
	// (showdown, refund, award) appears, so StreetFrame stacks and pot are
	// the betting truth, untouched by payouts.
	settle := func() {
		if !settled {
			settled = true
			closeStreet()
		}
	}

	for i, e := range rec.Events {
		switch e.Kind {
		case engine.EvPostBlind:
			pay(e.Seat, e.Amount)

		case engine.EvDealHole:
			m.DealtIn = m.DealtIn.Add(e.Seat)
			if len(e.Cards) == 2 {
				m.Holes[e.Seat] = [2]engine.Card{e.Cards[0], e.Cards[1]}
			}
			// Hole cards are dealt first seat left of the button, button
			// last — so the last EvDealHole seat IS the button. The record
			// does not store the button; the deal order does.
			m.Button = e.Seat

		case engine.EvDealBoard:
			closeStreet()
			street = e.Street
			committed = [engine.MaxSeats]engine.Chips{}
			board = append(board, e.Cards...)

		case engine.EvAction:
			if e.Seat == m.HeroSeat {
				d := DecisionFrame{
					EventIdx:  i,
					Street:    street,
					Action:    e.Action,
					Amount:    e.Amount,
					PotBefore: pot,
					Board:     append([]engine.Card(nil), board...),
					Stacks:    stacks,
					Committed: committed,
					Folded:    folded,
				}
				var high engine.Chips
				for _, s := range m.DealtIn.Seats() {
					if committed[s] > high {
						high = committed[s]
					}
				}
				if owe := high - committed[e.Seat]; owe > 0 {
					if owe > stacks[e.Seat] {
						owe = stacks[e.Seat]
					}
					d.ToCall = owe
				}
				if e.Amount > committed[e.Seat] {
					d.Paid = e.Amount - committed[e.Seat]
				}
				if g, ok := ann.Grades[i]; ok {
					// Copy, attach, done. The grade was computed before the
					// next card was dealt and is immutable here by design.
					frozen := g
					d.Frozen = &frozen
				}
				m.Decisions = append(m.Decisions, d)
			}
			switch e.Action {
			case engine.ActionFold:
				folded = folded.Add(e.Seat)
			case engine.ActionCheck:
				// No chips move.
			default:
				pay(e.Seat, e.Amount-committed[e.Seat])
			}

		case engine.EvShowdown:
			settle()
			m.Outcome.Showdown = true

		case engine.EvRefundUncalled:
			settle()
			stacks[e.Seat] += e.Amount
			pot -= e.Amount

		case engine.EvAwardPot:
			settle()
			stacks[e.Seat] += e.Amount
			pot -= e.Amount
			m.Outcome.Awards = append(m.Outcome.Awards,
				Award{Seat: e.Seat, Amount: e.Amount, Pot: e.PotIndex})
		}
	}
	if !settled {
		// Defensive: a truncated record (crash mid-hand) still closes its
		// last street so the frames stay coherent.
		closeStreet()
	}

	m.Outcome.Board = append([]engine.Card(nil), board...)
	m.Outcome.Stacks = stacks
	m.Outcome.Folded = folded
	m.finishOutcome()
	m.summarize(start)
	annotate(m)
	return m
}

// finishOutcome resolves the main-pot winner and, at showdown, which five of
// their seven cards play (eval.Best5) — highlighted board cards included, so
// the review keeps teaching "best five of seven".
func (m *ReplayModel) finishOutcome() {
	for _, a := range m.Outcome.Awards {
		if a.Pot == 0 {
			m.Outcome.WinnerSeat = a.Seat
			break
		}
	}
	if !m.Outcome.Showdown || !m.Outcome.WinnerSeat.Valid() || len(m.Outcome.Board) < 3 {
		return
	}
	hole, ok := m.Holes[m.Outcome.WinnerSeat]
	if !ok {
		return
	}
	rank, five := eval.Best5(hole, m.Outcome.Board)
	m.Outcome.WinningFive = five[:]
	m.Outcome.WinnerDesc = rank.String()
}

// summarize fills the two scoreboards. The EV ledger only ever reads the
// frozen EVLossBB values — the outcome column is computed separately and
// never feeds back into it.
func (m *ReplayModel) summarize(start [engine.MaxSeats]engine.Chips) {
	if m.HeroSeat.Valid() {
		m.Summary.HeroNet = m.Outcome.Stacks[m.HeroSeat] - start[m.HeroSeat]
	}
	if m.Record.Blinds.Big > 0 {
		m.Summary.HeroNetBB = float64(m.Summary.HeroNet) / float64(m.Record.Blinds.Big)
	}
	for i, d := range m.Decisions {
		if d.Frozen == nil {
			continue
		}
		m.Summary.Graded++
		m.Summary.GradeCounts[d.Frozen.Band]++
		m.Summary.EVLossBB += d.Frozen.EVLossBB
		if m.Summary.KeyDecision < 0 ||
			d.Frozen.EVLossBB > m.Decisions[m.Summary.KeyDecision].Frozen.EVLossBB {
			m.Summary.KeyDecision = i
		}
	}
}

// heroSeatOf finds the human: the first seat whose personality key is empty
// (design-learning.md §6 — "" marks the human player). Falls back to seat 0,
// the table screen's fixed hero seat.
func heroSeatOf(seats []engine.SeatInfo) engine.Seat {
	for _, si := range seats {
		if si.Personality == "" && si.Seat.Valid() {
			return si.Seat
		}
	}
	return 0
}
