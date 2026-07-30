package engine

import "fmt"

// Street is a betting street.
type Street uint8

// Streets, in deal order.
const (
	Preflop Street = iota
	Flop
	Turn
	River
)

var streetNames = [...]string{"preflop", "flop", "turn", "river"}

// String is the lowercase street name.
func (s Street) String() string {
	if int(s) >= len(streetNames) {
		return "?"
	}
	return streetNames[s]
}

// Phase is a hand's lifecycle phase.
type Phase uint8

// Phases. PhaseShowdown is transient — Apply resolves the showdown
// synchronously, so callers observe PhaseBetting during play and
// PhaseComplete afterwards; Result().Showdown says whether one happened.
const (
	PhaseBetting  Phase = iota // a betting round is open
	PhaseShowdown              // river betting closed with 2+ live players
	PhaseComplete              // pots awarded; result available
)

// Evaluator compares showdown hands. Higher return values win; equal values
// split. The value is the packed eval.HandRank — the engine treats it as an
// opaque totally-ordered strength and records it in showdown events.
//
// The engine cannot import internal/eval (the dependency edge is
// eval → engine), so the app layer injects an implementation.
// TODO(wire-eval): wire internal/eval's EvalHoldem here via a tiny adapter
// (e.g. Table.Eval = evalAdapter{} where EvalHoldem returns uint32(rank)).
type Evaluator interface {
	EvalHoldem(hole [2]Card, board []Card) uint32
}

// HandResult summarizes a completed hand.
type HandResult struct {
	Awards []PotAward // per pot layer: winners, shares, winning rank
	// Net is won − invested per seat; it always sums to zero.
	Net      [MaxSeats]Chips
	WentTo   Street // how far the hand got
	Showdown bool
}

// Hand is one deal — the analogue of euchre's Round. It is created by
// Table.StartHand or NewHandFromSetup with blinds already posted and hole
// cards dealt, and is mutated only through Apply.
type Hand struct {
	blinds  Blinds
	button  Seat
	dealtIn SeatSet
	eval    Evaluator
	deck    CardSource

	stacks      [MaxSeats]Chips // behind, mid-hand
	startStacks [MaxSeats]Chips
	holes       [MaxSeats][2]Card
	total       [MaxSeats]Chips // contributed to the hand, across streets

	phase   Phase
	street  Street
	board   []Card
	folded  SeatSet
	allIn   SeatSet
	bet     bettingState
	current Seat // seat to act; NoSeat when no decision pends

	// riverAggressor is the last seat to bet or raise on the river, for the
	// showdown reveal order. NoSeat if the river checked through (or was
	// never bet — run-outs).
	riverAggressor Seat

	events []Event
	result *HandResult
}

// newHand posts blinds, deals hole cards, and opens preflop betting.
// stacks[s] > 0 for every s in dealtIn is a precondition (validated by the
// callers, Table.StartHand and NewHandFromSetup).
func newHand(blinds Blinds, button Seat, dealtIn SeatSet, stacks [MaxSeats]Chips, deck CardSource, eval Evaluator) *Hand {
	h := &Hand{
		blinds:         blinds,
		button:         button,
		dealtIn:        dealtIn,
		eval:           eval,
		deck:           deck,
		stacks:         stacks,
		startStacks:    stacks,
		current:        NoSeat,
		riverAggressor: NoSeat,
	}

	// Heads-up the button posts the small blind; otherwise the blinds are
	// the two seats after the button.
	sb := button
	if dealtIn.Count() > 2 {
		sb = dealtIn.Next(button)
	}
	bb := dealtIn.Next(sb)
	h.postBlind(sb, blinds.Small)
	h.postBlind(bb, blinds.Big)

	// Deal hole cards two at a time per seat, first seat left of the button
	// first, button last. No burn cards — burns defeat marked cards, which
	// don't exist digitally, and they'd force lesson scripts to insert junk.
	for s := dealtIn.Next(button); ; s = dealtIn.Next(s) {
		h.holes[s] = [2]Card{deck.Draw(), deck.Draw()}
		h.append(Event{Kind: EvDealHole, Seat: s, Cards: []Card{h.holes[s][0], h.holes[s][1]}})
		if s == button {
			break
		}
	}

	// Open preflop. Posting a blind was not acting — ActedAtBet stays -1 for
	// everyone, which is what gives the BB its option later.
	h.bet.reset(true, blinds.Big)
	h.bet.Committed = [MaxSeats]Chips{}
	for _, s := range dealtIn.Seats() {
		h.bet.Committed[s] = h.total[s]
	}
	h.openRound(bb)
	return h
}

// postBlind moves a compulsory blind into the pot. Blinds are engine events,
// never Actions — a short stack posts all-in for less.
func (h *Hand) postBlind(seat Seat, amount Chips) {
	if amount > h.stacks[seat] {
		amount = h.stacks[seat]
	}
	h.pay(seat, amount)
	h.append(Event{Kind: EvPostBlind, Seat: seat, Amount: amount, PotAfter: h.PotTotal()})
}

// pay moves chips from a seat's stack into the pot, tracking street and hand
// totals and flagging all-ins.
func (h *Hand) pay(seat Seat, amount Chips) {
	h.stacks[seat] -= amount
	h.bet.Committed[seat] += amount
	h.total[seat] += amount
	if h.stacks[seat] == 0 {
		h.allIn = h.allIn.Add(seat)
		h.bet.ToAct = h.bet.ToAct.Remove(seat)
	}
}

// actors is the set of live seats with chips behind — the seats capable of
// making a decision.
func (h *Hand) actors() SeatSet { return (h.dealtIn &^ h.folded) &^ h.allIn }

// openRound seeds ToAct for a street and picks the first seat to act:
// clockwise after `after` (the big blind preflop, the button postflop).
//
// The round is skipped (ToAct left empty) when fewer than two seats can act,
// with one exception: preflop a lone actor still owing chips against the
// blind price must act on it. A lone actor with all bets matched has nobody
// left to respond, so there is no decision to make.
func (h *Hand) openRound(after Seat) {
	acts := h.actors()
	switch {
	case acts.Count() >= 2:
		h.bet.ToAct = acts
	case acts.Count() == 1:
		s := acts.First()
		if h.bet.Committed[s] < h.bet.CurrentBet {
			h.bet.ToAct = acts
		}
	}
	if h.bet.ToAct.Empty() {
		h.current = NoSeat
		h.closeRound()
		return
	}
	h.current = h.bet.ToAct.Next(after)
}

// LegalActions returns the acting seat's options. The slice is empty iff the
// hand is not awaiting a decision. It is the single source of truth for
// legality: the TUI renders buttons from it, the AI and coach choose within
// it, and Apply re-validates against it (defense in depth).
func (h *Hand) LegalActions() ActionOptions {
	if h.phase != PhaseBetting || h.current == NoSeat {
		return nil
	}
	return h.optionsFor(h.current)
}

func (h *Hand) optionsFor(seat Seat) ActionOptions {
	canRespond := h.actors().Remove(seat).Count() > 0
	return h.bet.options(seat, h.stacks[seat], h.blinds.Big, canRespond)
}

// Validate reports whether a would be legal right now, without applying it.
// The coach uses this to grade hypotheticals cheaply.
func (h *Hand) Validate(a Action) error {
	if h.phase != PhaseBetting {
		return ErrHandComplete
	}
	if a.Seat() != h.current {
		return ErrNotYourTurn
	}
	opt, ok := h.optionsFor(a.Seat()).Find(a.Type())
	if !ok {
		return fmt.Errorf("%w: %s", ErrIllegalAction, a.Type())
	}
	amount, sized := actionAmount(a)
	if sized && (amount < opt.Min || amount > opt.Max) {
		return fmt.Errorf("%w: %s %d outside [%d, %d]",
			ErrBetSizing, a.Type(), amount, opt.Min, opt.Max)
	}
	return nil
}

// actionAmount extracts the caller-chosen size, if the action carries one.
func actionAmount(a Action) (Chips, bool) {
	switch v := a.(type) {
	case Bet:
		return v.Amount, true
	case Raise:
		return v.To, true
	case *Bet:
		return v.Amount, true
	case *Raise:
		return v.To, true
	}
	return 0, false
}

// Apply validates and applies an action, advancing streets and phases as
// needed (including the all-in run-out). It is the only mutator during a
// hand.
func (h *Hand) Apply(a Action) error {
	if err := h.Validate(a); err != nil {
		return err
	}
	seat := a.Seat()

	switch a.Type() {
	case ActionFold:
		h.folded = h.folded.Add(seat)
		h.bet.ToAct = h.bet.ToAct.Remove(seat)
		h.appendAction(seat, ActionFold, h.bet.Committed[seat])

	case ActionCheck:
		h.bet.recordCheckOrCall(seat)
		h.appendAction(seat, ActionCheck, h.bet.Committed[seat])

	case ActionCall:
		owe := h.bet.CurrentBet - h.bet.Committed[seat]
		if owe > h.stacks[seat] {
			owe = h.stacks[seat] // all-in call for less
		}
		h.pay(seat, owe)
		h.bet.recordCheckOrCall(seat)
		h.appendAction(seat, ActionCall, h.bet.Committed[seat])

	case ActionBet:
		amount, _ := actionAmount(a)
		full := amount >= h.blinds.Big // else: all-in for less than a min bet
		h.pay(seat, amount-h.bet.Committed[seat])
		h.bet.recordAggression(seat, amount, full, h.actors())
		h.noteAggression(seat)
		h.appendAction(seat, ActionBet, amount)

	case ActionRaise:
		to, _ := actionAmount(a)
		// A raise below CurrentBet + LastFullRaiseSz is only reachable as an
		// all-in (LegalActions pins Min = Max = all-in then) — the
		// incomplete raise that moves the price without reopening action.
		full := to >= h.bet.CurrentBet+h.bet.LastFullRaiseSz
		h.pay(seat, to-h.bet.Committed[seat])
		h.bet.recordAggression(seat, to, full, h.actors())
		h.noteAggression(seat)
		h.appendAction(seat, ActionRaise, to)
	}

	h.advance(seat)
	return nil
}

func (h *Hand) noteAggression(seat Seat) {
	if h.street == River {
		h.riverAggressor = seat
	}
}

func (h *Hand) appendAction(seat Seat, t ActionType, streetTotal Chips) {
	h.append(Event{
		Kind: EvAction, Street: h.street, Seat: seat,
		Action: t, Amount: streetTotal, PotAfter: h.PotTotal(),
	})
}

// advance moves the hand forward after an applied action: next seat, or
// close the round, or end the hand.
func (h *Hand) advance(from Seat) {
	// A fold that leaves one live player ends the hand immediately —
	// remaining streets are NEVER dealt. Dealing them would leak information
	// the real game doesn't reveal; "what would have come" is a review
	// feature, computed from the recorded deck, not engine behavior.
	if h.live().Count() == 1 {
		h.finishUncontested()
		return
	}
	if !h.bet.ToAct.Empty() {
		h.current = h.bet.ToAct.Next(from)
		return
	}
	h.closeRound()
}

// closeRound fires when ToAct is empty: run out the board, go to showdown,
// or open the next street.
func (h *Hand) closeRound() {
	h.current = NoSeat
	if h.street == River {
		h.showdown()
		return
	}
	if h.actors().Count() <= 1 {
		// All-in run-out: everyone live is all-in, or one player has chips
		// and every bet is matched. Deal the remaining board in one step,
		// emitting per-street events for the TUI to animate.
		for h.street < River {
			h.street++
			h.dealBoard()
		}
		h.showdown()
		return
	}
	h.street++
	h.dealBoard()
	h.bet.reset(false, h.blinds.Big)
	h.openRound(h.button) // first live seat left of the button opens
}

// dealBoard deals the current street's board cards: three for the flop, one
// for the turn and river.
func (h *Hand) dealBoard() {
	n := 1
	if h.street == Flop {
		n = 3
	}
	cards := make([]Card, n)
	for i := range cards {
		cards[i] = h.deck.Draw()
	}
	h.board = append(h.board, cards...)
	h.append(Event{Kind: EvDealBoard, Street: h.street, Cards: cards})
}

// finishUncontested awards everything to the last live seat without a
// showdown; the winner needn't show.
func (h *Hand) finishUncontested() {
	h.current = NoSeat
	pots, refund := BuildPots(h.total, h.folded, h.live())
	h.applyRefunds(refund)
	awards := make([]PotAward, 0, len(pots))
	for i := len(pots) - 1; i >= 0; i-- {
		awards = append(awards, h.payAward(awardPot(i, pots[i], pots[i].Eligible, 0, h.button)))
	}
	h.complete(awards, false)
}

// showdown evaluates every live hand and awards the pot layers, last side
// pot first, main pot last.
func (h *Hand) showdown() {
	h.phase = PhaseShowdown
	if h.eval == nil {
		// Reaching showdown without an evaluator is a wiring error, not a
		// runtime condition; there is no sensible way to pick a winner.
		// TODO(wire-eval): the app layer injects the eval-backed Evaluator.
		panic("engine: showdown reached with nil Evaluator")
	}

	var ranks [MaxSeats]uint32
	for _, s := range h.live().Seats() {
		ranks[s] = h.eval.EvalHoldem(h.holes[s], h.board)
	}

	// Reveal order: last aggressor on the river shows first, then clockwise;
	// if the river checked through (or was run out), the first live seat
	// left of the button shows first. Losers are recorded as
	// mucked-but-known — the review reveals everything anyway.
	first := h.riverAggressor
	if first == NoSeat {
		first = h.live().Next(h.button)
	}
	for s, n := first, h.live().Count(); n > 0; s, n = h.live().Next(s), n-1 {
		h.append(Event{
			Kind: EvShowdown, Street: h.street, Seat: s,
			Cards: []Card{h.holes[s][0], h.holes[s][1]}, Rank: ranks[s],
		})
	}

	pots, refund := BuildPots(h.total, h.folded, h.live())
	h.applyRefunds(refund)
	awards := make([]PotAward, 0, len(pots))
	for i := len(pots) - 1; i >= 0; i-- {
		var winners SeatSet
		var best uint32
		for _, s := range pots[i].Eligible.Seats() {
			switch {
			case winners.Empty() || ranks[s] > best:
				winners, best = NewSeatSet(s), ranks[s]
			case ranks[s] == best:
				winners = winners.Add(s)
			}
		}
		awards = append(awards, h.payAward(awardPot(i, pots[i], winners, best, h.button)))
	}
	h.complete(awards, true)
}

func (h *Hand) applyRefunds(refund [MaxSeats]Chips) {
	for s := Seat(0); s < MaxSeats; s++ {
		if refund[s] > 0 {
			h.stacks[s] += refund[s]
			h.total[s] -= refund[s]
			h.append(Event{Kind: EvRefundUncalled, Street: h.street, Seat: s, Amount: refund[s]})
		}
	}
}

// payAward moves an award's shares into the winners' stacks and logs them.
func (h *Hand) payAward(a PotAward) PotAward {
	for i, s := range a.Winners {
		h.stacks[s] += a.Shares[i]
		h.append(Event{
			Kind: EvAwardPot, Street: h.street, Seat: s,
			Amount: a.Shares[i], PotIndex: a.Pot,
		})
	}
	return a
}

func (h *Hand) complete(awards []PotAward, showdown bool) {
	r := &HandResult{Awards: awards, WentTo: h.street, Showdown: showdown}
	for s := Seat(0); s < MaxSeats; s++ {
		r.Net[s] = h.stacks[s] - h.startStacks[s]
	}
	h.result = r
	h.phase = PhaseComplete
}

func (h *Hand) append(e Event) { h.events = append(h.events, e) }

func (h *Hand) live() SeatSet { return h.dealtIn &^ h.folded }

// --- Read-only views -------------------------------------------------------

// Phase returns the hand's lifecycle phase.
func (h *Hand) Phase() Phase { return h.phase }

// Street returns the current street.
func (h *Hand) Street() Street { return h.street }

// Board returns the community cards dealt so far.
func (h *Hand) Board() []Card {
	out := make([]Card, len(h.board))
	copy(out, h.board)
	return out
}

// HoleCards returns a seat's hole cards; ok is false if the seat was not
// dealt in. This accessor sees every seat — it is for the TUI, review, and
// tests. Code acting for a seat must use View instead.
func (h *Hand) HoleCards(seat Seat) ([2]Card, bool) {
	if !h.dealtIn.Has(seat) {
		return [2]Card{}, false
	}
	return h.holes[seat], true
}

// CurrentSeat returns the seat to act, or NoSeat when no decision is pending.
func (h *Hand) CurrentSeat() Seat { return h.current }

// Live returns the seats that have not folded.
func (h *Hand) Live() SeatSet { return h.live() }

// DealtIn returns the seats dealt into this hand.
func (h *Hand) DealtIn() SeatSet { return h.dealtIn }

// AllIn returns the seats that are all-in.
func (h *Hand) AllIn() SeatSet { return h.allIn }

// Stack returns a seat's chips behind, mid-hand.
func (h *Hand) Stack(seat Seat) Chips {
	if !seat.Valid() {
		return 0
	}
	return h.stacks[seat]
}

// Committed returns a seat's chips committed this street.
func (h *Hand) Committed(seat Seat) Chips {
	if !seat.Valid() {
		return 0
	}
	return h.bet.Committed[seat]
}

// PotTotal returns everything committed so far. Once the hand completes the
// pot has been paid out, so it reads zero.
func (h *Hand) PotTotal() Chips {
	if h.phase == PhaseComplete {
		return 0
	}
	var sum Chips
	for _, c := range h.total {
		sum += c
	}
	return sum
}

// Pots returns the derived pot layers, side pots included. Mid-hand, an
// outstanding uncalled excess (a bet nobody has matched yet) is shown as its
// own top layer only the bettor is eligible for — semantically true, since
// only they can get it back.
func (h *Hand) Pots() []Pot {
	pots, refund := BuildPots(h.total, h.folded, h.live())
	for s := Seat(0); s < MaxSeats; s++ {
		if refund[s] > 0 {
			pots = append(pots, Pot{Amount: refund[s], Eligible: NewSeatSet(s)})
		}
	}
	return pots
}

// ToCall returns the chips the seat must add to call, capped at its stack;
// 0 when checking is legal.
func (h *Hand) ToCall(seat Seat) Chips {
	if !seat.Valid() {
		return 0
	}
	owe := h.bet.CurrentBet - h.bet.Committed[seat]
	if owe < 0 {
		owe = 0
	}
	if owe > h.stacks[seat] {
		owe = h.stacks[seat]
	}
	return owe
}

// Button returns the dealer button seat.
func (h *Hand) Button() Seat { return h.button }

// Blinds returns the blinds this hand was dealt at.
func (h *Hand) Blinds() Blinds { return h.blinds }

// Events returns the hand's event log. The caller must not mutate it.
func (h *Hand) Events() []Event { return h.events }

// Result returns the hand's outcome; ok only in PhaseComplete.
func (h *Hand) Result() (*HandResult, bool) {
	if h.phase != PhaseComplete {
		return nil, false
	}
	return h.result, true
}

// Clone deep-copies the hand. Cheap: all state is value types and small
// slices. The coach clones to evaluate hypothetical lines without touching
// the real hand; the AI may clone for lookahead.
//
// The card source is cloned when it implements SourceCloner (Deck and
// ScriptedDeck do), so a clone's hypothetical run-out cannot consume the
// real hand's cards. A custom source without CloneSource is shared.
func (h *Hand) Clone() *Hand {
	cp := *h
	cp.board = make([]Card, len(h.board))
	copy(cp.board, h.board)
	cp.events = make([]Event, len(h.events))
	copy(cp.events, h.events)
	for i, e := range h.events {
		if e.Cards != nil {
			cp.events[i].Cards = make([]Card, len(e.Cards))
			copy(cp.events[i].Cards, e.Cards)
		}
	}
	if h.result != nil {
		r := *h.result
		r.Awards = make([]PotAward, len(h.result.Awards))
		copy(r.Awards, h.result.Awards)
		cp.result = &r
	}
	if c, ok := h.deck.(SourceCloner); ok {
		cp.deck = c.CloneSource()
	}
	return &cp
}

// --- PlayerView -------------------------------------------------------------

// PlayerView is everything a seat may legally know — opponents' hole cards
// are physically absent from the value, which is what makes "bots provably
// cannot cheat" an honest claim. It is the only thing ai.Player.Act ever
// receives; the coach consumes it too. Fields follow design-learning.md §1,
// retyped per DESIGN.md §2.1.
type PlayerView struct {
	Seat      Seat
	Hole      [2]Card
	Board     []Card // 0, 3, 4, or 5 cards
	Street    Street
	Pot       Chips // everything committed so far, current street included
	ToCall    Chips // 0 when checking is legal
	Stacks    [MaxSeats]Chips
	Committed [MaxSeats]Chips // this street
	InHand    SeatSet         // seats not folded
	Button    Seat
	Blinds    Blinds
	History   []Event       // visible log: other seats' hole cards blanked
	Legal     ActionOptions // non-nil only when it is this seat's turn
}

// View builds the seat's information set. Returns nil if the seat was not
// dealt into the hand.
//
// The no-cheat invariant is unconditional: the view never contains a card
// outside Hole ∪ Board. Other seats' EvDealHole and EvShowdown cards are
// blanked from History — revealed showdown hands are public in real poker,
// but PlayerView exists for *acting*, and the review layer reads revealed
// cards through the Hand accessors instead.
func (h *Hand) View(seat Seat) *PlayerView {
	if !h.dealtIn.Has(seat) {
		return nil
	}
	v := &PlayerView{
		Seat:      seat,
		Hole:      h.holes[seat],
		Board:     h.Board(),
		Street:    h.street,
		Pot:       h.PotTotal(),
		ToCall:    h.ToCall(seat),
		Stacks:    h.stacks,
		Committed: h.bet.Committed,
		InHand:    h.live(),
		Button:    h.button,
		Blinds:    h.blinds,
	}
	v.History = make([]Event, len(h.events))
	copy(v.History, h.events)
	for i, e := range v.History {
		if (e.Kind == EvDealHole || e.Kind == EvShowdown) && e.Seat != seat {
			v.History[i].Cards = nil
			continue
		}
		if e.Cards != nil {
			v.History[i].Cards = make([]Card, len(e.Cards))
			copy(v.History[i].Cards, e.Cards)
		}
	}
	if seat == h.current {
		v.Legal = h.LegalActions()
	}
	return v
}

// --- Scripted setups ---------------------------------------------------------

// HandSetup describes an exact scenario, for lessons and tests. Anything
// omitted is filled deterministically from Seed.
type HandSetup struct {
	Config TableConfig
	Button Seat
	Stacks map[Seat]Chips   // seats present in the hand
	Holes  map[Seat][2]Card // optional; unspecified seats get seeded-random cards
	Board  []Card           // 0..5 cards; the runout prefix
	Seed   uint64           // fills every unspecified card; recorded for replay
	// Eval resolves showdowns. May be nil for hands that will not reach one.
	// TODO(wire-eval): inject the internal/eval-backed adapter.
	Eval Evaluator
}

// NewHandFromSetup builds a Hand in preflop betting with blinds posted. It
// validates the scenario (duplicate cards, seat/stack sanity, ≥2 seats),
// lays the scripted cards into deal order (§1.2 of the engine doc), and
// backs the hand with a ScriptedDeck so unscripted cards fall out of Seed.
func NewHandFromSetup(s HandSetup) (*Hand, error) {
	if s.Config.BigBlind <= 0 || s.Config.SmallBlind <= 0 {
		return nil, fmt.Errorf("engine: setup blinds must be positive (got %d/%d)",
			s.Config.SmallBlind, s.Config.BigBlind)
	}
	var dealtIn SeatSet
	var stacks [MaxSeats]Chips
	for seat, stack := range s.Stacks {
		if !seat.Valid() {
			return nil, fmt.Errorf("%w: %d", ErrInvalidSeat, seat)
		}
		if stack <= 0 {
			return nil, fmt.Errorf("%w: seat %d stack %d", ErrInvalidBuyIn, seat, stack)
		}
		dealtIn = dealtIn.Add(seat)
		stacks[seat] = stack
	}
	if dealtIn.Count() < 2 {
		return nil, ErrNotEnoughPlayers
	}
	if !dealtIn.Has(s.Button) {
		return nil, fmt.Errorf("%w: button %d not among dealt-in seats", ErrInvalidSeat, s.Button)
	}
	if len(s.Board) > 5 {
		return nil, fmt.Errorf("engine: setup board has %d cards, max 5", len(s.Board))
	}

	// Collect scripted cards, rejecting duplicates and holes for absent seats.
	var used CardSet
	take := func(c Card) error {
		if !c.Valid() {
			return fmt.Errorf("engine: setup card %d out of range", c)
		}
		if used.Has(c) {
			return fmt.Errorf("%w: %s", ErrDuplicateCard, c.Code())
		}
		used = used.Add(c)
		return nil
	}
	for seat, hole := range s.Holes {
		if !dealtIn.Has(seat) {
			return nil, fmt.Errorf("%w: hole cards for absent seat %d", ErrInvalidSeat, seat)
		}
		if err := take(hole[0]); err != nil {
			return nil, err
		}
		if err := take(hole[1]); err != nil {
			return nil, err
		}
	}
	for _, c := range s.Board {
		if err := take(c); err != nil {
			return nil, err
		}
	}

	// Fill unspecified holes from a seeded shuffle of the unused cards, then
	// lay everything into deal order: holes two at a time starting left of
	// the button (button last), then the scripted board prefix. Cards past
	// the prefix (turn/river of a partial script) come from the
	// ScriptedDeck's own seeded fallback over whatever remains.
	pool := make([]Card, 0, NumCards)
	for c := Card(0); c < NumCards; c++ {
		if !used.Has(c) {
			pool = append(pool, c)
		}
	}
	rng := newRNG(s.Seed)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	prefix := make([]Card, 0, 2*dealtIn.Count()+len(s.Board))
	for seat := dealtIn.Next(s.Button); ; seat = dealtIn.Next(seat) {
		hole, ok := s.Holes[seat]
		if !ok {
			hole = [2]Card{pool[0], pool[1]}
			pool = pool[2:]
		}
		prefix = append(prefix, hole[0], hole[1])
		if seat == s.Button {
			break
		}
	}
	prefix = append(prefix, s.Board...)

	deck := NewScriptedDeck(prefix, s.Seed)
	blinds := Blinds{Small: s.Config.SmallBlind, Big: s.Config.BigBlind}
	return newHand(blinds, s.Button, dealtIn, stacks, deck, s.Eval), nil
}
