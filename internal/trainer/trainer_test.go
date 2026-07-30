package trainer

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// profAtLevel returns a fresh profile with a kind's gate tag pinned to a
// 0-based level, so tests can generate sessions at any difficulty.
func profAtLevel(kind QuizKind, level int) *profile.Profile {
	p := profile.NewProfile()
	p.DrillStats[kindTag(kind)] = profile.SkillStat{Level: level}
	return p
}

// trimFloat renders a float without trailing zeros ("7.5", "9") — the
// tests' input formatter for numeric answers.
func trimFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// correctInput is the input string that answers an item correctly. For spot
// items the ChoiceAnswer's Correct index is the coach's own pick, which
// grades Best.
func correctInput(t *testing.T, it *Item) string {
	t.Helper()
	switch a := it.Drill.Answer.(type) {
	case tutorial.ChoiceAnswer:
		return strconv.Itoa(a.Correct + 1)
	case tutorial.NumericAnswer:
		return trimFloat(a.Value)
	default:
		t.Fatalf("unexpected answer type %T", it.Drill.Answer)
		return ""
	}
}

// TestSessionDeterministic: the same seed and profile generate the
// identical slate — items, prompts, answers, frozen advice and all.
func TestSessionDeterministic(t *testing.T) {
	for kind := QuizKind(0); kind < NumQuizKinds; kind++ {
		a := NewSessionSeeded(kind, profile.NewProfile(), 42)
		b := NewSessionSeeded(kind, profile.NewProfile(), 42)
		if !reflect.DeepEqual(a.Items, b.Items) {
			t.Errorf("%s: identical seeds generated different items", kind)
		}
		c := NewSessionSeeded(kind, profile.NewProfile(), 43)
		if reflect.DeepEqual(a.Items, c.Items) {
			t.Errorf("%s: different seeds generated identical items — RNG unused?", kind)
		}
	}
}

// TestSessionScoring: correct answers extend the streak and score a point;
// a wrong answer resets the streak but keeps the best.
func TestSessionScoring(t *testing.T) {
	s := NewSessionSeeded(QuizRankings, profile.NewProfile(), 7)
	if !s.Timed {
		t.Error("rankings sessions must be timed")
	}
	if len(s.Items) != SessionItems {
		t.Fatalf("session has %d items, want %d", len(s.Items), SessionItems)
	}

	out := s.Answer(correctInput(t, s.Current()))
	if !out.Correct || out.Points != 1 {
		t.Fatalf("correct answer scored %+v", out)
	}
	s.Answer(correctInput(t, s.Current()))
	if s.Streak != 2 || s.BestStreak != 2 {
		t.Fatalf("streak %d best %d after two correct, want 2/2", s.Streak, s.BestStreak)
	}

	// A deliberately wrong choice: any index other than the correct one.
	wrong := "1"
	if s.Current().Drill.Answer.(tutorial.ChoiceAnswer).Correct == 0 {
		wrong = "2"
	}
	out = s.Answer(wrong)
	if out.Correct || out.Points != 0 {
		t.Fatalf("wrong answer scored %+v", out)
	}
	if s.Streak != 0 || s.BestStreak != 2 {
		t.Fatalf("streak %d best %d after a miss, want 0/2", s.Streak, s.BestStreak)
	}
	if s.Answered != 3 || s.Correct != 2 || s.Score != 2 {
		t.Fatalf("answered=%d correct=%d score=%v, want 3/2/2", s.Answered, s.Correct, s.Score)
	}
	if out.Explain == "" {
		t.Error("every answer must carry the explanation, right or wrong")
	}
}

// TestEquityHalfCredit: within ±5 scores full, within ±10 half (recorded as
// a miss), beyond ±10 nothing.
func TestEquityHalfCredit(t *testing.T) {
	cases := []struct {
		offset  float64
		points  float64
		correct bool
	}{
		{3, 1, true},
		{-5, 1, true},
		{7, 0.5, false},
		{-10, 0.5, false},
		{12, 0, false},
	}
	for _, tc := range cases {
		s := NewSessionSeeded(QuizEquity, profile.NewProfile(), 11)
		it := s.Current()
		value := it.Drill.Answer.(tutorial.NumericAnswer).Value
		out := s.Answer(trimFloat(value + tc.offset))
		if out.Correct != tc.correct || out.Points != tc.points {
			t.Errorf("offset %+v: got correct=%v points=%v, want %v/%v",
				tc.offset, out.Correct, out.Points, tc.correct, tc.points)
		}
	}
}

// TestLevelGates: the session level follows the gate tag's stored level and
// clamps at MaxLevel; the tag pool grows with the level.
func TestLevelGates(t *testing.T) {
	for kind := QuizKind(0); kind < NumQuizKinds; kind++ {
		if got := Level(profile.NewProfile(), kind); got != 1 {
			t.Errorf("%s: fresh profile level = %d, want 1", kind, got)
		}
		if got := Level(profAtLevel(kind, 1), kind); got != 2 {
			t.Errorf("%s: level-1 stat gives session level %d, want 2", kind, got)
		}
		if got := Level(profAtLevel(kind, 9), kind); got != MaxLevel {
			t.Errorf("%s: level clamps at %d, got %d", kind, MaxLevel, got)
		}
		if got := Level(nil, kind); got != 1 {
			t.Errorf("%s: nil profile level = %d, want 1", kind, got)
		}

		l1, l3 := tagPool(kind, 1), tagPool(kind, 3)
		if len(l3) <= len(l1) {
			t.Errorf("%s: level 3 pool (%d tags) must exceed level 1 (%d)", kind, len(l3), len(l1))
		}
	}
}

// TestUnlockThreshold pins the boundary: ≥80% EMA over ≥20 answers unlocks,
// and neither side fires early. The arithmetic lives in profile.SkillStat;
// this test holds the trainer to it end to end.
func TestUnlockThreshold(t *testing.T) {
	// The exact boundaries, on the stat itself.
	boundaries := []struct {
		stat profile.SkillStat
		want bool
	}{
		{profile.SkillStat{EMA: 0.8, Attempts: 20}, true},
		{profile.SkillStat{EMA: 0.799, Attempts: 20}, false},
		{profile.SkillStat{EMA: 0.9, Attempts: 19}, false},
		{profile.SkillStat{EMA: 1, Attempts: 100}, true},
	}
	for _, b := range boundaries {
		if got := b.stat.UnlockReady(); got != b.want {
			t.Errorf("UnlockReady(EMA=%v, n=%d) = %v, want %v", b.stat.EMA, b.stat.Attempts, got, b.want)
		}
	}

	// End to end: one answer short of the attempt gate does not level up...
	prof := profile.NewProfile()
	prof.DrillStats[kindTag(QuizRankings)] = profile.SkillStat{EMA: 0.95, Attempts: 18}
	s := NewSessionSeeded(QuizRankings, prof, 3)
	if out := s.Answer(correctInput(t, s.Current())); out.LeveledUp {
		t.Fatal("leveled up at 19 attempts — the gate is ≥20")
	}
	// ...and the 20th answer, with the EMA holding, does.
	out := s.Answer(correctInput(t, s.Current()))
	if !out.LeveledUp || !s.LeveledUp {
		t.Fatal("20th attempt at ≥80% EMA must unlock the next level")
	}
	got := prof.DrillStats[kindTag(QuizRankings)]
	if got.Level != 1 || got.Attempts != 0 {
		t.Fatalf("post-unlock stat = %+v, want level 1 with a fresh count", got)
	}

	// A low EMA never unlocks, no matter the attempt count.
	prof2 := profile.NewProfile()
	prof2.DrillStats[kindTag(QuizRankings)] = profile.SkillStat{EMA: 0.4, Attempts: 50}
	s2 := NewSessionSeeded(QuizRankings, prof2, 3)
	if out := s2.Answer(correctInput(t, s2.Current())); out.LeveledUp {
		t.Fatal("leveled up below the 80% EMA gate")
	}

	// At MaxLevel there is nothing to unlock.
	prof3 := profAtLevel(QuizRankings, MaxLevel-1)
	prof3.DrillStats[kindTag(QuizRankings)] = profile.SkillStat{EMA: 1, Attempts: 40, Level: MaxLevel - 1}
	s3 := NewSessionSeeded(QuizRankings, prof3, 3)
	if out := s3.Answer(correctInput(t, s3.Current())); out.LeveledUp {
		t.Fatal("leveled past MaxLevel")
	}
}

// TestAnswersRecordBothTags: every answer feeds the item's fine-grained tag
// (weakness weighting) and the kind's gate tag (level progression).
func TestAnswersRecordBothTags(t *testing.T) {
	prof := profile.NewProfile()
	s := NewSessionSeeded(QuizOuts, prof, 5)
	it := s.Current()
	s.Answer(correctInput(t, it))
	if prof.DrillStats[it.SkillTag].Attempts != 1 {
		t.Errorf("fine tag %q not recorded", it.SkillTag)
	}
	if prof.DrillStats[kindTag(QuizOuts)].Attempts != 1 {
		t.Error("gate tag not recorded")
	}
}

// TestWeightingBendsTowardWeakness: with one weak and one strong tag in the
// pool, the weak tag dominates selection — but the strong one never fully
// disappears (the floor).
func TestWeightingBendsTowardWeakness(t *testing.T) {
	prof := profile.NewProfile()
	prof.DrillStats[tagOutsFlush] = profile.SkillStat{EMA: 0.95, Attempts: 30}
	prof.DrillStats[tagOutsOESD] = profile.SkillStat{EMA: 0.10, Attempts: 30}

	rng := newRNG(1)
	counts := map[string]int{}
	const n = 2000
	for i := 0; i < n; i++ {
		counts[pickTag(rng, prof, []string{tagOutsFlush, tagOutsOESD})]++
	}
	weak, strong := counts[tagOutsOESD], counts[tagOutsFlush]
	if weak <= strong*5 {
		t.Errorf("weak tag picked %d times vs %d — weighting is not bending toward weakness", weak, strong)
	}
	if strong == 0 {
		t.Error("mastered tag vanished from rotation — the floor is supposed to prevent this")
	}
}

// TestSpotGradingBands: GradeBest and GradeGood both count as correct; a
// clear blunder does not. The advice is synthetic so the EV gaps are exact.
func TestSpotGradingBands(t *testing.T) {
	seat := engine.Seat(0)
	fold := engine.Fold{S: seat}
	call := engine.Call{S: seat}
	raise := engine.Raise{S: seat, To: 60}
	adv := coach.Advice{Decision: ai.Decision{
		Action: raise,
		Candidates: []ai.ScoredAction{
			{Action: fold, ScoreBB: -2},
			{Action: call, ScoreBB: 1.9},
			{Action: raise, ScoreBB: 2.0},
		},
	}}
	item := Item{
		Drill: tutorial.Drill{Answer: tutorial.ChoiceAnswer{
			Choices: []string{"Fold", "Call", "Raise to 60"}, Correct: 2,
		}},
		SkillTag: tagSpotFlop,
		Spot:     &Spot{Advice: adv, Options: []engine.Action{fold, call, raise}},
	}
	session := func() *Session {
		return &Session{
			Kind:   QuizSpots,
			Level:  MaxLevel,
			Items:  []Item{item},
			grader: coach.New(nil, 1),
		}
	}

	if out := session().Answer("3"); !out.Correct || out.Grade != coach.GradeBest {
		t.Errorf("the coach's own pick graded %v correct=%v, want Best/correct", out.Grade, out.Correct)
	}
	// Calling is 0.1bb behind the best line: GradeGood, and still correct —
	// close spots must never be marked wrong on a coin flip.
	if out := session().Answer("2"); !out.Correct || out.Grade != coach.GradeGood {
		t.Errorf("a near-equal line graded %v correct=%v, want Good/correct", out.Grade, out.Correct)
	}
	// Folding gives up 4bb: a blunder, and wrong.
	if out := session().Answer("1"); out.Correct || out.Grade != coach.GradeBlunder {
		t.Errorf("a 4bb punt graded %v correct=%v, want Blunder/incorrect", out.Grade, out.Correct)
	}
}

// TestSessionCompletes: answering every item finishes the session and the
// accuracy statistic agrees with the tally.
func TestSessionCompletes(t *testing.T) {
	s := NewSessionSeeded(QuizRankings, profile.NewProfile(), 9)
	for !s.Done() {
		s.Answer(correctInput(t, s.Current()))
	}
	if s.Current() != nil {
		t.Error("Current must be nil once done")
	}
	if s.Answered != SessionItems || s.Accuracy() != 1 {
		t.Errorf("answered=%d accuracy=%v, want %d/1", s.Answered, s.Accuracy(), SessionItems)
	}
	if out := s.Answer("1"); out.Correct || out.Points != 0 {
		t.Error("answering a finished session must be a no-op")
	}
}

// TestChoiceIndex: numbers and case-insensitive text resolve; junk does not.
func TestChoiceIndex(t *testing.T) {
	choices := []string{"Fold", "Call", "Raise to 60"}
	cases := []struct {
		in   string
		want int
	}{
		{"1", 0}, {"3", 2}, {"fold", 0}, {"RAISE TO 60", 2},
		{"0", -1}, {"4", -1}, {"nonsense", -1},
	}
	for _, tc := range cases {
		if got := choiceIndex(tc.in, choices); got != tc.want {
			t.Errorf("choiceIndex(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
