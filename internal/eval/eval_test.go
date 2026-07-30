package eval

import (
	"math/bits"
	"math/rand"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// cards7 parses exactly seven cards from "As Kd ..." notation.
func cards7(t *testing.T, s string) [7]Card {
	t.Helper()
	cs := engine.MustCards(s)
	if len(cs) != 7 {
		t.Fatalf("cards7(%q): got %d cards, want 7", s, len(cs))
	}
	var out [7]Card
	copy(out[:], cs)
	return out
}

// oracleBest5 is the deliberately dumb reference evaluator: try every 5-card
// subset with Eval5 and keep the best. Obviously correct, obviously slow.
func oracleBest5(cards []Card) HandRank {
	var best HandRank
	n := len(cards)
	for m := 0; m < 1<<n; m++ {
		if bits.OnesCount(uint(m)) != 5 {
			continue
		}
		var hand [5]Card
		k := 0
		for i := 0; i < n; i++ {
			if m>>i&1 == 1 {
				hand[k] = cards[i]
				k++
			}
		}
		if r := Eval5(hand); r > best {
			best = r
		}
	}
	return best
}

// TestExhaustiveFiveCardCensus enumerates all C(52,5) = 2,598,960 five-card
// hands and asserts the textbook category census. Royal flushes are counted
// separately from the other straight flushes here (36 + 4 = the usual 40);
// in the Category enum they are both StraightFlush, distinguished only by
// the top card being an Ace.
func TestExhaustiveFiveCardCensus(t *testing.T) {
	want := map[string]int{
		"High Card":                 1302540,
		"One Pair":                  1098240,
		"Two Pair":                  123552,
		"Three of a Kind":           54912,
		"Straight":                  10200,
		"Flush":                     5108,
		"Full House":                3744,
		"Four of a Kind":            624,
		"Straight Flush (no royal)": 36,
		"Royal Flush":               4,
	}

	var byCat [NumCategories]int
	royals := 0
	total := 0
	for a := 0; a < 48; a++ {
		for b := a + 1; b < 49; b++ {
			for c := b + 1; c < 50; c++ {
				for d := c + 1; d < 51; d++ {
					for e := d + 1; e < 52; e++ {
						r := Eval5([5]Card{Card(a), Card(b), Card(c), Card(d), Card(e)})
						byCat[r.Category()]++
						if r.Category() == StraightFlush && r.k(0) == engine.Ace {
							royals++
						}
						total++
					}
				}
			}
		}
	}

	got := map[string]int{
		"High Card":                 byCat[HighCard],
		"One Pair":                  byCat[OnePair],
		"Two Pair":                  byCat[TwoPair],
		"Three of a Kind":           byCat[ThreeOfAKind],
		"Straight":                  byCat[Straight],
		"Flush":                     byCat[Flush],
		"Full House":                byCat[FullHouse],
		"Four of a Kind":            byCat[FourOfAKind],
		"Straight Flush (no royal)": byCat[StraightFlush] - royals,
		"Royal Flush":               royals,
	}
	if total != 2598960 {
		t.Fatalf("enumerated %d hands, want 2598960", total)
	}
	sum := 0
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %d, want %d", name, got[name], w)
		}
		sum += got[name]
	}
	if sum != total {
		t.Errorf("census buckets sum to %d, want %d", sum, total)
	}
}

// TestEval7MatchesOracle cross-checks the direct 7-card evaluator against the
// 21-combination reference over a large random sample. Any disagreement is a
// bug in Eval7's shortcut logic (the oracle has none to get wrong).
func TestEval7MatchesOracle(t *testing.T) {
	draws := 200000
	if testing.Short() {
		draws = 20000
	}
	rng := rand.New(rand.NewSource(1))
	var deck [engine.NumCards]Card
	for i := range deck {
		deck[i] = Card(i)
	}
	for n := 0; n < draws; n++ {
		// Partial Fisher-Yates: the first 7 cards are a uniform draw.
		var hand [7]Card
		for i := 0; i < 7; i++ {
			j := i + rng.Intn(len(deck)-i)
			deck[i], deck[j] = deck[j], deck[i]
			hand[i] = deck[i]
		}
		got := Eval7(hand)
		want := oracleBest5(hand[:])
		if got != want {
			t.Fatalf("Eval7(%v) = %#x (%s), oracle = %#x (%s)",
				engine.CardsString(hand[:]), uint32(got), got.Describe(), uint32(want), want.Describe())
		}
	}
}

// TestEval7Adversarial pins the cases where a shortcut could silently lie:
// wheels, straight flush vs. higher flush cards, seven of one suit, boards
// that tempt a flush/boat confusion, quads on the board. Each entry is also
// checked against the oracle, so the expectation itself is double-checked.
func TestEval7Adversarial(t *testing.T) {
	tests := []struct {
		cards string
		want  string // Describe() of the best hand
	}{
		{"As 2d 3c 4h 5s Kd Qc", "Straight, Ace to Five"},
		{"Ah 2h 3h 4h 5h Kd Qc", "Straight Flush, Ace to Five"},
		{"As 2s 3s 4s 5s 6s 7s", "Straight Flush, Three to Seven"},
		{"6h 5h 4h 3h 2h Ah Kh", "Straight Flush, Two to Six"},
		{"Ah Kh Qh Jh Th 9h 8h", "Royal Flush"},
		// Trips plus a five-card flush: the flush plays, and the shortcut
		// must not be fooled into a boat that is not there.
		{"Kh Ks Kd Ah Qh Jh 9h", "Flush, Ace high with King, Queen, Jack, and Nine kickers"},
		// A real boat with only four suited cards: the flush path must not fire.
		{"As Ad Ac Kh Kd Qh Jh", "Full House, Aces full of Kings"},
		{"Kc Kd Kh Ks Ac 2d 3h", "Four of a Kind, Kings with an Ace kicker"},
		// Quads plus trips: the trips supply only the kicker.
		{"5h 5d 5c 5s 6h 6d 6c", "Four of a Kind, Fives with a Six kicker"},
		// Two trips make a boat of the higher over the lower.
		{"2c 2d 2h 3c 3d 3s 4s", "Full House, Threes full of Twos"},
		// Three pairs: best two play, and the third pair's rank competes
		// with the loose card for the kicker.
		{"Ac Ad 9h 9s Kc Kd 5h", "Two Pair, Aces and Kings with a Nine kicker"},
		// Straight on the board, flush draw that misses.
		{"9c 8d 7h 6s 5c Ah Kh", "Straight, Five to Nine"},
	}
	for _, tt := range tests {
		hand := cards7(t, tt.cards)
		got := Eval7(hand)
		if oracle := oracleBest5(hand[:]); got != oracle {
			t.Errorf("Eval7(%s) = %s disagrees with oracle %s", tt.cards, got.Describe(), oracle.Describe())
		}
		if got.Describe() != tt.want {
			t.Errorf("Eval7(%s).Describe() = %q, want %q", tt.cards, got.Describe(), tt.want)
		}
	}
}

// TestEvalHoldemBoardSizes checks the 5- and 6-card core against the oracle
// on every street, since Eval7's random cross-check only exercises n=7.
func TestEvalHoldemBoardSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	var deck [engine.NumCards]Card
	for i := range deck {
		deck[i] = Card(i)
	}
	for n := 0; n < 20000; n++ {
		var draw [7]Card
		for i := 0; i < 7; i++ {
			j := i + rng.Intn(len(deck)-i)
			deck[i], deck[j] = deck[j], deck[i]
			draw[i] = deck[i]
		}
		hole := [2]Card{draw[0], draw[1]}
		for boardLen := 3; boardLen <= 5; boardLen++ {
			board := draw[2 : 2+boardLen]
			got := EvalHoldem(hole, board)
			want := oracleBest5(draw[:2+boardLen])
			if got != want {
				t.Fatalf("EvalHoldem(%v | %v) = %s, oracle = %s",
					engine.CardsString(hole[:]), engine.CardsString(board), got.Describe(), want.Describe())
			}
		}
	}
}

func TestEvalHoldemPanicsOnBadBoard(t *testing.T) {
	for _, n := range []int{0, 1, 2, 6} {
		board := make([]Card, n)
		for i := range board {
			board[i] = Card(10 + i)
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("EvalHoldem with %d-card board did not panic", n)
				}
			}()
			EvalHoldem([2]Card{Card(0), Card(1)}, board)
		}()
	}
}

// TestBest5 checks the three properties the UI depends on: the rank matches
// Eval7, the reported five cards actually evaluate to that rank, and they
// are a subset of the input.
func TestBest5(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	var deck [engine.NumCards]Card
	for i := range deck {
		deck[i] = Card(i)
	}
	for n := 0; n < 5000; n++ {
		var draw [7]Card
		for i := 0; i < 7; i++ {
			j := i + rng.Intn(len(deck)-i)
			deck[i], deck[j] = deck[j], deck[i]
			draw[i] = deck[i]
		}
		hole := [2]Card{draw[0], draw[1]}
		board := draw[2:7]

		rank, five := Best5(hole, board)
		if want := Eval7(draw); rank != want {
			t.Fatalf("Best5 rank %s != Eval7 rank %s for %v", rank.Describe(), want.Describe(), engine.CardsString(draw[:]))
		}
		if got := Eval5(five); got != rank {
			t.Fatalf("Best5 returned cards %v ranking %s, claimed %s", engine.CardsString(five[:]), got.Describe(), rank.Describe())
		}
		in := engine.NewCardSet(draw[:]...)
		seen := engine.CardSet(0)
		for _, c := range five {
			if !in.Has(c) {
				t.Fatalf("Best5 returned %s not among inputs %v", c, engine.CardsString(draw[:]))
			}
			if seen.Has(c) {
				t.Fatalf("Best5 returned duplicate card %s", c)
			}
			seen = seen.Add(c)
		}
	}
}

// TestBest5PlaysTheBoard pins the classic case the highlight exists for:
// when the board plays, neither hole card may be in the five.
func TestBest5PlaysTheBoard(t *testing.T) {
	hole := [2]Card{engine.MustCard("2c"), engine.MustCard("7d")}
	board := engine.MustCards("Kc Kd Kh Ks Ac")
	rank, five := Best5(hole, board)
	if got, want := rank.Describe(), "Four of a Kind, Kings with an Ace kicker"; got != want {
		t.Fatalf("rank = %q, want %q", got, want)
	}
	want := engine.NewCardSet(board...)
	got := engine.NewCardSet(five[:]...)
	if got != want {
		t.Errorf("Best5 played %v, want the board %v", engine.CardsString(five[:]), engine.CardsString(board))
	}
}

// The equity code budget assumes Eval7 allocates nothing; this pins it.
var allocSink HandRank

func TestEval7NoAllocs(t *testing.T) {
	hand := cards7(t, "Ah Kd 7c 7h 2s 9d Jc")
	if n := testing.AllocsPerRun(1000, func() {
		allocSink = Eval7(hand)
	}); n != 0 {
		t.Errorf("Eval7 allocates %.1f objects per call, want 0", n)
	}
}

func BenchmarkEval7(b *testing.B) {
	rng := rand.New(rand.NewSource(4))
	var deck [engine.NumCards]Card
	for i := range deck {
		deck[i] = Card(i)
	}
	hands := make([][7]Card, 1024)
	for h := range hands {
		for i := 0; i < 7; i++ {
			j := i + rng.Intn(len(deck)-i)
			deck[i], deck[j] = deck[j], deck[i]
			hands[h][i] = deck[i]
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allocSink = Eval7(hands[i&1023])
	}
}
