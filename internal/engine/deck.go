package engine

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
)

// CardSource supplies cards in deal order. Implementations must panic only on
// programmer error (drawing more than 52 cards).
type CardSource interface {
	Draw() Card
}

// SourceCloner is implemented by card sources that can be deep-copied.
// Hand.Clone uses it so a cloned hand's hypothetical run-out does not consume
// cards from the real hand's deck. Deck and ScriptedDeck both implement it;
// custom sources that do not are shared between clones (documented on Clone).
type SourceCloner interface {
	CloneSource() CardSource
}

// pcgSalt is the fixed second word fed to rand/v2's PCG constructor. PCG takes
// two 64-bit seed words; the engine exposes a single seed (the one recorded in
// hand histories), so the second word is a constant. Any constant works — it
// only has to be the same on every platform and Go release forever.
const pcgSalt = 0x9e3779b97f4a7c15

// newRNG builds the engine's deterministic generator. rand/v2's PCG is a
// specified algorithm: the same seed produces the same shuffle everywhere,
// which is what replayable hands and scripted lessons require. The global
// rand source is deliberately never used in the engine.
func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, pcgSalt))
}

// Deck is a shuffled 52-card source. The full shuffle is fixed at
// construction, so a Deck is trivially cloneable and its behavior is a pure
// function of its seed.
type Deck struct {
	cards [NumCards]Card
	next  int
	seed  uint64
}

// NewDeck returns a full deck shuffled deterministically from seed.
func NewDeck(seed uint64) *Deck {
	d := &Deck{seed: seed}
	for i := range d.cards {
		d.cards[i] = Card(i)
	}
	r := newRNG(seed)
	r.Shuffle(NumCards, func(i, j int) {
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	})
	return d
}

// NewDeckRandom returns a deck shuffled from a crypto-random seed, for normal
// play. The seed is retained (see Seed) so even "random" hands can be
// recorded and replayed exactly.
func NewDeckRandom() *Deck {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		// crypto/rand failing is unrecoverable and essentially impossible;
		// there is no sensible fallback that preserves the security intent.
		panic(fmt.Errorf("engine: crypto/rand failed: %w", err))
	}
	return NewDeck(binary.LittleEndian.Uint64(b[:]))
}

// Seed returns the seed the deck was shuffled from, for hand records.
func (d *Deck) Seed() uint64 { return d.seed }

// Draw deals the next card. Drawing past 52 is a programmer error and panics
// with ErrDeckExhausted.
func (d *Deck) Draw() Card {
	if d.next >= NumCards {
		panic(ErrDeckExhausted)
	}
	c := d.cards[d.next]
	d.next++
	return c
}

// CloneSource returns an independent copy of the deck, positioned at the same
// next card.
func (d *Deck) CloneSource() CardSource {
	cp := *d
	return &cp
}

// ScriptedDeck deals a fixed prefix of cards, then falls back to a seeded
// shuffle of the remaining cards. Lessons usually script only the hero's
// cards and the flop; the fallback fills in the rest without the lesson
// author enumerating 52 cards.
//
// Like Deck, the full order is fixed at construction: prefix first, then the
// unscripted remainder shuffled by fallbackSeed.
type ScriptedDeck struct {
	cards []Card
	next  int
}

// NewScriptedDeck builds a scripted deck. A duplicate or invalid card in the
// prefix is a programmer error and panics (NewHandFromSetup validates user
// input and returns errors before ever reaching here).
func NewScriptedDeck(prefix []Card, fallbackSeed uint64) *ScriptedDeck {
	var used CardSet
	cards := make([]Card, 0, NumCards)
	for _, c := range prefix {
		if !c.Valid() {
			panic(fmt.Errorf("engine: scripted card %d out of range", c))
		}
		if used.Has(c) {
			panic(fmt.Errorf("%w: %s scripted twice", ErrDuplicateCard, c.Code()))
		}
		used = used.Add(c)
		cards = append(cards, c)
	}

	rest := make([]Card, 0, NumCards-len(cards))
	for c := Card(0); c < NumCards; c++ {
		if !used.Has(c) {
			rest = append(rest, c)
		}
	}
	r := newRNG(fallbackSeed)
	r.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	return &ScriptedDeck{cards: append(cards, rest...)}
}

// Draw deals the next card: the scripted prefix in order, then the shuffled
// remainder. Drawing past 52 panics with ErrDeckExhausted.
func (d *ScriptedDeck) Draw() Card {
	if d.next >= len(d.cards) {
		panic(ErrDeckExhausted)
	}
	c := d.cards[d.next]
	d.next++
	return c
}

// CloneSource returns an independent copy positioned at the same next card.
func (d *ScriptedDeck) CloneSource() CardSource {
	cp := &ScriptedDeck{cards: make([]Card, len(d.cards)), next: d.next}
	copy(cp.cards, d.cards)
	return cp
}
