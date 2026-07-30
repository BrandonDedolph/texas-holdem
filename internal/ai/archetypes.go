package ai

// Archetypes are the built-in opponent presets (design-learning.md §4.6).
// Each exists to teach one specific lesson, noted in its Blurb and
// compressed into the one-word seat-plate Read. The numbers follow the
// design table: Range / Aggression / Bluff / CallDown / CBet.
//
// "tag" is the baseline: every dial 1.0, the chart as printed. "coach" is
// the same numbers under the coach's name — the coach is literally the TAG
// bot advising from the hero's seat, which is what keeps the coach and the
// opponents from ever teaching contradictions.
var Archetypes = map[string]Personality{
	"nit": {
		Key: "nit", Label: "The Nit", Read: "tight",
		Blurb:      "Plays very few hands. When this player raises, fold good hands — and steal their blinds.",
		RangeScale: 0.6, Aggression: 0.9, BluffFreq: 0.2, CallDown: 1.1, CBetFreq: 0.7, Patience: 1.2,
	},
	"tag": {
		Key: "tag", Label: "Tight-Aggressive", Read: "solid",
		Blurb:      "Solid, disciplined, positionally aware. This is the game you are learning to play.",
		RangeScale: 1.0, Aggression: 1.0, BluffFreq: 1.0, CallDown: 1.0, CBetFreq: 1.0, Patience: 1.0,
	},
	"lag": {
		Key: "lag", Label: "Loose-Aggressive", Read: "loose",
		Blurb:      "Raises a lot of hands. Their raises mean less — call down lighter, but don't get run over.",
		RangeScale: 1.5, Aggression: 1.3, BluffFreq: 1.8, CallDown: 0.95, CBetFreq: 1.3, Patience: 0.7,
	},
	"station": {
		Key: "station", Label: "The Calling Station", Read: "sticky",
		Blurb:      "Calls with anything. Never bluff them; value-bet thinner and bigger.",
		RangeScale: 1.6, Aggression: 0.6, BluffFreq: 0.1, CallDown: 0.6, CBetFreq: 0.8, Patience: 1.0,
	},
	"maniac": {
		Key: "maniac", Label: "The Maniac", Read: "wild",
		Blurb:      "Raises constantly, huge. Tighten up, let them hang themselves, stack them with big hands.",
		RangeScale: 1.9, Aggression: 1.8, BluffFreq: 2.5, CallDown: 0.8, CBetFreq: 1.6, Patience: 0.1,
	},
	"coach": {
		Key: "coach", Label: "Coach", Read: "solid",
		Blurb:      "The baseline tight-aggressive game, advising from your seat.",
		RangeScale: 1.0, Aggression: 1.0, BluffFreq: 1.0, CallDown: 1.0, CBetFreq: 1.0, Patience: 1.0,
	},
}

// ArchetypeKeys lists the archetypes in teaching order — the order the
// setup screen and dossier panel present them. "coach" is deliberately
// absent: it is a role, not a seatable opponent.
var ArchetypeKeys = []string{"nit", "tag", "lag", "station", "maniac"}

// Archetype returns a preset by key.
func Archetype(key string) (Personality, bool) {
	p, ok := Archetypes[key]
	return p, ok
}

// Baseline returns the TAG personality — the chart-as-printed game every
// other archetype is a distortion of, and the personality the coach runs.
func Baseline() Personality { return Archetypes["tag"] }

// ClassroomMix is the default opponent lineup for a 6-max table: the hero
// plus one of each archetype (DESIGN.md §2.11).
var ClassroomMix = []string{"nit", "tag", "lag", "station", "maniac"}
