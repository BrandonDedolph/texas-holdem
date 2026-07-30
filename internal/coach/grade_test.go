package coach

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// TestAntiResulting is the acceptance test for the whole package (DESIGN.md
// §1 principle 1, CLAUDE.md "load-bearing tests"): a correct call graded at
// decision time must be byte-for-byte identical whether the runout wins or
// loses the pot. If the outcome can influence the grade in any way — the
// grade, the EV loss, a single character of the explanation — the package
// is wrong.
//
// The spot: 9s8s (flush draw) on Ks7s2h facing 25 into 75. Required equity
// 25%, flush-draw equity ~35%: the call is correct on pot odds. World A
// rivers the flush (hero wins); world B bricks (hero loses).
func TestAntiResulting(t *testing.T) {
	build := func(river string) *engine.Hand {
		return flushDrawFacingBet(t, "Ks 7s 2h 3d "+river, "Kd Qd", 25, 91)
	}
	winning := build("As") // spade river: hero makes the flush
	losing := build("Ah")  // brick: villain's top pair holds

	vWin, vLose := heroView(t, winning, 0), heroView(t, losing, 0)

	// The two decision-time information sets must be identical — the only
	// difference between the worlds is cards the hero cannot see yet.
	if !reflect.DeepEqual(vWin, vLose) {
		t.Fatal("decision-time views differ; the spot is mis-built and the test proves nothing")
	}

	coachWin, coachLose := New(profile.NewProfile(), 42), New(profile.NewProfile(), 42)
	advWin, advLose := coachWin.Advise(vWin), coachLose.Advise(vLose)

	call := engine.Call{S: 0}
	gradeWin := coachWin.GradeAction(advWin, call)
	gradeLose := coachLose.GradeAction(advLose, call)

	// Now let both worlds happen. The grades above are already frozen —
	// nothing after this point may touch them.
	mustApply(t, winning, call)
	mustApply(t, losing, call)
	resWin, resLose := checkDown(t, winning), checkDown(t, losing)
	if resWin.Net[0] <= 0 {
		t.Fatalf("world A was supposed to win the pot, net %v", resWin.Net[0])
	}
	if resLose.Net[0] >= 0 {
		t.Fatalf("world B was supposed to lose the pot, net %v", resLose.Net[0])
	}

	// The heart of the app: same grade, same EV loss, same explanation,
	// byte for byte.
	if !reflect.DeepEqual(gradeWin, gradeLose) {
		t.Fatalf("outcome leaked into the grade:\nwin:  %+v\nlose: %+v", gradeWin, gradeLose)
	}
	if got, want := fmt.Sprintf("%#v", gradeWin), fmt.Sprintf("%#v", gradeLose); got != want {
		t.Fatalf("graded decisions not byte-identical:\n%s\nvs\n%s", got, want)
	}
	if gradeWin.Body != gradeLose.Body {
		t.Fatalf("explanation text differs between outcomes: %q vs %q", gradeWin.Body, gradeLose.Body)
	}

	// And the call itself — correct on pot odds — must grade well in both.
	if !gradeWin.Grade.GoodOrBetter() {
		t.Errorf("a pot-odds-correct call graded %v (EV loss %.2fbb): %s",
			gradeWin.Grade, gradeWin.EVLossBB, gradeWin.Body)
	}

	// The advice the two coaches rendered must match too — the frozen
	// audit trail includes the words the hero read.
	if !reflect.DeepEqual(advWin, advLose) {
		t.Error("advice differs between identical decision-time views")
	}
}

// TestFoldingNutsIsBlunder: the hero holds the royal flush on the river and
// folds to a pot-sized bet. No information set makes that defensible, and
// the EV-loss band must say so.
func TestFoldingNutsIsBlunder(t *testing.T) {
	h := huHand(t, "As Ts", "Kd Qd", "Ks Qs Js 9h 2d", 1000, 17,
		engine.Raise{S: 0, To: 25},
		engine.Call{S: 1},
		engine.Check{S: 1}, engine.Check{S: 0}, // flop
		engine.Check{S: 1}, engine.Check{S: 0}, // turn
		engine.Bet{S: 1, Amount: 50}, // river: villain bets half pot
	)
	v := heroView(t, h, 0)
	c := New(nil, 7)
	adv := c.Advise(v)

	g := c.GradeAction(adv, engine.Fold{S: 0})
	if g.Grade != GradeBlunder {
		t.Errorf("folding the nuts graded %v (EV loss %.2fbb), want blunder", g.Grade, g.EVLossBB)
	}
	if g.EVLossBB < evBlunderBB {
		t.Errorf("EV loss %.2fbb below the blunder band", g.EVLossBB)
	}
	if g.Body == "" {
		t.Error("blunder carries no explanation")
	}
}

// TestMatchedActionIsBest: taking exactly the coach's action — or its size
// within 25% — is GradeBest by definition. Following the coach can never
// be graded down.
func TestMatchedActionIsBest(t *testing.T) {
	adv := syntheticAdvice(engine.Raise{S: 0, To: 60},
		sa(engine.Fold{S: 0}, 0),
		sa(engine.Call{S: 0}, 1.9),
		sa(engine.Raise{S: 0, To: 60}, 2.0),
	)
	c := New(nil, 1)

	if g := c.GradeAction(adv, engine.Raise{S: 0, To: 60}); g.Grade != GradeBest {
		t.Errorf("exact match graded %v", g.Grade)
	}
	// 70 is within ±25% of 60: same decision, human sizing wobble.
	if g := c.GradeAction(adv, engine.Raise{S: 0, To: 70}); g.Grade != GradeBest {
		t.Errorf("size within tolerance graded %v", g.Grade)
	}
}

// TestNearEqualLinesBothGradeWell: when calling and raising score within
// 0.25bb, taking the non-recommended one is GradeGood, not a mistake — and
// the explanation says so in words. Poker has too many close spots for a
// binary right/wrong; this is the graded scale doing its job.
func TestNearEqualLinesBothGradeWell(t *testing.T) {
	adv := syntheticAdvice(engine.Raise{S: 0, To: 60},
		sa(engine.Fold{S: 0}, 0),
		sa(engine.Call{S: 0}, 1.9),
		sa(engine.Raise{S: 0, To: 60}, 2.0),
	)
	c := New(nil, 1)

	g := c.GradeAction(adv, engine.Call{S: 0})
	if g.Grade != GradeGood {
		t.Fatalf("near-equal alternative graded %v (EV loss %.2f)", g.Grade, g.EVLossBB)
	}
	want := "Raise to 60 was my pick, but calling is fine here too."
	if g.Body != want {
		t.Errorf("body %q, want %q", g.Body, want)
	}
}

// TestGradeBands pins the EV-loss boundaries: 0.25 / 1 / 3 big blinds.
func TestGradeBands(t *testing.T) {
	c := New(nil, 1)
	cases := []struct {
		bestScore float64
		want      Grade
	}{
		{0.24, GradeGood},
		{0.25, GradeInaccuracy},
		{0.99, GradeInaccuracy},
		{1.0, GradeMistake},
		{2.99, GradeMistake},
		{3.0, GradeBlunder},
		{7.5, GradeBlunder},
	}
	for _, tc := range cases {
		// Coach picks the call; the hero folds (score 0), losing exactly
		// bestScore big blinds.
		adv := syntheticAdvice(engine.Call{S: 0},
			sa(engine.Fold{S: 0}, 0),
			sa(engine.Call{S: 0}, tc.bestScore),
		)
		g := c.GradeAction(adv, engine.Fold{S: 0})
		if g.Grade != tc.want {
			t.Errorf("EV loss %.2f graded %v, want %v", tc.bestScore, g.Grade, tc.want)
		}
		if g.EVLossBB != tc.bestScore {
			t.Errorf("EV loss recorded %.2f, want %.2f", g.EVLossBB, tc.bestScore)
		}
	}
}

// TestGradeRecordedInOffMode: verbosity is a display choice, not an
// amnesty. With the coach off, GradeAction still records the lifetime
// grade count and still returns the full graded decision for the review.
func TestGradeRecordedInOffMode(t *testing.T) {
	prof := profile.NewProfile()
	prof.CoachMode = profile.CoachOff
	c := New(prof, 3)

	h := flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5)
	adv := c.Advise(heroView(t, h, 0))
	g := c.GradeAction(adv, engine.Call{S: 0})

	if got := prof.GradeTotals[g.Grade.String()]; got != 1 {
		t.Errorf("grade %v not recorded with coach off: totals %v", g.Grade, prof.GradeTotals)
	}
	if g.Feedback(profile.CoachOff) != "" {
		t.Error("coach off must stay silent even though the grade was recorded")
	}
}

// TestFeedbackModes: Full always speaks; Mistakes withholds opinion on good
// moves and speaks on leaks; Off never speaks.
func TestFeedbackModes(t *testing.T) {
	good := GradedDecision{Grade: GradeBest, Body: "Call was my pick too."}
	bad := GradedDecision{Grade: GradeMistake, Body: "A mistake — folding gives up about 2.0bb; Call was the play."}

	if good.Feedback(profile.CoachFull) == "" || bad.Feedback(profile.CoachFull) == "" {
		t.Error("full mode must always show the grade line")
	}
	if got := good.Feedback(profile.CoachMistakes); got != "" {
		t.Errorf("mistakes mode showed opinion on a good move: %q", got)
	}
	if bad.Feedback(profile.CoachMistakes) == "" {
		t.Error("mistakes mode stayed silent on a mistake")
	}
	if good.Feedback(profile.CoachOff) != "" || bad.Feedback(profile.CoachOff) != "" {
		t.Error("off mode spoke")
	}
}

// TestViewDigestIsHeroKnowledgeOnly: the digest — all the grader ever keeps
// of the spot — contains exactly what the hero knew, and it is captured at
// advise time, not at grade time.
func TestViewDigestIsHeroKnowledgeOnly(t *testing.T) {
	h := flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5)
	v := heroView(t, h, 0)
	c := New(nil, 3)
	adv := c.Advise(v)

	d := adv.Digest
	if d.Street != engine.Flop || d.Hole != "9s 8s" || d.Board != "Ks 7s 2h" {
		t.Errorf("digest mis-snapshotted the spot: %+v", d)
	}
	if d.ToCall != 25 || d.Pot != 75 {
		t.Errorf("digest pot/toCall wrong: %+v", d)
	}
	g := c.GradeAction(adv, engine.Call{S: 0})
	if g.ViewDigest != d {
		t.Error("grade did not carry the frozen digest verbatim")
	}
}

// sa builds a scored candidate.
func sa(a engine.Action, score float64) ai.ScoredAction {
	return ai.ScoredAction{Action: a, ScoreBB: score}
}

// syntheticAdvice hand-builds an Advice around a candidate slate, for
// grading tests that need exact scores rather than engine-derived ones.
func syntheticAdvice(pick engine.Action, cands ...ai.ScoredAction) Advice {
	return Advice{
		Decision: ai.Decision{Action: pick, Candidates: cands},
		Digest:   ViewDigest{Street: engine.Flop},
	}
}

// TestFollowingTheCoachNeverCostsEV pins the invariant that the grade band
// and the EV ledger agree. GradeBest with a nonzero EVLossBB would have the
// app congratulate a player and bill them for the same decision, and that
// number feeds the review screen's "EV lost this hand" line.
func TestFollowingTheCoachNeverCostsEV(t *testing.T) {
	c := New(profile.NewProfile(), 42)
	spots := []*engine.PlayerView{
		heroView(t, flushDrawFacingBet(t, "As 7s 2h", "Kd Qc", 25, 3), 0),
		heroView(t, bustedComboRiverSpot(t, 5), 0),
		heroView(t, huHand(t, "Ah Js", "Kd Qc", "", 1000, 11), 0),
		heroView(t, huHand(t, "7h 7c", "Kd Qc", "", 1000, 12), 0),
	}
	for i, v := range spots {
		adv := c.Advise(v)
		g := c.GradeAction(adv, adv.Decision.Action)
		if g.Grade != GradeBest {
			t.Errorf("spot %d: taking the coach's own pick graded %v, want GradeBest", i, g.Grade)
		}
		if g.EVLossBB != 0 {
			t.Errorf("spot %d: taking the coach's own pick cost %.3fbb, want 0", i, g.EVLossBB)
		}
	}
}
