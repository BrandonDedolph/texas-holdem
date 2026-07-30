package eval

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// rank5 parses exactly five cards and ranks them.
func rank5(t *testing.T, s string) HandRank {
	t.Helper()
	cs := engine.MustCards(s)
	if len(cs) != 5 {
		t.Fatalf("rank5(%q): got %d cards, want 5", s, len(cs))
	}
	var hand [5]Card
	copy(hand[:], cs)
	return Eval5(hand)
}

// TestDescriptionGoldens pins the exact English for every category — the
// coach shows these strings verbatim, so wording changes must be deliberate.
func TestDescriptionGoldens(t *testing.T) {
	tests := []struct {
		cards        string
		wantString   string
		wantDescribe string
	}{
		{
			"As Ks Qs Js Ts",
			"Royal Flush",
			"Royal Flush",
		},
		{
			"9h 8h 7h 6h 5h",
			"Straight Flush, Nine high",
			"Straight Flush, Five to Nine",
		},
		{
			// The steel wheel is a FIVE-high straight flush, and both
			// forms must say so — the Ace plays low.
			"Ah 2h 3h 4h 5h",
			"Straight Flush, Five high",
			"Straight Flush, Ace to Five",
		},
		{
			"Qc Qd Qh Qs Ac",
			"Four of a Kind, Queens",
			"Four of a Kind, Queens with an Ace kicker",
		},
		{
			"8c 8d 8h 8s Kc",
			"Four of a Kind, Eights",
			"Four of a Kind, Eights with a King kicker",
		},
		{
			"Kc Kd Kh 4s 4c",
			"Full House, Kings full of Fours",
			"Full House, Kings full of Fours",
		},
		{
			"Kd Jd 9d 6d 4d",
			"Flush, King high",
			"Flush, King high (K-J-9-6-4)",
		},
		{
			"Kh Qs Jd Tc 9h",
			"Straight, King high",
			"Straight, Nine to King",
		},
		{
			// The wheel: five-high, never ace-high.
			"5h 4d 3c 2s Ah",
			"Straight, Five high",
			"Straight, Ace to Five",
		},
		{
			"7c 7d 7h Ad Ks",
			"Three of a Kind, Sevens",
			"Three of a Kind, Sevens with Ace and King kickers",
		},
		{
			"Ac Ad 9h 9s Qc",
			"Two Pair, Aces and Nines",
			"Two Pair, Aces and Nines with a Queen kicker",
		},
		{
			"6c 6d Th 8s Ac",
			"Pair of Sixes",
			"Pair of Sixes with Ace, Ten, and Eight kickers",
		},
		{
			"Ac Kd Qh 9s 5c",
			"High Card, Ace",
			"High Card, Ace with King, Queen, Nine, and Five kickers",
		},
	}
	for _, tt := range tests {
		r := rank5(t, tt.cards)
		if got := r.String(); got != tt.wantString {
			t.Errorf("Eval5(%s).String() = %q, want %q", tt.cards, got, tt.wantString)
		}
		if got := r.Describe(); got != tt.wantDescribe {
			t.Errorf("Eval5(%s).Describe() = %q, want %q", tt.cards, got, tt.wantDescribe)
		}
	}
}

// TestOrdering walks a ladder of hands, each strictly stronger than the
// last, spanning every category boundary and the kicker cases that most
// often go wrong (wheel below six-high straight, kicker-only differences).
func TestOrdering(t *testing.T) {
	ladder := []string{
		"7c 5d 4h 3s 2c", // the worst hand in poker
		"7c 6d 4h 3s 2c", // one kicker better
		"Ac Kd Qh Js 9c", // best high card
		"2c 2d 5h 4s 3c", // the worst pair beats the best high card
		"2c 2d Ah Ks Qc", // same pair, better kickers
		"Ac Ad 5h 4s 3c", // higher pair beats better kickers
		"2c 2d 3h 3s 4c", // worst two pair
		"Ac Ad Kh Ks Qc", // best two pair
		"2c 2d 2h 4s 3c", // worst trips
		"Ac Ad Ah Ks Qc", // best trips
		"5c 4d 3h 2s Ac", // the wheel: worst straight
		"6c 5d 4h 3s 2c", // six-high straight beats the wheel
		"Ac Kd Qh Js Tc", // Broadway
		"7c 5c 4c 3c 2c", // worst flush
		"Ac Kc Qc Jc 9c", // best flush
		"2c 2d 2h 3s 3c", // worst boat
		"Ac Ad Ah Ks Kc", // best boat
		"2c 2d 2h 2s 3c", // worst quads
		"Ac Ad Ah As Kc", // best quads
		"5c 4c 3c 2c Ac", // steel wheel: worst straight flush
		"Kc Qc Jc Tc 9c", // king-high straight flush
		"Ac Kc Qc Jc Tc", // royal flush
	}
	for i := 1; i < len(ladder); i++ {
		lo := rank5(t, ladder[i-1])
		hi := rank5(t, ladder[i])
		if !lo.Less(hi) {
			t.Errorf("expected %q (%s) < %q (%s)", ladder[i-1], lo.Describe(), ladder[i], hi.Describe())
		}
		if hi.Less(lo) || hi.Less(hi) {
			t.Errorf("Less is not a strict order at %q vs %q", ladder[i-1], ladder[i])
		}
	}
}

// TestTiesAcrossSuits: suits never break ties, so the same ranks in
// different suits must produce the identical HandRank.
func TestTiesAcrossSuits(t *testing.T) {
	pairs := [][2]string{
		{"Ac Kd Qh 9s 5c", "Ad Kh Qs 9c 5d"},
		{"Kd Jd 9d 6d 4d", "Kh Jh 9h 6h 4h"},
		{"5h 4d 3c 2s Ah", "5s 4c 3d 2h As"},
	}
	for _, p := range pairs {
		a, b := rank5(t, p[0]), rank5(t, p[1])
		if a != b {
			t.Errorf("suit-permuted hands rank differently: %q=%#x %q=%#x", p[0], uint32(a), p[1], uint32(b))
		}
	}
}

// TestCategoryDecode: the packed value must decode to the category the
// cards actually form — this is the property Describe() rests on.
func TestCategoryDecode(t *testing.T) {
	tests := []struct {
		cards string
		want  Category
	}{
		{"Ac Kd Qh 9s 5c", HighCard},
		{"6c 6d Th 8s Ac", OnePair},
		{"Ac Ad 9h 9s Qc", TwoPair},
		{"7c 7d 7h Ad Ks", ThreeOfAKind},
		{"5h 4d 3c 2s Ah", Straight},
		{"Kd Jd 9d 6d 4d", Flush},
		{"Kc Kd Kh 4s 4c", FullHouse},
		{"Qc Qd Qh Qs Ac", FourOfAKind},
		{"Ah 2h 3h 4h 5h", StraightFlush},
		{"As Ks Qs Js Ts", StraightFlush},
	}
	for _, tt := range tests {
		if got := rank5(t, tt.cards).Category(); got != tt.want {
			t.Errorf("Eval5(%s).Category() = %s, want %s", tt.cards, got, tt.want)
		}
	}
}

// TestCategoryString covers the enum's own names, which the stats screen
// uses for its category breakdown.
func TestCategoryString(t *testing.T) {
	want := []string{
		"High Card", "One Pair", "Two Pair", "Three of a Kind", "Straight",
		"Flush", "Full House", "Four of a Kind", "Straight Flush",
	}
	for c := HighCard; c < NumCategories; c++ {
		if got := c.String(); got != want[c] {
			t.Errorf("Category(%d).String() = %q, want %q", c, got, want[c])
		}
	}
	if got := Category(NumCategories).String(); got != "?" {
		t.Errorf("out-of-range Category.String() = %q, want %q", got, "?")
	}
}
