package engine

import "fmt"

// TableConfig is the table's fixed configuration.
type TableConfig struct {
	SmallBlind, BigBlind Chips
	MinBuyIn, MaxBuyIn   Chips // e.g. 40BB / 100BB; zero means unconstrained
	Seats                int   // ≤ MaxSeats; 6 for this product
}

// SeatStatus is a seat's cross-hand state.
type SeatStatus uint8

// Seat statuses.
const (
	SeatEmpty      SeatStatus = iota
	SeatActive                // dealt into hands (while stack > 0)
	SeatSittingOut            // seated but not dealt
	// SeatWaitingForBB is seated (or returned from sitting out) but not dealt
	// in until the big blind reaches the seat. This replaces casino-style
	// dead blinds: posting dead money would corrupt the pot-odds arithmetic
	// the coach is trying to teach.
	SeatWaitingForBB
)

var seatStatusNames = [...]string{"empty", "active", "sitting-out", "waiting-for-bb"}

// String is the lowercase status name.
func (s SeatStatus) String() string {
	if int(s) >= len(seatStatusNames) {
		return "?"
	}
	return seatStatusNames[s]
}

// Table owns configuration and cross-hand state — stacks, button, seat
// status — and manufactures Hands. It is the analogue of euchre's Game.
type Table struct {
	// Eval resolves showdowns for the hands this table deals. The engine
	// cannot import internal/eval, so the app layer sets this once at
	// construction time. TODO(wire-eval).
	Eval Evaluator

	cfg     TableConfig
	status  [MaxSeats]SeatStatus
	names   [MaxSeats]string
	stacks  [MaxSeats]Chips
	button  Seat
	hands   int // completed hands
	current *Hand
}

// NewTable builds an empty table. An invalid configuration (non-positive
// blinds, SB > BB, bad seat count) is a programmer error and panics.
func NewTable(cfg TableConfig) *Table {
	if cfg.Seats <= 0 || cfg.Seats > MaxSeats {
		panic(fmt.Errorf("engine: TableConfig.Seats %d outside 1..%d", cfg.Seats, MaxSeats))
	}
	if cfg.SmallBlind <= 0 || cfg.BigBlind <= 0 || cfg.SmallBlind > cfg.BigBlind {
		panic(fmt.Errorf("engine: invalid blinds %d/%d", cfg.SmallBlind, cfg.BigBlind))
	}
	if cfg.MinBuyIn < 0 || cfg.MaxBuyIn < 0 || (cfg.MaxBuyIn > 0 && cfg.MinBuyIn > cfg.MaxBuyIn) {
		panic(fmt.Errorf("engine: invalid buy-in range %d..%d", cfg.MinBuyIn, cfg.MaxBuyIn))
	}
	return &Table{cfg: cfg, button: NoSeat}
}

// Config returns the table configuration.
func (t *Table) Config() TableConfig { return t.cfg }

// Button returns the current dealer button, NoSeat before the first hand.
func (t *Table) Button() Seat { return t.button }

// Stack returns a seat's chips.
func (t *Table) Stack(seat Seat) Chips {
	if !seat.Valid() {
		return 0
	}
	return t.stacks[seat]
}

// Status returns a seat's status.
func (t *Table) Status(seat Seat) SeatStatus {
	if !seat.Valid() {
		return SeatEmpty
	}
	return t.status[seat]
}

// Name returns the name a seat was seated with.
func (t *Table) Name(seat Seat) string {
	if !seat.Valid() {
		return ""
	}
	return t.names[seat]
}

// HandsPlayed returns the number of completed hands.
func (t *Table) HandsPlayed() int { return t.hands }

func (t *Table) checkSeat(seat Seat) error {
	if !seat.Valid() || int(seat) >= t.cfg.Seats {
		return fmt.Errorf("%w: %d", ErrInvalidSeat, seat)
	}
	return nil
}

func (t *Table) checkBetweenHands() error {
	if t.current != nil {
		return ErrHandInProgress
	}
	return nil
}

// Sit seats a player with a buy-in. Before the first hand the seat is
// immediately active; afterwards it waits for the big blind, like a
// returning sit-out — being dealt in mid-orbit for free would steal blinds.
func (t *Table) Sit(seat Seat, name string, buyIn Chips) error {
	if err := t.checkBetweenHands(); err != nil {
		return err
	}
	if err := t.checkSeat(seat); err != nil {
		return err
	}
	if t.status[seat] != SeatEmpty {
		return fmt.Errorf("%w: seat %d occupied", ErrInvalidSeat, seat)
	}
	if buyIn <= 0 ||
		(t.cfg.MinBuyIn > 0 && buyIn < t.cfg.MinBuyIn) ||
		(t.cfg.MaxBuyIn > 0 && buyIn > t.cfg.MaxBuyIn) {
		return fmt.Errorf("%w: %d", ErrInvalidBuyIn, buyIn)
	}
	t.names[seat] = name
	t.stacks[seat] = buyIn
	if t.hands == 0 {
		t.status[seat] = SeatActive
	} else {
		t.status[seat] = SeatWaitingForBB
	}
	return nil
}

// Leave empties a seat, cashing out its stack.
func (t *Table) Leave(seat Seat) error {
	if err := t.checkBetweenHands(); err != nil {
		return err
	}
	if err := t.checkSeat(seat); err != nil {
		return err
	}
	if t.status[seat] == SeatEmpty {
		return fmt.Errorf("%w: seat %d empty", ErrInvalidSeat, seat)
	}
	t.status[seat] = SeatEmpty
	t.names[seat] = ""
	t.stacks[seat] = 0
	return nil
}

// SitOut marks a seat as not to be dealt in.
func (t *Table) SitOut(seat Seat) error {
	if err := t.checkBetweenHands(); err != nil {
		return err
	}
	if err := t.checkSeat(seat); err != nil {
		return err
	}
	if t.status[seat] != SeatActive && t.status[seat] != SeatWaitingForBB {
		return fmt.Errorf("%w: seat %d not active", ErrInvalidSeat, seat)
	}
	t.status[seat] = SeatSittingOut
	return nil
}

// SitIn returns a sitting-out seat to play. The seat waits for the big
// blind: it is dealt in only when its seat is due to post it.
func (t *Table) SitIn(seat Seat) error {
	if err := t.checkBetweenHands(); err != nil {
		return err
	}
	if err := t.checkSeat(seat); err != nil {
		return err
	}
	if t.status[seat] != SeatSittingOut {
		return fmt.Errorf("%w: seat %d not sitting out", ErrInvalidSeat, seat)
	}
	t.status[seat] = SeatWaitingForBB
	return nil
}

// Rebuy adds chips to a seat between hands, capped so stack ≤ MaxBuyIn.
func (t *Table) Rebuy(seat Seat, amount Chips) error {
	if err := t.checkBetweenHands(); err != nil {
		return err
	}
	if err := t.checkSeat(seat); err != nil {
		return err
	}
	if t.status[seat] == SeatEmpty {
		return fmt.Errorf("%w: seat %d empty", ErrInvalidSeat, seat)
	}
	if amount <= 0 || (t.cfg.MaxBuyIn > 0 && t.stacks[seat]+amount > t.cfg.MaxBuyIn) {
		return fmt.Errorf("%w: rebuy %d", ErrInvalidBuyIn, amount)
	}
	t.stacks[seat] += amount
	return nil
}

// activeSeats returns the seats that would be dealt in right now: active
// with chips.
func (t *Table) activeSeats() SeatSet {
	var set SeatSet
	for s := Seat(0); int(s) < t.cfg.Seats; s++ {
		if t.status[s] == SeatActive && t.stacks[s] > 0 {
			set = set.Add(s)
		}
	}
	return set
}

// StartHand posts blinds, deals, and opens preflop betting. Pass nil to deal
// from a fresh crypto-seeded deck; pass a NewDeck or ScriptedDeck for
// deterministic replays and lessons.
func (t *Table) StartHand(src CardSource) (*Hand, error) {
	if t.current != nil {
		return nil, ErrHandInProgress
	}

	dealtIn := t.activeSeats()

	// Pick the button before admitting waiters: a waiting seat joins when
	// the big blind falls on it, which depends on where the button is.
	button := t.button
	if !dealtIn.Has(button) {
		button = dealtIn.Next(button) // first hand, or the button seat left
	}

	// Admit seats waiting for the big blind. If the game cannot otherwise
	// continue (fewer than two active seats) there is no orbit to wait on,
	// so every funded waiter is admitted instead.
	if dealtIn.Count() < 2 {
		for s := Seat(0); int(s) < t.cfg.Seats; s++ {
			if t.status[s] == SeatWaitingForBB && t.stacks[s] > 0 {
				t.status[s] = SeatActive
				dealtIn = dealtIn.Add(s)
			}
		}
		if !dealtIn.Has(button) {
			button = dealtIn.Next(button)
		}
	} else {
		// Scan clockwise from the button so that when two waiters could each
		// claim the big blind, the one the orbit reaches first gets it and
		// the other keeps waiting — admitting both would deal one of them in
		// without ever posting.
		for i := 1; i <= MaxSeats; i++ {
			s := Seat((int(button) + i) % MaxSeats)
			if int(s) >= t.cfg.Seats || t.status[s] != SeatWaitingForBB || t.stacks[s] <= 0 {
				continue
			}
			if bigBlindSeat(dealtIn.Add(s), button) == s {
				t.status[s] = SeatActive
				dealtIn = dealtIn.Add(s)
			}
		}
	}

	if dealtIn.Count() < 2 {
		return nil, ErrNotEnoughPlayers
	}
	t.button = button

	if src == nil {
		src = NewDeckRandom()
	}
	blinds := Blinds{Small: t.cfg.SmallBlind, Big: t.cfg.BigBlind}
	var stacks [MaxSeats]Chips
	for _, s := range dealtIn.Seats() {
		stacks[s] = t.stacks[s]
	}
	h := newHand(blinds, t.button, dealtIn, stacks, src, t.Eval)
	t.current = h
	return h, nil
}

// bigBlindSeat computes which seat posts the big blind for a given lineup:
// heads-up the non-button seat, otherwise two seats after the button.
func bigBlindSeat(dealtIn SeatSet, button Seat) Seat {
	if dealtIn.Count() == 2 {
		return dealtIn.Next(button)
	}
	return dealtIn.Next(dealtIn.Next(button))
}

// FinishHand applies a completed hand's result to the table's stacks and
// moves the button to the next seat that will be dealt in (simple moving
// button — the casino dead-button rule is deliberately not implemented; see
// the engine design §5).
func (t *Table) FinishHand(h *Hand) error {
	if t.current == nil || h != t.current {
		return fmt.Errorf("engine: FinishHand: not this table's live hand")
	}
	if h.Phase() != PhaseComplete {
		return ErrHandInProgress
	}
	for _, s := range h.DealtIn().Seats() {
		t.stacks[s] = h.Stack(s)
		// A seat finishing at zero stays SeatActive but is simply not dealt
		// (activeSeats requires chips); the app layer decides between rebuy
		// and sit-out — stack == 0 is the "busted" flag.
	}
	t.current = nil
	t.hands++
	t.button = t.activeSeats().Next(t.button)
	return nil
}

// Position names a seat's strategic position in this hand (BTN/SB/BB/UTG/
// HJ/CO), derived from the button and the dealt-in seats — never stored.
// Meaningful only for seats dealt into the hand.
func (h *Hand) Position(seat Seat) Position {
	return Positions(h.dealtIn, h.button)[seat]
}
