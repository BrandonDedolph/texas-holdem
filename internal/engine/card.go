package engine

import (
	"fmt"
	"math/bits"
	"strings"
)

// Rank is a card rank. Two is 0 and Ace is 12, so numeric order is rank order.
type Rank uint8

// Card ranks, low to high.
const (
	Two Rank = iota
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
	NumRanks = 13
)

// Suit is a card suit. Suits have no rank order in Hold'em; the numbering
// exists only to make Card packing total.
type Suit uint8

// Card suits.
const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
	NumSuits = 4
)

// Card is a compact card encoding: Card = Rank<<2 | Suit, values 0..51.
//
// The rank-major layout means c>>2 is the rank — the operation the hand
// evaluator performs constantly — and sorting cards numerically sorts by rank.
// See docs/design-engine.md §1.1 for why this is a uint8 rather than a struct.
type Card uint8

// NumCards is the size of a standard deck.
const NumCards = 52

// noCard is the zero value's sentinel. Card(0) is a legitimate card (2♣), so
// code that needs "no card" uses a separate bool rather than a magic value.

// MakeCard builds a Card from a rank and suit.
func MakeCard(r Rank, s Suit) Card { return Card(uint8(r)<<2 | uint8(s)) }

// Rank returns the card's rank.
func (c Card) Rank() Rank { return Rank(c >> 2) }

// Suit returns the card's suit.
func (c Card) Suit() Suit { return Suit(c & 3) }

// Valid reports whether c is within the 52-card range.
func (c Card) Valid() bool { return c < NumCards }

const rankLetters = "23456789TJQKA"

var (
	suitGlyphs  = [NumSuits]string{"♣", "♦", "♥", "♠"}
	suitLetters = [NumSuits]string{"c", "d", "h", "s"}
	rankNames   = [NumRanks]string{
		"Two", "Three", "Four", "Five", "Six", "Seven", "Eight",
		"Nine", "Ten", "Jack", "Queen", "King", "Ace",
	}
	rankPlurals = [NumRanks]string{
		"Twos", "Threes", "Fours", "Fives", "Sixes", "Sevens", "Eights",
		"Nines", "Tens", "Jacks", "Queens", "Kings", "Aces",
	}
)

// Letter is the single-character form of a rank ("T" for Ten).
func (r Rank) Letter() byte { return rankLetters[r] }

// String is the English name of a rank ("Ace").
func (r Rank) String() string {
	if r >= NumRanks {
		return "?"
	}
	return rankNames[r]
}

// Plural is the English plural of a rank ("Aces"), used by hand descriptions.
func (r Rank) Plural() string {
	if r >= NumRanks {
		return "?"
	}
	return rankPlurals[r]
}

// Glyph is the Unicode suit symbol.
func (s Suit) Glyph() string {
	if s >= NumSuits {
		return "?"
	}
	return suitGlyphs[s]
}

// String is the single-letter suit code ("s" for spades).
func (s Suit) String() string {
	if s >= NumSuits {
		return "?"
	}
	return suitLetters[s]
}

// String is the display form of a card: "A♠", "T♦".
func (c Card) String() string {
	if !c.Valid() {
		return "??"
	}
	return string(c.Rank().Letter()) + c.Suit().Glyph()
}

// Code is the canonical two-character form: "As", "Td". This is the notation
// used in tests, lesson content, and hand histories.
func (c Card) Code() string {
	if !c.Valid() {
		return "??"
	}
	return string(c.Rank().Letter()) + c.Suit().String()
}

// ParseRank parses a single rank character. It accepts "10" only via
// ParseCard; here a rank is exactly one byte.
func ParseRank(b byte) (Rank, error) {
	if b >= 'a' && b <= 'z' {
		b -= 'a' - 'A'
	}
	if i := strings.IndexByte(rankLetters, b); i >= 0 {
		return Rank(i), nil
	}
	return 0, fmt.Errorf("engine: invalid rank %q", string(b))
}

// ParseSuit parses a suit from a letter ("s") or a Unicode glyph ("♠").
func ParseSuit(s string) (Suit, error) {
	lower := strings.ToLower(s)
	for i, l := range suitLetters {
		if lower == l {
			return Suit(i), nil
		}
	}
	for i, g := range suitGlyphs {
		if s == g {
			return Suit(i), nil
		}
	}
	return 0, fmt.Errorf("engine: invalid suit %q", s)
}

// ParseCard parses one card. It accepts "As", "as", "A♠", and "10h".
func ParseCard(s string) (Card, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("engine: empty card")
	}
	// Accept "10x" as an alias for "Tx".
	if strings.HasPrefix(s, "10") {
		s = "T" + s[2:]
	}
	r, err := ParseRank(s[0])
	if err != nil {
		return 0, err
	}
	suit, err := ParseSuit(s[1:])
	if err != nil {
		return 0, err
	}
	return MakeCard(r, suit), nil
}

// MustCard parses one card and panics on failure. For tests and lesson content.
func MustCard(s string) Card {
	c, err := ParseCard(s)
	if err != nil {
		panic(err)
	}
	return c
}

// ParseCards parses a whitespace-separated list ("As Kd 7h") or a run of
// two-character codes ("AsKd7h"). Glyph forms require whitespace separation.
func ParseCards(s string) ([]Card, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if fields := strings.Fields(s); len(fields) > 1 {
		out := make([]Card, 0, len(fields))
		for _, f := range fields {
			c, err := ParseCard(f)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	}
	// Single token: either one card or a run of two-character codes.
	if len([]rune(s)) <= 2 {
		c, err := ParseCard(s)
		if err != nil {
			return nil, err
		}
		return []Card{c}, nil
	}
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("engine: cannot split %q into two-character cards", s)
	}
	out := make([]Card, 0, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		c, err := ParseCard(s[i : i+2])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// MustCards parses a card list and panics on failure.
func MustCards(s string) []Card {
	cards, err := ParseCards(s)
	if err != nil {
		panic(err)
	}
	return cards
}

// Holes parses exactly two cards into a hole-card pair. For lesson content and
// tests: engine.Holes("9c 9d").
func Holes(s string) [2]Card {
	cards := MustCards(s)
	if len(cards) != 2 {
		panic(fmt.Errorf("engine: Holes(%q) yielded %d cards, want 2", s, len(cards)))
	}
	return [2]Card{cards[0], cards[1]}
}

// CardsString renders a card slice as space-separated codes ("As Kd 7h").
func CardsString(cards []Card) string {
	if len(cards) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range cards {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(c.Code())
	}
	return b.String()
}

// CardSet is a bitmask of cards; bit c is set iff Card(c) is in the set.
type CardSet uint64

// NewCardSet builds a set from cards.
func NewCardSet(cards ...Card) CardSet {
	var s CardSet
	for _, c := range cards {
		s = s.Add(c)
	}
	return s
}

// FullDeck is the set of all 52 cards.
func FullDeck() CardSet { return CardSet(1)<<NumCards - 1 }

// Has reports whether c is in the set.
func (s CardSet) Has(c Card) bool { return s&(1<<c) != 0 }

// Add returns the set with c included.
func (s CardSet) Add(c Card) CardSet { return s | 1<<c }

// Remove returns the set with c excluded.
func (s CardSet) Remove(c Card) CardSet { return s &^ (1 << c) }

// Count returns the number of cards in the set.
func (s CardSet) Count() int { return bits.OnesCount64(uint64(s)) }

// Cards expands the set into a slice, in ascending card order.
func (s CardSet) Cards() []Card {
	out := make([]Card, 0, s.Count())
	for rest := uint64(s); rest != 0; rest &= rest - 1 {
		out = append(out, Card(bits.TrailingZeros64(rest)))
	}
	return out
}

// String renders the set as space-separated codes.
func (s CardSet) String() string { return CardsString(s.Cards()) }
