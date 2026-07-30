package ai

// Personality is a parameter block over the one rule-based strategy. Every
// archetype at the table — and the coach — is the same code running with a
// different Personality; this is DESIGN.md §1 principle 2 made structural.
// A value of 1.0 in every dial is the baseline TAG game the coach teaches.
type Personality struct {
	Key   string // stable identifier: "tag", "nit", ...
	Label string // display name: "Tight-Aggressive"
	Blurb string // one-line table read shown to the player

	// RangeScale scales every preflop chart: 1.0 plays the chart as
	// printed, 0.7 plays the strongest 70% of it (a nit), 1.6 extends past
	// it into progressively weaker hands (a LAG). Charts are written
	// strongest-first exactly so this can be a prefix operation.
	RangeScale float64

	// Aggression scales the preference for betting and raising over
	// calling. Below ~0.7 the strategy starts calling in spots the
	// baseline raises (the station's limp); above ~1.4 it starts raising
	// hands the baseline only calls (the maniac's top-pair raises).
	Aggression float64

	// BluffFreq multiplies every baseline bluffing frequency (c-bets with
	// air, semi-bluff raises, river bluffs). 0 never bluffs; 1 is baseline;
	// >1 bluffs more often than is good for you.
	BluffFreq float64

	// CallDown discounts the equity required to call: the strategy calls
	// when equity ≥ required × CallDown. 1.0 demands the true price; a
	// station's 0.6 calls needing only 60% of it; a nit's 1.1 demands a
	// premium. Below 0.8 an opponent is treated as unbluffable — the
	// "never bluff into a calling station" rule keys off this dial.
	CallDown float64

	// CBetFreq multiplies the baseline continuation-bet frequency.
	CBetFreq float64

	// Patience resists tilting bet sizes upward. At 1.0 the strategy uses
	// its standard sizes; near 0 it reaches for the large size whenever it
	// bets or raises (the maniac's overbets).
	Patience float64
}
