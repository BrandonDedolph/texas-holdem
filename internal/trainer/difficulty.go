package trainer

import (
	"math/rand/v2"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// MaxLevel is the top difficulty of every quiz kind (design-learning.md §9
// defines exactly three levels per quiz).
const MaxLevel = 3

// Skill tags. The fine-grained tags name one sub-skill each and carry the
// EMA that bends item selection toward weakness; the per-kind gate tag
// (kindTag) carries the level progression. Two trackers on purpose: the
// gate's attempts reset on level-up (an average earned at an easier level
// says nothing about this one), while a sub-skill's EMA is a lifetime
// weakness signal that must survive level changes.
const (
	tagRankCategory    = "rank.category"    // L1: different hand categories
	tagRankKicker      = "rank.kicker"      // L2: same category, kickers decide
	tagRankSplit       = "rank.split"       // L3: the board plays — split trap
	tagRankCounterfeit = "rank.counterfeit" // L3: a made pair that never plays

	tagOutsFlush   = "outs.flush"   // L1: pure nine-out flush draw
	tagOutsOESD    = "outs.oesd"    // L1: pure eight-out open-ender
	tagOutsCombo   = "outs.combo"   // L2: flush + straight combo draw
	tagOutsTainted = "outs.tainted" // L3: some outs complete villain too

	tagEquityFlop  = "equity.flop"  // L1: rule of 4 on the flop
	tagEquityTurn  = "equity.turn"  // L2: rule of 2 on the turn
	tagEquityRange = "equity.range" // L3: versus a named range

	tagSpotPreflop = "spot.preflop"    // L1: preflop chart spots
	tagSpotFlop    = "spot.flop-price" // L2: flop pot-odds spots
	tagSpotLate    = "spot.turn-river" // L3: turn/river with a read shown
)

// quizTags maps each kind to its tags per level (index 0 is level 1).
var quizTags = map[QuizKind][MaxLevel][]string{
	QuizRankings: {
		{tagRankCategory},
		{tagRankKicker},
		{tagRankSplit, tagRankCounterfeit},
	},
	QuizOuts: {
		{tagOutsFlush, tagOutsOESD},
		{tagOutsCombo},
		{tagOutsTainted},
	},
	QuizEquity: {
		{tagEquityFlop},
		{tagEquityTurn},
		{tagEquityRange},
	},
	QuizSpots: {
		{tagSpotPreflop},
		{tagSpotFlop},
		{tagSpotLate},
	},
}

// levelBlurbs is what each level of each kind drills, in the player's
// vocabulary — one short clause per level, derived from the tag table above
// (docs/ui-review.md §5.6: the ladder shows its mechanism). Kept next to
// quizTags so a tag change and its description change in the same diff.
var levelBlurbs = map[QuizKind][MaxLevel]string{
	QuizRankings: {
		"different hand categories",
		"same category, kickers decide",
		"the board plays: splits and counterfeits",
	},
	QuizOuts: {
		"clean flush and straight draws",
		"combo draws: flush plus straight",
		"tainted outs that also help the villain",
	},
	QuizEquity: {
		"rule of 4 on the flop",
		"rule of 2 on the turn",
		"equity against a named range",
	},
	QuizSpots: {
		"preflop chart decisions",
		"flop pot-odds decisions",
		"turn and river with a read",
	},
}

// LevelBlurb describes what one 1-based level of a kind drills, for the
// trainer's ladder. Out-of-range levels return "".
func LevelBlurb(kind QuizKind, level int) string {
	blurbs, ok := levelBlurbs[kind]
	if !ok || level < 1 || level > MaxLevel {
		return ""
	}
	return blurbs[level-1]
}

// GateStat is the profile stat gating a kind's next level unlock — the EMA
// and attempt count the ladder's progress line reports. Safe on nil.
func GateStat(prof *profile.Profile, kind QuizKind) profile.SkillStat {
	if prof == nil {
		return profile.SkillStat{}
	}
	return prof.DrillStats[kindTag(kind)]
}

// kindTag is the gate tag a kind's level progression is recorded under. It
// is the kind's String() — one vocabulary, so a profile reads sensibly.
func kindTag(kind QuizKind) string { return kind.String() }

// Level reports the 1-based difficulty a new session of this kind will run
// at, for the trainer menu. Level N+1 unlocks when the gate tag's stat
// reaches ≥80% EMA over ≥20 answers at level N (profile.SkillStat).
func Level(prof *profile.Profile, kind QuizKind) int { return levelFor(prof, kind) }

func levelFor(prof *profile.Profile, kind QuizKind) int {
	if prof == nil {
		return 1
	}
	lvl := prof.DrillStats[kindTag(kind)].Level + 1
	if lvl > MaxLevel {
		lvl = MaxLevel
	}
	return lvl
}

// tagPool is every tag available at a level: the level's own tags plus all
// earlier ones. Lower-level skills stay in rotation so the weakness
// weighting has somewhere to send a player whose fundamentals slipped.
func tagPool(kind QuizKind, level int) []string {
	if level < 1 {
		level = 1
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	var pool []string
	for l := 0; l < level; l++ {
		pool = append(pool, quizTags[kind][l]...)
	}
	return pool
}

// weightFloor keeps every tag in rotation. Without it a mastered skill
// (EMA 1 → weight 0) would never be asked again and silently decay; with a
// large floor the weighting would stop mattering. 0.05 keeps a mastered tag
// at roughly one item in twenty against a fresh weak tag.
const weightFloor = 0.05

// pickTag samples a tag weighted by (1 − EMA)² + floor. Squaring bends
// practice toward weakness without turning the session into a punishment
// loop: a 60% skill gets ~4× the weight of an 80% skill, not 40×.
func pickTag(rng *rand.Rand, prof *profile.Profile, tags []string) string {
	if len(tags) == 1 {
		return tags[0]
	}
	weights := make([]float64, len(tags))
	total := 0.0
	for i, tag := range tags {
		ema := 0.0
		if prof != nil {
			ema = prof.DrillStats[tag].EMA
		}
		miss := 1 - ema
		weights[i] = miss*miss + weightFloor
		total += weights[i]
	}
	x := rng.Float64() * total
	for i, w := range weights {
		x -= w
		if x < 0 {
			return tags[i]
		}
	}
	return tags[len(tags)-1]
}
