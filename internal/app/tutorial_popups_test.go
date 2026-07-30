package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// Teachable-moment wiring tests (ui-review F10): the coach package's
// PendingMoment machinery was fully built and tested — these pin the last
// wire, from the table's decision loop to a dismissible popup.
//
// The fixture: hand #1 at seed 17 arms the hero on the button with 77, and
// the coach's rationale carries the "BTN open" chart fact — the
// first_button_raise_hand moment's exact trigger.

const buttonMomentID = "first_button_raise_hand"

// freshMomentScenario is scenarioPreflop with a virgin profile, so moments
// are live rather than pre-seen.
func freshMomentScenario() (tableScenario, *profile.Profile) {
	prof := profile.NewProfile()
	sc := scenarioPreflop()
	sc.prof = prof
	return sc, prof
}

func TestMomentFiresOnHeroTurnAndPersists(t *testing.T) {
	sc, prof := freshMomentScenario()
	m := buildTable(t, sc, 80, 24)

	if m.overlay == nil {
		t.Fatal("a first-contact moment should fire when the hero's turn arms")
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "The button raise") {
		t.Errorf("popup must render the moment over the table:\n%s", view)
	}
	if !strings.Contains(view, "any key continues") {
		t.Error("popup must say how to dismiss itself")
	}
	if _, seen := prof.MomentsSeen[buttonMomentID]; !seen {
		t.Error("firing must persist to profile.MomentsSeen immediately")
	}
	// At most one moment per decision, made observable at the app level.
	if len(prof.MomentsSeen) != 1 {
		t.Errorf("one decision marked %d moments seen, want exactly 1", len(prof.MomentsSeen))
	}
}

// TestMomentDismissConsumesTheKey: the dismissing key must close the popup
// and do nothing else — an eaten "f" folding the hero would be a disaster.
func TestMomentDismissConsumesTheKey(t *testing.T) {
	sc, _ := freshMomentScenario()
	m := buildTable(t, sc, 80, 24)
	if m.overlay == nil {
		t.Fatal("setup: moment should be up")
	}

	events := len(m.hand.Events())
	m.Update(key("f")) // bound to fold in this state — must only dismiss
	if m.overlay != nil {
		t.Error("any key must dismiss the popup")
	}
	if len(m.hand.Events()) != events || !m.hand.Live().Has(heroSeat) {
		t.Error("the dismissing key leaked into the game")
	}
	if m.bar.State() != ActionBarChoosing {
		t.Error("the hero's turn must still be armed after dismissal")
	}
}

func TestMomentFiresOncePerProfileEver(t *testing.T) {
	sc, prof := freshMomentScenario()
	m := buildTable(t, sc, 80, 24)
	m.Update(key("z")) // dismiss

	// A new session over the same profile meets the same spot: silence.
	sc2 := scenarioPreflop()
	sc2.prof = prof
	m2 := buildTable(t, sc2, 80, 24)
	if m2.overlay != nil {
		t.Error("a seen moment must never fire again for this profile")
	}
}

// TestMomentNeverDuringVillainTurn steps the presentation queue one tick at
// a time after the hero acts and asserts the popup can only exist while the
// hero's own turn is armed.
func TestMomentNeverDuringVillainTurn(t *testing.T) {
	sc, _ := freshMomentScenario()
	sc.speed = SpeedNormal
	sc.script[1] = []engine.Action{engine.Call{S: 1}, engine.Check{S: 1}}
	sc.script[2] = []engine.Action{engine.Check{S: 2}, engine.Check{S: 2}}
	m := buildTable(t, sc, 80, 24)
	m.Update(key("z")) // dismiss the button moment
	m.Update(key("c")) // hero calls; villains respond on timers

	for i := 0; i < 2000; i++ {
		if m.bar.State() == ActionBarChoosing || m.handDone {
			return // reached the hero's next turn without a mid-villain popup
		}
		if m.overlay != nil {
			t.Fatal("a moment fired outside the hero's turn")
		}
		m.Update(stepTickMsg{seq: m.seq})
		m.Update(villainTickMsg{seq: m.seq})
	}
	t.Fatal("hand never advanced")
}

// TestMomentsSilentAtCoachOff: Off means leave me alone — and an unfired
// moment is not burned, so nothing may be marked seen either.
func TestMomentsSilentAtCoachOff(t *testing.T) {
	sc, prof := freshMomentScenario()
	sc.coach = CoachOff
	m := buildTable(t, sc, 80, 24)
	if m.overlay != nil {
		t.Error("no popup may open at CoachOff")
	}
	if len(prof.MomentsSeen) != 0 {
		t.Errorf("CoachOff consulted (and burned) %d moments", len(prof.MomentsSeen))
	}
}

// TestMomentPopupKeepsFrameHeight: the popup band replaces rows, never adds
// them — the frame must stay exactly the terminal height.
func TestMomentPopupKeepsFrameHeight(t *testing.T) {
	for _, bp := range breakpoints {
		sc, _ := freshMomentScenario()
		m := buildTable(t, sc, bp.w, bp.h)
		if m.overlay == nil {
			t.Fatalf("%dx%d: setup: moment should be up", bp.w, bp.h)
		}
		rows := strings.Split(m.View(), "\n")
		if len(rows) != bp.h {
			t.Errorf("%dx%d: popup frame is %d rows", bp.w, bp.h, len(rows))
		}
		for i, r := range rows {
			if w := len([]rune(stripANSI(r))); w > bp.w {
				t.Errorf("%dx%d: popup row %d is %d cells wide", bp.w, bp.h, i, w)
			}
		}
	}
}
