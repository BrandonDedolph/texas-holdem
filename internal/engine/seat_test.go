package engine

import "testing"

func TestSeatSet(t *testing.T) {
	s := NewSeatSet(0, 3, 5)
	if !s.Has(0) || !s.Has(3) || !s.Has(5) {
		t.Fatal("set is missing a seat it was built with")
	}
	if s.Has(1) || s.Has(NoSeat) {
		t.Fatal("set contains a seat it was not built with")
	}
	if s.Count() != 3 {
		t.Fatalf("Count = %d, want 3", s.Count())
	}
	if s.First() != 0 {
		t.Fatalf("First = %d, want 0", s.First())
	}
	if SeatSet(0).First() != NoSeat {
		t.Fatal("First of an empty set should be NoSeat")
	}
	if !SeatSet(0).Empty() {
		t.Fatal("zero SeatSet should be empty")
	}
}

func TestSeatSetNextWraps(t *testing.T) {
	s := NewSeatSet(0, 3, 5)
	tests := []struct{ after, want Seat }{
		{0, 3}, {1, 3}, {3, 5}, {4, 5}, {5, 0}, {2, 3},
	}
	for _, tc := range tests {
		if got := s.Next(tc.after); got != tc.want {
			t.Errorf("Next(%d) = %d, want %d", tc.after, got, tc.want)
		}
	}
	// A single-seat set returns that seat from anywhere, including itself.
	one := NewSeatSet(2)
	if got := one.Next(2); got != 2 {
		t.Errorf("single-seat Next(2) = %d, want 2", got)
	}
	if got := SeatSet(0).Next(0); got != NoSeat {
		t.Errorf("empty Next = %d, want NoSeat", got)
	}
}

func TestPositionsSixHanded(t *testing.T) {
	all := NewSeatSet(0, 1, 2, 3, 4, 5)
	pos := Positions(all, 0)
	want := map[Seat]Position{
		0: PosBTN, 1: PosSB, 2: PosBB, 3: PosUTG, 4: PosHJ, 5: PosCO,
	}
	for seat, wantPos := range want {
		if got := pos[seat]; got != wantPos {
			t.Errorf("seat %d = %v, want %v", seat, got, wantPos)
		}
	}
}

func TestPositionsHeadsUpButtonIsSmallBlind(t *testing.T) {
	// Heads-up, the button posts the small blind and there is no separate SB seat.
	pos := Positions(NewSeatSet(1, 4), 1)
	if pos[1] != PosBTN {
		t.Errorf("button seat = %v, want BTN", pos[1])
	}
	if pos[4] != PosBB {
		t.Errorf("other seat = %v, want BB", pos[4])
	}
	if len(pos) != 2 {
		t.Errorf("got %d positions, want 2", len(pos))
	}
}

func TestPositionsThreeHanded(t *testing.T) {
	// Three-handed there is no room for CO/HJ/UTG: everyone is a blind or the button.
	pos := Positions(NewSeatSet(0, 2, 4), 0)
	if pos[0] != PosBTN || pos[2] != PosSB || pos[4] != PosBB {
		t.Errorf("three-handed positions = %v/%v/%v, want BTN/SB/BB", pos[0], pos[2], pos[4])
	}
}

func TestPositionsFourHanded(t *testing.T) {
	// The seat between the BB and the button is the cutoff, not UTG.
	pos := Positions(NewSeatSet(0, 1, 2, 3), 0)
	want := map[Seat]Position{0: PosBTN, 1: PosSB, 2: PosBB, 3: PosCO}
	for seat, wantPos := range want {
		if got := pos[seat]; got != wantPos {
			t.Errorf("seat %d = %v, want %v", seat, got, wantPos)
		}
	}
}

func TestPositionsSkipsEmptySeats(t *testing.T) {
	// Positions follow the dealt-in seats clockwise, ignoring gaps.
	pos := Positions(NewSeatSet(0, 2, 4, 5), 5)
	want := map[Seat]Position{5: PosBTN, 0: PosSB, 2: PosBB, 4: PosCO}
	for seat, wantPos := range want {
		if got := pos[seat]; got != wantPos {
			t.Errorf("seat %d = %v, want %v", seat, got, wantPos)
		}
	}
}

func TestPositionsButtonNotDealtIn(t *testing.T) {
	if got := Positions(NewSeatSet(1, 2), 0); len(got) != 0 {
		t.Errorf("button not dealt in should yield no positions, got %v", got)
	}
}
