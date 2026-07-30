package engine

// Street-order and adjacency helpers for the table UI. Every function here
// is a pure read of state the hand already tracks — the button, the dealt-in
// set, and the current street — so the UI can stamp each seat plate with its
// position in this street's action order and re-stamp when the street turns.

// SmallBlindSeat returns the seat that posted the small blind this hand.
// Heads-up the button is the small blind.
func (h *Hand) SmallBlindSeat() Seat {
	if h.dealtIn.Count() > 2 {
		return h.dealtIn.Next(h.button)
	}
	return h.button
}

// BigBlindSeat returns the seat that posted the big blind this hand.
func (h *Hand) BigBlindSeat() Seat {
	return h.dealtIn.Next(h.SmallBlindSeat())
}

// StreetOrder returns the seats dealt into this hand in the order they act
// on the current street, first to act first. Preflop the action opens left
// of the big blind, so the blinds act last; postflop it opens left of the
// button, so the blinds act first — the preflop→postflop renumbering of the
// same fixed plates is the rule this helper exists to make visible. Heads-up
// the button posts the small blind, acting first preflop and last postflop.
//
// Every dealt-in seat appears exactly once, folded and all-in seats
// included: a fold vacates a turn, not a position on the rim. Callers that
// need only the seats still able to act filter with Live() and AllIn().
func (h *Hand) StreetOrder() []Seat {
	after := h.button
	if h.street == Preflop {
		after = h.BigBlindSeat()
	}
	out := make([]Seat, 0, h.dealtIn.Count())
	for s, n := h.dealtIn.Next(after), h.dealtIn.Count(); n > 0; s, n = h.dealtIn.Next(s), n-1 {
		out = append(out, s)
	}
	return out
}

// LeftOf returns the seat at s's immediate left: the next dealt-in seat
// clockwise, the neighbour who acts directly after s in an unopened round.
// Returns NoSeat when s is the only dealt-in seat.
func (h *Hand) LeftOf(s Seat) Seat { return h.dealtIn.Remove(s).Next(s) }

// RightOf returns the seat at s's immediate right: the previous dealt-in
// seat counter-clockwise, the neighbour who acts directly before s in an
// unopened round. Returns NoSeat when s is the only dealt-in seat.
func (h *Hand) RightOf(s Seat) Seat { return h.dealtIn.Remove(s).Prev(s) }
