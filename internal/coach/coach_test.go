package coach

import (
	"reflect"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/ai/rulebased"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// TestCoachRunsTheBotStrategy is principle 2 (one source of strategic
// truth) at the package boundary: the coach's Decision is byte-identical
// to what the rule-based strategy under the "coach" archetype produces for
// the same view and seed. There is no second brain to disagree with the
// bots.
func TestCoachRunsTheBotStrategy(t *testing.T) {
	build := func() *engine.PlayerView {
		return heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0)
	}
	adv := New(nil, 99).Advise(build())
	want := rulebased.NewStrategy(ai.Archetypes["coach"], 99).Decide(build())
	if !reflect.DeepEqual(adv.Decision, want) {
		t.Errorf("coach decision diverges from the bot strategy:\ncoach: %+v\nbot:   %+v",
			adv.Decision.Action, want.Action)
	}
}

// TestAdviseDeterminism: same view, same seed → the identical Advice,
// text included, across independently built coaches and independently
// built (identical) hands. Replays and the frozen audit trail rest on it.
func TestAdviseDeterminism(t *testing.T) {
	spots := []func() *engine.PlayerView{
		func() *engine.PlayerView {
			return heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 61), 0)
		},
		func() *engine.PlayerView {
			return heroView(t, bustedComboRiverSpot(t, 62), 0)
		},
		func() *engine.PlayerView {
			h := sixHand(t, map[engine.Seat][2]engine.Card{0: engine.Holes("Ad Jd")}, 63,
				engine.Fold{S: 3}, engine.Fold{S: 4}, engine.Fold{S: 5})
			return heroView(t, h, 0)
		},
	}
	for i, build := range spots {
		a := New(profile.NewProfile(), 42).Advise(build())
		b := New(profile.NewProfile(), 42).Advise(build())
		if !reflect.DeepEqual(a, b) {
			t.Errorf("spot %d: advice differs across identical coaches:\n%q\nvs\n%q", i, a.Body, b.Body)
		}
	}
}

// TestAdviceForMode pins the verbosity contract (DESIGN.md §2.9): Full
// shows everything; Mistakes shows numbers but withholds the opinion; Off
// shows nothing. Decision and Digest survive every mode, because grading
// happens — and is recorded — regardless of what was displayed.
func TestAdviceForMode(t *testing.T) {
	adv := New(nil, 3).Advise(heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0))
	if adv.Headline == "" || adv.Body == "" || len(adv.Numbers) == 0 {
		t.Fatal("spot produced empty advice; test is mis-built")
	}

	full := adv.ForMode(profile.CoachFull)
	if !reflect.DeepEqual(full, adv) {
		t.Error("full mode altered the advice")
	}

	mist := adv.ForMode(profile.CoachMistakes)
	if mist.Headline != "" || mist.Body != "" {
		t.Errorf("mistakes mode leaked the opinion: %q / %q", mist.Headline, mist.Body)
	}
	if !reflect.DeepEqual(mist.Numbers, adv.Numbers) {
		t.Error("mistakes mode must keep the numbers — pot odds are the scoreboard, not coaching")
	}

	off := adv.ForMode(profile.CoachOff)
	if off.Headline != "" || off.Body != "" || off.Numbers != nil {
		t.Errorf("off mode showed something: %+v", off)
	}

	for name, a := range map[string]Advice{"mistakes": mist, "off": off} {
		if !reflect.DeepEqual(a.Decision, adv.Decision) || a.Digest != adv.Digest {
			t.Errorf("%s mode dropped grading state", name)
		}
	}
}

// TestMode: the coach reads verbosity from the profile, defaulting to full.
func TestMode(t *testing.T) {
	if got := New(nil, 1).Mode(); got != profile.CoachFull {
		t.Errorf("no-profile mode %q", got)
	}
	prof := profile.NewProfile()
	prof.CoachMode = profile.CoachMistakes
	if got := New(prof, 1).Mode(); got != profile.CoachMistakes {
		t.Errorf("mode %q, want mistakes", got)
	}
}

// TestAdviceIsFrozenValue: advising twice from the same view yields equal
// values, and grading consumes the advice value without mutating it —
// the "captured at the moment the turn begins and frozen" contract.
func TestAdviceIsFrozenValue(t *testing.T) {
	c := New(nil, 9)
	v := heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0)
	adv := c.Advise(v)
	before := adv.Body

	_ = c.GradeAction(adv, engine.Call{S: 0})
	_ = c.GradeAction(adv, engine.Fold{S: 0})

	if adv.Body != before {
		t.Error("grading mutated the frozen advice")
	}
	if again := c.Advise(v); !reflect.DeepEqual(again, adv) {
		t.Error("re-advising the same view produced different advice")
	}
}
