package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Preflop sizing presets (docs/ui-review.md F5): opening preflop, the bar
// speaks big blinds — 2bb / 2.5bb / 3bb / 4bb / all-in — with resolved chip
// amounts, and the coach's recommended open is a preset (or one 1bb nudge)
// away. Postflop the presets stay pot fractions. These tests drive the real
// coach over real engine hands so the "coach's size is reachable" claim is
// checked against what the coach actually says, not a hardcoded 25.

// preflopSpot is a real engine hand paused on the hero's preflop decision.
type preflopSpot struct {
	name string
	hero engine.Seat
	bb   engine.Chips
	hand *engine.Hand
}

// buildPreflopSpot deals a 6-max hand (button seat 0: SB=1, BB=2, UTG=3,
// HJ=4, CO=5, BTN=0) with the hero's holes fixed, applies the scripted
// actions, and checks the hero is the one to act.
func buildPreflopSpot(t *testing.T, name string, hero engine.Seat, sb, bbChips, stack engine.Chips, holes string, actions ...engine.Action) preflopSpot {
	t.Helper()
	stacks := make(map[engine.Seat]engine.Chips, 6)
	for s := engine.Seat(0); s < 6; s++ {
		stacks[s] = stack
	}
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: sb, BigBlind: bbChips, Seats: 6},
		Button: 0,
		Stacks: stacks,
		Holes:  map[engine.Seat][2]engine.Card{hero: engine.Holes(holes)},
		Seed:   7,
		Eval:   eval.Standard{},
	})
	if err != nil {
		t.Fatalf("%s: setup: %v", name, err)
	}
	for _, a := range actions {
		if err := h.Apply(a); err != nil {
			t.Fatalf("%s: apply %v seat %d: %v", name, a.Type(), a.Seat(), err)
		}
	}
	if h.CurrentSeat() != hero {
		t.Fatalf("%s: expected hero seat %d to act, current %d", name, hero, h.CurrentSeat())
	}
	return preflopSpot{name: name, hero: hero, bb: bbChips, hand: h}
}

// armFromHand arms a bar exactly the way TableScreen.armBar does: every
// number straight off the engine.
func armFromHand(spot preflopSpot) *ActionBar {
	b := NewActionBar()
	b.Arm(spot.hand.LegalActions(), spot.hand.Street(), spot.hand.PotTotal(),
		spot.hand.ToCall(spot.hero), spot.hand.Committed(spot.hero),
		spot.bb, "")
	return b
}

func TestPreflopCoachOpenIsPresetOrOneNudgeAway(t *testing.T) {
	fold := func(s engine.Seat) engine.Action { return engine.Fold{S: s} }
	limp := func(s engine.Seat) engine.Action { return engine.Call{S: s} }

	spots := []preflopSpot{
		buildPreflopSpot(t, "UTG first-in, QQ", 3, 5, 10, 1000, "Qs Qd"),
		buildPreflopSpot(t, "CO first-in, AKs", 5, 5, 10, 1000, "As Ks",
			fold(3), fold(4)),
		buildPreflopSpot(t, "BTN first-in, AQs", 0, 5, 10, 1000, "As Qs",
			fold(3), fold(4), fold(5)),
		buildPreflopSpot(t, "SB first-in, KQs", 1, 5, 10, 1000, "Ks Qs",
			fold(3), fold(4), fold(5), fold(0)),
		buildPreflopSpot(t, "BB iso vs one limper, AKo", 2, 5, 10, 1000, "As Kd",
			limp(3), fold(4), fold(5), fold(0), fold(1)),
		buildPreflopSpot(t, "CO first-in at 25/50, AKs", 5, 25, 50, 5000, "As Ks",
			fold(3), fold(4)),
	}

	for _, spot := range spots {
		c := coach.New(nil, 1)
		adv := c.Advise(spot.hand.View(spot.hero))
		raise, ok := adv.Decision.Action.(engine.Raise)
		if !ok {
			t.Fatalf("%s: coach recommends %v, test needs a raise spot",
				spot.name, adv.Decision.Action.Type())
		}
		want := raise.To

		b := armFromHand(spot)
		if _, skipped := b.OpenSizing(engine.ActionRaise); skipped {
			t.Fatalf("%s: sizing skipped with a deep stack", spot.name)
		}
		if !b.openingPreflop() {
			t.Fatalf("%s: bar did not detect a preflop open", spot.name)
		}

		amts := b.presetAmounts()
		reachable := false
		for _, p := range amts {
			if p == want || b.clamp(p+spot.bb) == want || b.clamp(p-spot.bb) == want {
				reachable = true
				break
			}
		}
		if !reachable {
			t.Errorf("%s: coach says raise to %v; presets %v (nudge %v) cannot reach it",
				spot.name, want, amts, spot.bb)
		}
		t.Logf("%s: coach raises to %v; presets %v, nudge %v", spot.name, want, amts, spot.bb)
	}
}

func TestPreflopPresetLabelsAreBigBlindsWithChips(t *testing.T) {
	spot := buildPreflopSpot(t, "CO first-in", 5, 5, 10, 1000, "As Ks",
		engine.Fold{S: 3}, engine.Fold{S: 4})
	b := armFromHand(spot)
	b.OpenSizing(engine.ActionRaise)

	// The ladder itself: standard opens, engine-clamped.
	want := [5]engine.Chips{20, 25, 30, 40, 1000}
	if got := b.presetAmounts(); got != want {
		t.Fatalf("preflop presets = %v, want %v", got, want)
	}
	// Sizing opens ON the standard 2.5bb open, not a pot fraction.
	if got := b.ConfirmAmount(); got != 25 {
		t.Fatalf("initial preflop sizing amount = %v, want the 2.5bb open 25", got)
	}

	view := stripANSI(b.View(80))
	for _, s := range []string{"1 2bb 20", "2 2.5bb 25", "3 3bb 30", "4 4bb 40", "5 all-in"} {
		if !strings.Contains(view, s) {
			t.Errorf("preflop preset row must show %q (bb and chips):\n%s", s, view)
		}
	}
	for _, banned := range []string{"1/3", "1/2", "2/3", "pot "} {
		if strings.Contains(view, banned) {
			t.Errorf("preflop preset row must not speak pot fractions (%q):\n%s", banned, view)
		}
	}

	// One nudge from the 2.5bb rung lands on 3.5bb — the coach's one-limper
	// iso size — so the 1bb step walks the half-bb ladder the coach uses.
	b.Preset(1)
	b.Nudge(+1)
	if got := b.ConfirmAmount(); got != 35 {
		t.Errorf("2.5bb preset + one nudge = %v, want 35 (3.5bb)", got)
	}

	// Typed over-max still shows and applies the engine clamp.
	for _, d := range []byte("99999") {
		b.TypeDigit(d)
	}
	if got := b.ConfirmAmount(); got != 1000 {
		t.Errorf("typed over-max = %v, want engine clamp 1000", got)
	}
	if view := stripANSI(b.View(80)); !strings.Contains(view, "all-in 1,000") {
		t.Errorf("over-max view must show the clamp it will receive:\n%s", view)
	}
}

func TestPostflopPresetLabelsStayPotFractions(t *testing.T) {
	// Betting: unchanged fraction ladder.
	b := NewActionBar()
	b.Arm(engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 10, Max: 900},
	}, engine.Flop, 185, 0, 0, 10, "")
	b.OpenSizing(engine.ActionBet)
	view := stripANSI(b.View(80))
	for _, s := range []string{"1 1/3 62", "2 1/2 93", "3 2/3 123", "4 pot 185"} {
		if !strings.Contains(view, s) {
			t.Errorf("postflop bet presets must stay pot fractions, missing %q:\n%s", s, view)
		}
	}
	if strings.Contains(view, "bb") {
		t.Errorf("postflop bet presets must not speak big blinds:\n%s", view)
	}

	// Raising over a real bet POSTFLOP: fractions, the postflop vocabulary.
	b = NewActionBar()
	b.Arm(engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 60, Max: 60},
		{Type: engine.ActionRaise, Min: 200, Max: 1000},
	}, engine.Flop, 300, 60, 40, 10, "")
	b.OpenSizing(engine.ActionRaise)
	if b.openingPreflop() {
		t.Fatal("a raise over a 100-chip bet is not a preflop open")
	}
	view = stripANSI(b.View(80))
	for _, s := range []string{"1 1/3 220", "2 1/2 280", "3 2/3 340", "4 pot 460"} {
		if !strings.Contains(view, s) {
			t.Errorf("raise-over-bet presets must stay pot fractions, missing %q:\n%s", s, view)
		}
	}
}

func TestPreflopShortStackStillSkipsSizing(t *testing.T) {
	// A 15-chip stack facing the 10 blind: the only legal raise is all-in,
	// so sizing mode is skipped even though this is a preflop open spot.
	b := NewActionBar()
	b.Arm(engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 10, Max: 10},
		{Type: engine.ActionRaise, Min: 15, Max: 15},
	}, engine.Preflop, 15, 10, 0, 10, "")
	amt, skipped := b.OpenSizing(engine.ActionRaise)
	if !skipped || amt != 15 {
		t.Fatalf("pinned preflop raise: got (%v, %v), want (15, skipped)", amt, skipped)
	}
	if b.State() != ActionBarChoosing {
		t.Fatalf("skipped sizing must not change state, got %v", b.State())
	}
}

// TestPreflop3BetLadderReachesTheCoach: facing an open, the coach's 3-bet is
// a multiple of that open, not of the blind. Pot fractions cannot express
// that, so before the street reached the bar this spot offered sizes the
// coach would never name.
func TestPreflop3BetLadderReachesTheCoach(t *testing.T) {
	b := NewActionBar()
	// Hero on the button facing a 3bb open: current bet level 30, blind 10.
	b.Arm(engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 30, Max: 30},
		{Type: engine.ActionRaise, Min: 50, Max: 1000},
	}, engine.Preflop, 45, 30, 0, 10, "")
	b.OpenSizing(engine.ActionRaise)

	if b.openingPreflop() {
		t.Fatal("facing an open is not an opening spot")
	}
	got := b.presetAmounts()
	// 2.5x / 3x / 3.5x / 4x of a 30 open.
	want := []engine.Chips{75, 90, 105, 120}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("preset %d = %v, want %v (a multiple of the 30 open); got %v",
				i+1, got[i], w, got)
		}
	}

	// The standard 3-bet (3x the open) must be one keypress away.
	view := stripANSI(b.View(80))
	if !strings.Contains(view, "90") {
		t.Errorf("the standard 3x 3-bet is not on the ladder:\n%s", view)
	}
}

// TestPostflopRaiseKeepsFractions: the street is what distinguishes the two
// vocabularies, and postflop must be unaffected by the preflop ladder.
func TestPostflopRaiseKeepsFractions(t *testing.T) {
	b := NewActionBar()
	b.Arm(engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 30, Max: 30},
		{Type: engine.ActionRaise, Min: 50, Max: 1000},
	}, engine.Flop, 45, 30, 0, 10, "")
	b.OpenSizing(engine.ActionRaise)
	if b.preflopRaise() {
		t.Fatal("a flop raise must not use the preflop ladder")
	}
	if got := b.presetAmounts(); got[1] == 90 {
		t.Errorf("flop presets look like the 3-bet ladder: %v", got)
	}
}
