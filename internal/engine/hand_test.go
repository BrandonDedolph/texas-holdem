package engine

import (
	"errors"
	"reflect"
	"testing"
)

// --- Shared test scaffolding -------------------------------------------------
//
// The engine cannot import internal/eval (built in parallel; the dependency
// edge is eval → engine), so tests inject stub Evaluators. TODO(wire-eval):
// nothing here changes when the real evaluator lands — only the app layer's
// injection does.

// evalFunc adapts a function to the Evaluator interface.
type evalFunc func(hole [2]Card, board []Card) uint32

func (f evalFunc) EvalHoldem(hole [2]Card, board []Card) uint32 { return f(hole, board) }

// holeEval ranks a hand by its first hole card through a fixed map — enough
// to script exactly who wins a showdown. Unmapped cards rank 0.
func holeEval(ranks map[Card]uint32) Evaluator {
	return evalFunc(func(hole [2]Card, _ []Card) uint32 { return ranks[hole[0]] })
}

// hashEval is an arbitrary but fully deterministic evaluator (FNV-1a over
// hole + board) for fuzzing and replay tests, where any total order will do.
var hashEval Evaluator = evalFunc(func(hole [2]Card, board []Card) uint32 {
	h := uint32(2166136261)
	mix := func(c Card) { h = (h ^ uint32(c)) * 16777619 }
	mix(hole[0])
	mix(hole[1])
	for _, c := range board {
		mix(c)
	}
	return h
})

// assertConserved checks the engine's core invariant after every action:
// Σ stacks + Σ committed == Σ starting stacks (the pot reads zero once paid
// out, at which point the stacks alone must carry the full amount).
func assertConserved(t *testing.T, h *Hand) {
	t.Helper()
	var have, want Chips
	for s := Seat(0); s < MaxSeats; s++ {
		have += h.Stack(s)
	}
	have += h.PotTotal()
	for _, c := range h.startStacks {
		want += c
	}
	if have != want {
		t.Fatalf("chip conservation broken: stacks+pot = %d, want %d", have, want)
	}
}

// mustApply applies an action that must be legal, then re-checks chip
// conservation — wrapping every scenario test in the invariant.
func mustApply(t *testing.T, h *Hand, a Action) {
	t.Helper()
	if err := h.Apply(a); err != nil {
		t.Fatalf("Apply(%s by seat %d) failed: %v", a.Type(), a.Seat(), err)
	}
	assertConserved(t, h)
}

func mustSetup(t *testing.T, s HandSetup) *Hand {
	t.Helper()
	h, err := NewHandFromSetup(s)
	if err != nil {
		t.Fatalf("NewHandFromSetup: %v", err)
	}
	assertConserved(t, h)
	return h
}

// cfg12 is the standard test table: blinds 1/2.
func cfg12() TableConfig { return TableConfig{SmallBlind: 1, BigBlind: 2, Seats: MaxSeats} }

// foldOut folds every acting seat until the hand completes (a walk).
func foldOut(t *testing.T, h *Hand) {
	t.Helper()
	for h.Phase() == PhaseBetting {
		mustApply(t, h, Fold{h.CurrentSeat()})
	}
}

func countEvents(h *Hand, kind EventKind) int {
	n := 0
	for _, e := range h.Events() {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func assertNet(t *testing.T, h *Hand, want map[Seat]Chips) {
	t.Helper()
	res, ok := h.Result()
	if !ok {
		t.Fatal("Result not available on a completed hand")
	}
	var sum Chips
	for s := Seat(0); s < MaxSeats; s++ {
		sum += res.Net[s]
		if res.Net[s] != want[s] {
			t.Errorf("Net[%d] = %d, want %d", s, res.Net[s], want[s])
		}
	}
	if sum != 0 {
		t.Errorf("Net sums to %d, want 0", sum)
	}
}

// --- Scenarios ---------------------------------------------------------------

func TestWalkEveryoneFoldsToBB(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100, 3: 100},
		Seed:   1,
	})
	// Preflop order 4-handed: UTG (3), BTN (0), SB (1); BB never acts — the
	// third fold leaves one live player and the hand ends instantly.
	foldOut(t, h)

	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete", h.Phase())
	}
	if got := countEvents(h, EvDealBoard); got != 0 {
		t.Fatalf("walk dealt %d board events; remaining streets must never be dealt", got)
	}
	if got := countEvents(h, EvShowdown); got != 0 {
		t.Fatal("walk produced showdown events; the winner needn't show")
	}
	res, _ := h.Result()
	if res.Showdown || res.WentTo != Preflop {
		t.Fatalf("result = %+v, want no showdown, went to preflop", res)
	}
	// The BB wins the small blind; everyone else is flat except the SB.
	assertNet(t, h, map[Seat]Chips{1: -1, 2: 1})
}

func TestCheckedDownToShowdown(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 200, 1: 200},
		Holes:  map[Seat][2]Card{0: Holes("2c 2d"), 1: Holes("Ac Ad")},
		Seed:   2,
		Eval:   holeEval(map[Card]uint32{MustCard("2c"): 1, MustCard("Ac"): 9}),
	})

	// Heads-up: the button posts the small blind and acts first preflop.
	if h.CurrentSeat() != 0 {
		t.Fatalf("preflop first to act = %d, want button 0", h.CurrentSeat())
	}
	mustApply(t, h, Call{0})
	mustApply(t, h, Check{1}) // BB option closes the round
	for st := Flop; st <= River; st++ {
		if h.Street() != st {
			t.Fatalf("street = %v, want %v", h.Street(), st)
		}
		// Postflop the button acts last.
		if h.CurrentSeat() != 1 {
			t.Fatalf("%v first to act = %d, want 1", st, h.CurrentSeat())
		}
		mustApply(t, h, Check{1})
		mustApply(t, h, Check{0})
	}

	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete", h.Phase())
	}
	if len(h.Board()) != 5 {
		t.Fatalf("board has %d cards, want 5", len(h.Board()))
	}
	res, _ := h.Result()
	if !res.Showdown || res.WentTo != River {
		t.Fatalf("result = %+v, want showdown at river", res)
	}
	assertNet(t, h, map[Seat]Chips{0: -2, 1: 2})

	// River checked through: first live seat left of the button reveals first.
	var reveals []Seat
	for _, e := range h.Events() {
		if e.Kind == EvShowdown {
			reveals = append(reveals, e.Seat)
		}
	}
	if !reflect.DeepEqual(reveals, []Seat{1, 0}) {
		t.Fatalf("reveal order = %v, want [1 0]", reveals)
	}
}

func TestAllInRunOutDealsBoardInOneStep(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 150, 1: 150},
		Holes:  map[Seat][2]Card{0: Holes("Kc Kd"), 1: Holes("Ac Ad")},
		Seed:   3,
		Eval:   holeEval(map[Card]uint32{MustCard("Kc"): 5, MustCard("Ac"): 9}),
	})
	mustApply(t, h, Raise{0, 150}) // open-shove
	mustApply(t, h, Call{1})       // all-in call

	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete after run-out", h.Phase())
	}
	if len(h.Board()) != 5 {
		t.Fatalf("board has %d cards, want 5", len(h.Board()))
	}
	// The run-out still emits one board event per street for the TUI.
	var streets []Street
	for _, e := range h.Events() {
		if e.Kind == EvDealBoard {
			streets = append(streets, e.Street)
		}
	}
	if !reflect.DeepEqual(streets, []Street{Flop, Turn, River}) {
		t.Fatalf("board events = %v, want [flop turn river]", streets)
	}
	res, _ := h.Result()
	if !res.Showdown {
		t.Fatal("run-out must end in a showdown")
	}
	assertNet(t, h, map[Seat]Chips{0: -150, 1: 150})
	if h.Stack(1) != 300 || h.Stack(0) != 0 {
		t.Fatalf("stacks = %d/%d, want 0/300", h.Stack(0), h.Stack(1))
	}
}

func TestFoldEndsHandMidStreetImmediately(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   4,
		Eval:   hashEval,
	})
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{1})
	mustApply(t, h, Check{2})
	// Flop: SB bets, both fold — the hand must end mid-street with no turn.
	mustApply(t, h, Bet{1, 10})
	mustApply(t, h, Fold{2})
	mustApply(t, h, Fold{0})

	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete", h.Phase())
	}
	if got := countEvents(h, EvDealBoard); got != 1 {
		t.Fatalf("dealt %d board events, want 1 (flop only)", got)
	}
	res, _ := h.Result()
	if res.WentTo != Flop || res.Showdown {
		t.Fatalf("result = %+v, want uncontested on the flop", res)
	}
	// The uncalled 10 goes back to the bettor: SB wins 2+2 from the others.
	assertNet(t, h, map[Seat]Chips{0: -2, 1: 4, 2: -2})
	if got := countEvents(h, EvRefundUncalled); got != 1 {
		t.Fatalf("refund events = %d, want 1", got)
	}
}

func TestReplayDeterminism(t *testing.T) {
	setup := HandSetup{
		Config: cfg12(),
		Button: 2,
		Stacks: map[Seat]Chips{0: 80, 2: 120, 4: 60, 5: 200},
		Seed:   77,
		Eval:   hashEval,
	}
	// A scripted line reaching a multiway showdown with a short all-in.
	// Button 2, SB 4, BB 5, UTG 0.
	play := func() *Hand {
		h := mustSetup(t, setup)
		mustApply(t, h, Raise{0, 6}) // UTG opens
		mustApply(t, h, Call{2})
		mustApply(t, h, Call{4})
		mustApply(t, h, Call{5})
		mustApply(t, h, Check{4}) // flop, first live seat left of the button
		mustApply(t, h, Check{5})
		mustApply(t, h, Bet{0, 12})
		mustApply(t, h, Call{2})
		mustApply(t, h, Raise{4, 54}) // all-in for a full raise
		mustApply(t, h, Fold{5})
		mustApply(t, h, Call{0})
		mustApply(t, h, Call{2})
		mustApply(t, h, Check{0}) // turn
		mustApply(t, h, Check{2})
		mustApply(t, h, Bet{0, 20}) // river: all-in
		mustApply(t, h, Call{2})
		return h
	}
	a, b := play(), play()
	if a.Phase() != PhaseComplete {
		t.Fatalf("scripted line did not complete: %v on %v", a.Phase(), a.Street())
	}
	if !reflect.DeepEqual(a.Events(), b.Events()) {
		t.Fatal("same seed + same action list produced different event logs")
	}
}

func TestScriptedCardsLandOnTheRightSeats(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 200, 1: 200, 2: 200},
		Holes:  map[Seat][2]Card{2: Holes("9c 9d")},
		Board:  MustCards("Ah 9s 4s"),
		Seed:   42,
		Eval:   hashEval,
	})
	hole, ok := h.HoleCards(2)
	if !ok || hole != Holes("9c 9d") {
		t.Fatalf("seat 2 holes = %v %v, want scripted 9c 9d", hole, ok)
	}
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{1})
	mustApply(t, h, Check{2})
	if got := CardsString(h.Board()); got != "Ah 9s 4s" {
		t.Fatalf("flop = %q, want scripted \"Ah 9s 4s\"", got)
	}
	// Every dealt card must be unique across holes and board.
	var seen CardSet
	for s := Seat(0); s < 3; s++ {
		hc, _ := h.HoleCards(s)
		for _, c := range hc[:] {
			if seen.Has(c) {
				t.Fatalf("card %s dealt twice", c.Code())
			}
			seen = seen.Add(c)
		}
	}
	for _, c := range h.Board() {
		if seen.Has(c) {
			t.Fatalf("board card %s duplicates a hole card", c.Code())
		}
		seen = seen.Add(c)
	}
}

func TestHandSetupValidation(t *testing.T) {
	base := func() HandSetup {
		return HandSetup{
			Config: cfg12(),
			Button: 0,
			Stacks: map[Seat]Chips{0: 100, 1: 100},
			Seed:   1,
		}
	}
	tests := []struct {
		name   string
		mutate func(*HandSetup)
		want   error
	}{
		{"one player", func(s *HandSetup) { delete(s.Stacks, 1) }, ErrNotEnoughPlayers},
		{"zero stack", func(s *HandSetup) { s.Stacks[1] = 0 }, ErrInvalidBuyIn},
		{"button absent", func(s *HandSetup) { s.Button = 3 }, ErrInvalidSeat},
		{"holes for absent seat", func(s *HandSetup) {
			s.Holes = map[Seat][2]Card{4: Holes("As Ks")}
		}, ErrInvalidSeat},
		{"duplicate across holes", func(s *HandSetup) {
			s.Holes = map[Seat][2]Card{0: Holes("As Ks"), 1: Holes("As Qs")}
		}, ErrDuplicateCard},
		{"duplicate hole/board", func(s *HandSetup) {
			s.Holes = map[Seat][2]Card{0: Holes("As Ks")}
			s.Board = MustCards("As 2c 3c")
		}, ErrDuplicateCard},
		{"bad seat", func(s *HandSetup) { s.Stacks[Seat(9)] = 100 }, ErrInvalidSeat},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.mutate(&s)
			if _, err := NewHandFromSetup(s); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}

	s := base()
	s.Config.BigBlind = 0
	if _, err := NewHandFromSetup(s); err == nil {
		t.Fatal("zero big blind accepted")
	}
	s = base()
	s.Board = MustCards("2c 3c 4c 5c 6c 7c")
	if _, err := NewHandFromSetup(s); err == nil {
		t.Fatal("six board cards accepted")
	}
}

func TestValidateAndApplyErrors(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   5,
	})
	if h.CurrentSeat() != 0 {
		t.Fatalf("first to act = %d, want 0", h.CurrentSeat())
	}
	if err := h.Apply(Fold{1}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("out-of-turn fold: %v, want ErrNotYourTurn", err)
	}
	if err := h.Apply(Check{0}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("check facing the blind: %v, want ErrIllegalAction", err)
	}
	if err := h.Apply(Bet{0, 10}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("bet with a live bet outstanding: %v, want ErrIllegalAction", err)
	}
	if err := h.Apply(Raise{0, 3}); !errors.Is(err, ErrBetSizing) {
		t.Fatalf("raise below minimum: %v, want ErrBetSizing", err)
	}
	if err := h.Apply(Raise{0, 101}); !errors.Is(err, ErrBetSizing) {
		t.Fatalf("raise beyond stack: %v, want ErrBetSizing", err)
	}
	// Validate must not mutate: the same raise still works afterwards.
	if err := h.Validate(Raise{0, 4}); err != nil {
		t.Fatalf("Validate(min-raise): %v", err)
	}
	mustApply(t, h, Raise{0, 4})

	foldOut(t, h)
	if err := h.Apply(Fold{2}); !errors.Is(err, ErrHandComplete) {
		t.Fatalf("action on a complete hand: %v, want ErrHandComplete", err)
	}
	if got := h.LegalActions(); len(got) != 0 {
		t.Fatalf("LegalActions on a complete hand = %v, want empty", got)
	}
	if h.CurrentSeat() != NoSeat {
		t.Fatalf("CurrentSeat = %d, want NoSeat", h.CurrentSeat())
	}
}

func TestPlayerViewNeverLeaksCards(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   6,
		Eval:   hashEval,
	})
	assertNoCheat := func() {
		t.Helper()
		for _, seat := range h.DealtIn().Seats() {
			v := h.View(seat)
			if v == nil {
				t.Fatalf("View(%d) = nil for a dealt-in seat", seat)
			}
			allowed := NewCardSet(v.Hole[0], v.Hole[1])
			for _, c := range v.Board {
				allowed = allowed.Add(c)
			}
			check := func(c Card) {
				if !allowed.Has(c) {
					t.Fatalf("View(%d) leaked %s (outside Hole ∪ Board)", seat, c.Code())
				}
			}
			for _, c := range v.Board {
				check(c)
			}
			check(v.Hole[0])
			check(v.Hole[1])
			for _, e := range v.History {
				for _, c := range e.Cards {
					check(c)
				}
			}
		}
	}

	// Check at every decision point, and again after the showdown.
	script := []Action{Call{0}, Call{1}, Check{2}, Bet{1, 6}, Call{2}, Call{0},
		Check{1}, Check{2}, Check{0}, Check{1}, Bet{2, 20}, Call{0}, Fold{1}}
	assertNoCheat()
	for _, a := range script {
		mustApply(t, h, a)
		assertNoCheat()
	}
	if h.Phase() != PhaseComplete {
		t.Fatalf("script did not finish the hand: %v %v", h.Phase(), h.Street())
	}
}

func TestPlayerViewFields(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   7,
	})
	mustApply(t, h, Raise{0, 6})

	v := h.View(1) // SB facing a raise, to act
	if v.Seat != 1 || v.Button != 0 || v.Street != Preflop {
		t.Fatalf("view basics wrong: %+v", v)
	}
	if v.ToCall != 5 {
		t.Fatalf("ToCall = %d, want 5", v.ToCall)
	}
	if v.Pot != 9 {
		t.Fatalf("Pot = %d, want 9 (1+2+6)", v.Pot)
	}
	if v.InHand != NewSeatSet(0, 1, 2) {
		t.Fatalf("InHand = %v", v.InHand)
	}
	if v.Blinds != (Blinds{Small: 1, Big: 2}) {
		t.Fatalf("Blinds = %+v", v.Blinds)
	}
	if len(v.Legal) == 0 {
		t.Fatal("acting seat's view has no legal actions")
	}
	if got := v.Legal.CallAmount(); got != 5 {
		t.Fatalf("Legal.CallAmount = %d, want 5", got)
	}
	if _, ok := v.Legal.Find(ActionRaise); !ok {
		t.Fatal("SB facing a full raise must be able to reraise")
	}
	if other := h.View(2); other.Legal != nil {
		t.Fatalf("non-acting seat's view has legal actions: %v", other.Legal)
	}
	if h.View(4) != nil {
		t.Fatal("View of a seat not dealt in must be nil")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100},
		Holes:  map[Seat][2]Card{0: Holes("Kc Kd"), 1: Holes("Ac Ad")},
		Seed:   8,
		Eval:   holeEval(map[Card]uint32{MustCard("Kc"): 5, MustCard("Ac"): 9}),
	})
	mustApply(t, h, Call{0})

	c := h.Clone()
	// Run the clone to completion via an all-in; the original must not move.
	mustApply(t, c, Raise{1, 100})
	mustApply(t, c, Call{0})
	if c.Phase() != PhaseComplete {
		t.Fatalf("clone phase = %v, want complete", c.Phase())
	}
	if h.Phase() != PhaseBetting || h.Street() != Preflop || h.CurrentSeat() != 1 {
		t.Fatal("advancing the clone mutated the original")
	}
	if len(h.Board()) != 0 {
		t.Fatal("clone's run-out dealt cards onto the original's board")
	}
	if got := countEvents(h, EvDealBoard); got != 0 {
		t.Fatal("clone's board events leaked into the original's log")
	}

	// The clone's deck was cloned too: finishing the original now yields the
	// exact same board, because both decks continue from the same card.
	mustApply(t, h, Raise{1, 100})
	mustApply(t, h, Call{0})
	if !reflect.DeepEqual(h.Board(), c.Board()) {
		t.Fatalf("original board %v differs from clone board %v — deck was shared or lost",
			CardsString(h.Board()), CardsString(c.Board()))
	}
	if !reflect.DeepEqual(h.Events(), c.Events()) {
		t.Fatal("identical lines on original and clone produced different logs")
	}
}

func TestShortBlindAllInIsRefundedCorrectly(t *testing.T) {
	// The BB can only post 1 of the 2; the SB (button, heads-up) completes.
	// The BB's short post caps what it can win; the SB's extra chip returns.
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 1},
		Holes:  map[Seat][2]Card{0: Holes("Kc Kd"), 1: Holes("Ac Ad")},
		Seed:   9,
		Eval:   holeEval(map[Card]uint32{MustCard("Kc"): 5, MustCard("Ac"): 9}),
	})
	// BB is all-in from the post; the SB owes the difference to the full BB
	// price, and the excess over the BB's actual chips comes back.
	mustApply(t, h, Call{0})
	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete (run-out)", h.Phase())
	}
	assertNet(t, h, map[Seat]Chips{0: -1, 1: 1})
}
