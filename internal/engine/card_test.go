package engine

import "testing"

func TestCardPackingRoundTrips(t *testing.T) {
	seen := map[Card]bool{}
	for r := Two; r <= Ace; r++ {
		for s := Clubs; s <= Spades; s++ {
			c := MakeCard(r, s)
			if !c.Valid() {
				t.Fatalf("MakeCard(%v,%v) = %d, out of range", r, s, c)
			}
			if c.Rank() != r || c.Suit() != s {
				t.Fatalf("MakeCard(%v,%v) round-tripped to (%v,%v)", r, s, c.Rank(), c.Suit())
			}
			if seen[c] {
				t.Fatalf("MakeCard(%v,%v) = %d collides with an earlier card", r, s, c)
			}
			seen[c] = true
		}
	}
	if len(seen) != NumCards {
		t.Fatalf("got %d distinct cards, want %d", len(seen), NumCards)
	}
}

func TestCardOrderingIsRankOrdering(t *testing.T) {
	// Rank-major packing must make numeric card order agree with rank order.
	if !(MakeCard(Two, Spades) < MakeCard(Three, Clubs)) {
		t.Fatal("2s should sort below 3c under rank-major packing")
	}
	if !(MakeCard(King, Spades) < MakeCard(Ace, Clubs)) {
		t.Fatal("Ks should sort below Ac under rank-major packing")
	}
}

func TestParseCard(t *testing.T) {
	tests := []struct {
		in   string
		want Card
	}{
		{"As", MakeCard(Ace, Spades)},
		{"as", MakeCard(Ace, Spades)},
		{"A♠", MakeCard(Ace, Spades)},
		{"Td", MakeCard(Ten, Diamonds)},
		{"10d", MakeCard(Ten, Diamonds)},
		{"10♦", MakeCard(Ten, Diamonds)},
		{"2c", MakeCard(Two, Clubs)},
		{"9h", MakeCard(Nine, Hearts)},
	}
	for _, tc := range tests {
		got, err := ParseCard(tc.in)
		if err != nil {
			t.Errorf("ParseCard(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseCard(%q) = %v, want %v", tc.in, got.Code(), tc.want.Code())
		}
	}

	for _, bad := range []string{"", "X", "1s", "Ax", "A", "sA"} {
		if c, err := ParseCard(bad); err == nil {
			t.Errorf("ParseCard(%q) = %v, want error", bad, c.Code())
		}
	}
}

func TestCardStringAndCodeRoundTrip(t *testing.T) {
	for c := Card(0); c < NumCards; c++ {
		if got := MustCard(c.Code()); got != c {
			t.Fatalf("Code round-trip: %q parsed back to %q", c.Code(), got.Code())
		}
		if got := MustCard(c.String()); got != c {
			t.Fatalf("String round-trip: %q parsed back to %q", c.String(), got.Code())
		}
	}
}

func TestTenDisplaysAs10(t *testing.T) {
	// The owner's decision: the ten renders "10" on output everywhere;
	// "T" survives only as an input alias.
	ten := MakeCard(Ten, Diamonds)
	if got := ten.Code(); got != "10d" {
		t.Errorf("Code = %q, want %q", got, "10d")
	}
	if got := ten.String(); got != "10♦" {
		t.Errorf("String = %q, want %q", got, "10♦")
	}
	if got := Ten.Symbol(); got != "10" {
		t.Errorf("Symbol = %q, want %q", got, "10")
	}
	if got := CardsString([]Card{ten, MustCard("9s")}); got != "10d 9s" {
		t.Errorf("CardsString = %q, want %q", got, "10d 9s")
	}
}

func TestParseCards(t *testing.T) {
	want := []Card{MustCard("As"), MustCard("Kd"), MustCard("7h")}
	for _, in := range []string{"As Kd 7h", "AsKd7h", "  As   Kd 7h  ", "as kd 7h"} {
		got, err := ParseCards(in)
		if err != nil {
			t.Fatalf("ParseCards(%q) error: %v", in, err)
		}
		if len(got) != len(want) {
			t.Fatalf("ParseCards(%q) = %v, want %v", in, CardsString(got), CardsString(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ParseCards(%q) = %v, want %v", in, CardsString(got), CardsString(want))
			}
		}
	}

	// Runs and lists round-trip the three-character "10" codes too.
	wantTens := []Card{MustCard("As"), MustCard("Td"), MustCard("Th")}
	for _, in := range []string{"As 10d 10h", "As10d10h", "As Td 10h", "AsTd10h"} {
		got, err := ParseCards(in)
		if err != nil {
			t.Fatalf("ParseCards(%q) error: %v", in, err)
		}
		if CardsString(got) != CardsString(wantTens) {
			t.Fatalf("ParseCards(%q) = %v, want %v", in, CardsString(got), CardsString(wantTens))
		}
	}
	if got, err := ParseCards(CardsString(wantTens)); err != nil || CardsString(got) != "As 10d 10h" {
		t.Fatalf("CardsString round-trip = %v (%v), want As 10d 10h", CardsString(got), err)
	}

	if _, err := ParseCards("AsKd7"); err == nil {
		t.Error("ParseCards of an odd-length run should error")
	}
}

func TestHoles(t *testing.T) {
	h := Holes("9c 9d")
	if h[0] != MustCard("9c") || h[1] != MustCard("9d") {
		t.Fatalf("Holes(\"9c 9d\") = %v", CardsString(h[:]))
	}
}

func TestCardSet(t *testing.T) {
	s := NewCardSet(MustCard("As"), MustCard("Kd"))
	if !s.Has(MustCard("As")) || !s.Has(MustCard("Kd")) {
		t.Fatal("set is missing a card it was built with")
	}
	if s.Has(MustCard("Qh")) {
		t.Fatal("set contains a card it was not built with")
	}
	if s.Count() != 2 {
		t.Fatalf("Count = %d, want 2", s.Count())
	}
	if got := s.Remove(MustCard("As")); got.Count() != 1 || got.Has(MustCard("As")) {
		t.Fatal("Remove did not remove the card")
	}
	// Add is idempotent.
	if s.Add(MustCard("As")) != s {
		t.Fatal("Add of a present card changed the set")
	}
	if FullDeck().Count() != NumCards {
		t.Fatalf("FullDeck has %d cards, want %d", FullDeck().Count(), NumCards)
	}
	if got := len(FullDeck().Cards()); got != NumCards {
		t.Fatalf("FullDeck().Cards() has %d cards, want %d", got, NumCards)
	}
}

func TestChipsString(t *testing.T) {
	tests := []struct {
		in   Chips
		want string
	}{
		{0, "0"}, {5, "5"}, {999, "999"}, {1000, "1,000"},
		{1240, "1,240"}, {12345, "12,345"}, {1234567, "1,234,567"},
		{-1240, "-1,240"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Chips(%d).String() = %q, want %q", int64(tc.in), got, tc.want)
		}
	}
}

func TestBlindsBB(t *testing.T) {
	b := Blinds{Small: 5, Big: 10}
	tests := []struct {
		in   Chips
		want string
	}{
		{10, "1bb"}, {125, "12.5bb"}, {1000, "100bb"}, {5, "0.5bb"}, {0, "0bb"},
	}
	for _, tc := range tests {
		if got := b.BB(tc.in); got != tc.want {
			t.Errorf("BB(%d) = %q, want %q", int64(tc.in), got, tc.want)
		}
	}
}
