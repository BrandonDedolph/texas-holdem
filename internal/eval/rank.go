package eval

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// Category classifies a five-card poker hand. Higher categories beat lower
// ones; ties within a category are broken by the kicker fields in HandRank.
type Category uint8

// Hand categories, weakest to strongest.
const (
	HighCard Category = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush // an Ace-high straight flush describes itself as "Royal Flush"
	NumCategories = 9
)

var categoryNames = [NumCategories]string{
	"High Card", "One Pair", "Two Pair", "Three of a Kind", "Straight",
	"Flush", "Full House", "Four of a Kind", "Straight Flush",
}

// String is the English name of a category ("Three of a Kind").
func (c Category) String() string {
	if c >= NumCategories {
		return "?"
	}
	return categoryNames[c]
}

// HandRank is a totally-ordered hand strength. Higher is better. Layout:
//
//	bits 23..20  Category
//	bits 19..0   five 4-bit rank fields k1..k5, category-specific:
//	  HighCard:      k1..k5 = the five ranks, descending
//	  OnePair:       k1 = pair rank, k2..k4 = kickers descending, k5 = 0
//	  TwoPair:       k1 = high pair, k2 = low pair, k3 = kicker, k4 = k5 = 0
//	  ThreeOfAKind:  k1 = trips rank, k2..k3 = kickers descending, k4 = k5 = 0
//	  Straight:      k1 = top card of the straight (Five for the wheel)
//	  Flush:         k1..k5 = the five ranks, descending
//	  FullHouse:     k1 = trips rank, k2 = pair rank, k3..k5 = 0
//	  FourOfAKind:   k1 = quad rank, k2 = kicker, k3..k5 = 0
//	  StraightFlush: k1 = top card of the straight (Five for the steel wheel)
//
// Because the layout is category-then-lexicographic-kickers, integer
// comparison IS hand comparison, and the fields decode back into English —
// the coach's Describe() is a pure decode, no reverse table needed.
type HandRank uint32

// pack assembles a HandRank from its category and kicker fields. Unused
// fields must be zero; two hands of the same category always populate the
// same fields, so zero-padding never distorts comparison.
func pack(cat Category, k1, k2, k3, k4, k5 engine.Rank) HandRank {
	return HandRank(uint32(cat)<<20 |
		uint32(k1)<<16 | uint32(k2)<<12 | uint32(k3)<<8 | uint32(k4)<<4 | uint32(k5))
}

// Category returns the hand's category.
func (r HandRank) Category() Category { return Category(r >> 20 & 0xF) }

// k returns kicker field i (0-based: k(0) is k1).
func (r HandRank) k(i uint) engine.Rank { return engine.Rank(r >> (16 - 4*i) & 0xF) }

// Less reports whether r loses to o. It is just r < o; the method exists so
// call sites read as hand comparison rather than integer trivia.
func (r HandRank) Less(o HandRank) bool { return r < o }

// String is the short form the table UI shows:
// "Flush, King high" / "Two Pair, Aces and Nines" / "Royal Flush".
func (r HandRank) String() string {
	switch r.Category() {
	case HighCard:
		return "High Card, " + r.k(0).String()
	case OnePair:
		return "Pair of " + r.k(0).Plural()
	case TwoPair:
		return "Two Pair, " + r.k(0).Plural() + " and " + r.k(1).Plural()
	case ThreeOfAKind:
		return "Three of a Kind, " + r.k(0).Plural()
	case Straight:
		return "Straight, " + r.k(0).String() + " high"
	case Flush:
		return "Flush, " + r.k(0).String() + " high"
	case FullHouse:
		return "Full House, " + r.k(0).Plural() + " full of " + r.k(1).Plural()
	case FourOfAKind:
		return "Four of a Kind, " + r.k(0).Plural()
	case StraightFlush:
		if r.k(0) == engine.Ace {
			return "Royal Flush"
		}
		return "Straight Flush, " + r.k(0).String() + " high"
	}
	return "?"
}

// Describe is the full form the coach and post-hand review use:
// "Two Pair, Aces and Nines with a Queen kicker" / "Straight, Nine to King" /
// "Full House, Kings full of Fours". The wheel decodes as "Ace to Five" so a
// learner sees it is a five-high straight, not an ace-high one.
func (r HandRank) Describe() string {
	switch r.Category() {
	case HighCard:
		return "High Card, " + r.k(0).String() + kickerPhrase(r.k(1), r.k(2), r.k(3), r.k(4))
	case OnePair:
		return "Pair of " + r.k(0).Plural() + kickerPhrase(r.k(1), r.k(2), r.k(3))
	case TwoPair:
		return r.String() + kickerPhrase(r.k(2))
	case ThreeOfAKind:
		return r.String() + kickerPhrase(r.k(1), r.k(2))
	case Straight:
		return "Straight, " + straightSpan(r.k(0))
	case Flush:
		return r.String() + kickerPhrase(r.k(1), r.k(2), r.k(3), r.k(4))
	case FullHouse:
		return r.String()
	case FourOfAKind:
		return r.String() + kickerPhrase(r.k(1))
	case StraightFlush:
		if r.k(0) == engine.Ace {
			return "Royal Flush"
		}
		return "Straight Flush, " + straightSpan(r.k(0))
	}
	return "?"
}

// straightSpan renders a straight's extent, low card to high: "Nine to King".
// The wheel's low card is the Ace, hence "Ace to Five".
func straightSpan(top engine.Rank) string {
	low := top - 4
	if top == engine.Five {
		low = engine.Ace
	}
	return low.String() + " to " + top.String()
}

// kickerPhrase renders the trailing kicker clause of Describe:
// " with a Queen kicker", " with Ace and King kickers",
// " with King, Queen, and Nine kickers".
func kickerPhrase(ks ...engine.Rank) string {
	switch len(ks) {
	case 0:
		return ""
	case 1:
		return " with " + article(ks[0]) + " " + ks[0].String() + " kicker"
	case 2:
		return " with " + ks[0].String() + " and " + ks[1].String() + " kickers"
	}
	var b strings.Builder
	b.WriteString(" with ")
	for i, k := range ks {
		if i == len(ks)-1 {
			b.WriteString("and ")
		}
		b.WriteString(k.String())
		if i < len(ks)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteString(" kickers")
	return b.String()
}

// article picks "a" or "an" for a rank name — this is a teaching tool, and
// "a Ace kicker" would read as a bug.
func article(r engine.Rank) string {
	if r == engine.Ace || r == engine.Eight {
		return "an"
	}
	return "a"
}
