package profile

import (
	"math"
	"testing"
)

const eps = 1e-9

func almostEqual(a, b float64) bool { return math.Abs(a-b) < eps }

func TestSkillStatRecordSeedsFirstAttempt(t *testing.T) {
	// The first answer sets the EMA directly instead of blending with a
	// zero prior.
	got := SkillStat{}.Record(true)
	if !almostEqual(got.EMA, 1.0) || got.Attempts != 1 {
		t.Fatalf("Record(true) on fresh stat = %+v, want EMA 1.0, Attempts 1", got)
	}
	got = SkillStat{}.Record(false)
	if !almostEqual(got.EMA, 0.0) || got.Attempts != 1 {
		t.Fatalf("Record(false) on fresh stat = %+v, want EMA 0.0, Attempts 1", got)
	}
}

func TestSkillStatEMAMath(t *testing.T) {
	// Fold in a fixed sequence and check every intermediate value against
	// the closed-form EMA with alpha = 0.3.
	answers := []bool{true, true, false, true, false, false, true}
	want := []float64{
		1.0,                  // seeded
		0.7*1.0 + 0.3*1.0,    // 1.0
		0.7*1.0 + 0.3*0.0,    // 0.7
		0.7*0.7 + 0.3*1.0,    // 0.79
		0.7*0.79 + 0.3*0.0,   // 0.553
		0.7*0.553 + 0.3*0.0,  // 0.3871
		0.7*0.3871 + 0.3*1.0, // 0.57097
	}

	var s SkillStat
	for i, correct := range answers {
		s = s.Record(correct)
		if !almostEqual(s.EMA, want[i]) {
			t.Fatalf("after answer %d: EMA = %v, want %v", i+1, s.EMA, want[i])
		}
		if s.Attempts != i+1 {
			t.Fatalf("after answer %d: Attempts = %d, want %d", i+1, s.Attempts, i+1)
		}
	}
}

func TestSkillStatRecordDoesNotMutateReceiver(t *testing.T) {
	s := SkillStat{EMA: 0.5, Attempts: 5}
	_ = s.Record(true)
	if s.EMA != 0.5 || s.Attempts != 5 {
		t.Fatalf("Record mutated its value receiver: %+v", s)
	}
}

func TestUnlockReadyBoundaries(t *testing.T) {
	cases := []struct {
		name string
		stat SkillStat
		want bool
	}{
		{"exactly 80% over exactly 20", SkillStat{EMA: 0.8, Attempts: 20}, true},
		{"above both thresholds", SkillStat{EMA: 0.95, Attempts: 40}, true},
		{"80% but only 19 attempts", SkillStat{EMA: 0.8, Attempts: 19}, false},
		{"20 attempts but just under 80%", SkillStat{EMA: 0.7999, Attempts: 20}, false},
		{"fresh stat", SkillStat{}, false},
		{"perfect but too few", SkillStat{EMA: 1.0, Attempts: 1}, false},
	}
	for _, tc := range cases {
		if got := tc.stat.UnlockReady(); got != tc.want {
			t.Errorf("%s: UnlockReady() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestUnlockReachableThroughRecord(t *testing.T) {
	// A realistic path: 20 straight correct answers must unlock (EMA is
	// exactly 1.0 the whole way).
	var s SkillStat
	for i := 0; i < 19; i++ {
		s = s.Record(true)
		if s.UnlockReady() {
			t.Fatalf("unlocked after only %d attempts", i+1)
		}
	}
	s = s.Record(true)
	if !s.UnlockReady() {
		t.Fatalf("20 straight correct answers should unlock; got %+v", s)
	}
}

func TestRecordDrillOnZeroValueProfile(t *testing.T) {
	// The zero-value Profile has nil maps; the drill helpers must still work
	// (Load guards this too, but the methods should not depend on it).
	var p Profile
	got := p.RecordDrill("outs.flushdraw", true)
	if got.Attempts != 1 || !almostEqual(got.EMA, 1.0) {
		t.Fatalf("RecordDrill = %+v, want 1 attempt at EMA 1.0", got)
	}
	if stored := p.DrillStats["outs.flushdraw"]; stored != got {
		t.Fatalf("stored stat %+v differs from returned %+v", stored, got)
	}
}

func TestAdvanceSkill(t *testing.T) {
	p := NewProfile()
	tag := "rank.twopair-vs-trips"

	// Not ready: nothing changes.
	p.DrillStats[tag] = SkillStat{EMA: 0.75, Attempts: 30, Level: 1}
	if p.AdvanceSkill(tag) {
		t.Fatal("AdvanceSkill promoted a skill below 80% EMA")
	}
	if got := p.DrillStats[tag]; got.Level != 1 || got.Attempts != 30 {
		t.Fatalf("failed advance mutated the stat: %+v", got)
	}

	// Ready: level up, and the new level starts from a clean slate — an
	// average earned at an easier level must not count toward the next one.
	p.DrillStats[tag] = SkillStat{EMA: 0.8, Attempts: 20, Level: 1}
	if !p.AdvanceSkill(tag) {
		t.Fatal("AdvanceSkill refused a skill at exactly 80% over exactly 20")
	}
	if got, want := p.DrillStats[tag], (SkillStat{Level: 2}); got != want {
		t.Fatalf("after advance: %+v, want %+v", got, want)
	}
}

func TestMomentsSeenKeepsFirstTimestamp(t *testing.T) {
	p := NewProfile()
	p.MarkMomentSeen("first_flush_draw")
	first := p.MomentsSeen["first_flush_draw"]
	if first.IsZero() {
		t.Fatal("MarkMomentSeen recorded a zero time")
	}
	p.MarkMomentSeen("first_flush_draw")
	if got := p.MomentsSeen["first_flush_draw"]; !got.Equal(first) {
		t.Fatalf("second MarkMomentSeen changed the timestamp: %v -> %v", first, got)
	}
}

func TestRecordGradeAndCompleteLesson(t *testing.T) {
	p := NewProfile()
	p.RecordGrade("good")
	p.RecordGrade("good")
	p.RecordGrade("blunder")
	if p.GradeTotals["good"] != 2 || p.GradeTotals["blunder"] != 1 {
		t.Fatalf("GradeTotals = %v", p.GradeTotals)
	}

	p.CompleteLesson("pot-odds")
	if p.LessonsDone["pot-odds"].IsZero() {
		t.Fatal("CompleteLesson did not record a time")
	}
}

func TestNewProfileDefaults(t *testing.T) {
	p := NewProfile()
	if p.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", p.Version, CurrentVersion)
	}
	if p.CoachMode != CoachFull {
		t.Errorf("CoachMode = %q, want %q", p.CoachMode, CoachFull)
	}
	if p.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if p.Bankroll != DefaultBankroll {
		t.Errorf("Bankroll = %d, want %d", p.Bankroll, DefaultBankroll)
	}
	if len(p.TableDefaults.Lineup) != 5 {
		t.Errorf("default lineup = %v, want the five-archetype classroom mix", p.TableDefaults.Lineup)
	}
	// Every map is allocated so callers can index without nil checks.
	p.LessonsDone["x"] = p.CreatedAt
	p.MomentsSeen["x"] = p.CreatedAt
	p.DrillStats["x"] = SkillStat{}
	p.GradeTotals["x"]++
}
