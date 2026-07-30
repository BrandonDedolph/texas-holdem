package engine

import "math/bits"

// MaxSeats is the table capacity. The product is 6-max; the engine is written
// against this constant rather than the number 6.
const MaxSeats = 6

// Seat identifies a seat at the table. Valid seats are 0..MaxSeats-1;
// NoSeat means "no seat".
type Seat int8

// NoSeat is the absent-seat sentinel, returned by accessors like CurrentSeat
// when no decision is pending.
const NoSeat Seat = -1

// Valid reports whether s addresses a real seat.
func (s Seat) Valid() bool { return s >= 0 && s < MaxSeats }

// SeatSet is a bitmask of seats.
type SeatSet uint8

// NewSeatSet builds a set from seats.
func NewSeatSet(seats ...Seat) SeatSet {
	var set SeatSet
	for _, s := range seats {
		set = set.Add(s)
	}
	return set
}

// Has reports whether s is in the set.
func (set SeatSet) Has(s Seat) bool { return s.Valid() && set&(1<<uint8(s)) != 0 }

// Add returns the set with s included.
func (set SeatSet) Add(s Seat) SeatSet {
	if !s.Valid() {
		return set
	}
	return set | 1<<uint8(s)
}

// Remove returns the set with s excluded.
func (set SeatSet) Remove(s Seat) SeatSet {
	if !s.Valid() {
		return set
	}
	return set &^ (1 << uint8(s))
}

// Count returns the number of seats in the set.
func (set SeatSet) Count() int { return bits.OnesCount8(uint8(set)) }

// Empty reports whether the set has no seats.
func (set SeatSet) Empty() bool { return set == 0 }

// Seats expands the set into a slice in ascending seat order.
func (set SeatSet) Seats() []Seat {
	out := make([]Seat, 0, set.Count())
	for s := Seat(0); s < MaxSeats; s++ {
		if set.Has(s) {
			out = append(out, s)
		}
	}
	return out
}

// First returns the lowest seat in the set, or NoSeat if empty.
func (set SeatSet) First() Seat {
	if set == 0 {
		return NoSeat
	}
	return Seat(bits.TrailingZeros8(uint8(set)))
}

// Next returns the next seat in the set clockwise from (and excluding) after,
// wrapping around the table. Returns NoSeat if the set is empty.
func (set SeatSet) Next(after Seat) Seat {
	if set == 0 {
		return NoSeat
	}
	for i := 1; i <= MaxSeats; i++ {
		s := Seat((int(after) + i) % MaxSeats)
		if set.Has(s) {
			return s
		}
	}
	return NoSeat
}

// Position names a seat's strategic position at the table. Position is a
// function of the button and the set of seats dealt in, never stored state.
type Position uint8

// Positions, in preflop action order after the blinds.
const (
	PosUTG Position = iota // first to act preflop
	PosHJ                  // hijack
	PosCO                  // cutoff
	PosBTN                 // button (dealer)
	PosSB                  // small blind
	PosBB                  // big blind
)

var positionNames = [...]string{"UTG", "HJ", "CO", "BTN", "SB", "BB"}

// String is the short label ("BTN").
func (p Position) String() string {
	if int(p) >= len(positionNames) {
		return "?"
	}
	return positionNames[p]
}

var positionLongNames = [...]string{
	"Under the Gun", "Hijack", "Cutoff", "Button", "Small Blind", "Big Blind",
}

// Long is the spelled-out name ("Button"), for lessons and the coach.
func (p Position) Long() string {
	if int(p) >= len(positionLongNames) {
		return "?"
	}
	return positionLongNames[p]
}

// Positions assigns a Position to each seat dealt in, given the button.
//
// The blinds are anchored first (the button is the small blind heads-up), then
// the remaining seats are labelled backwards from the button: the seat before
// the button is the cutoff, before that the hijack, and everything earlier is
// under the gun. Returns a map from seat to position covering exactly dealtIn.
func Positions(dealtIn SeatSet, button Seat) map[Seat]Position {
	out := make(map[Seat]Position, dealtIn.Count())
	n := dealtIn.Count()
	if n == 0 || !dealtIn.Has(button) {
		return out
	}
	if n == 1 {
		out[button] = PosBTN
		return out
	}

	// Heads-up: the button posts the small blind.
	if n == 2 {
		out[button] = PosBTN
		out[dealtIn.Next(button)] = PosBB
		return out
	}

	sb := dealtIn.Next(button)
	bb := dealtIn.Next(sb)
	out[button] = PosBTN
	out[sb] = PosSB
	out[bb] = PosBB

	// Label the seats between the big blind and the button. Walking backwards
	// from the button gives CO, HJ; anything further out is UTG.
	late := []Position{PosCO, PosHJ}
	remaining := make([]Seat, 0, MaxSeats)
	for s := dealtIn.Next(bb); s != button; s = dealtIn.Next(s) {
		remaining = append(remaining, s)
	}
	for i, s := range remaining {
		fromButton := len(remaining) - 1 - i // 0 == immediately before the button
		if fromButton < len(late) {
			out[s] = late[fromButton]
		} else {
			out[s] = PosUTG
		}
	}
	return out
}
