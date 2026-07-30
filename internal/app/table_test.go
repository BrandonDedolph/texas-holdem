package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// Table-screen tests. Every game state is reached through the engine: a
// scripted villain plays queued actions, the hero plays key presses through
// the real Update loop. The one sanctioned exception is presentation-only
// state (action-bar substate, label text) in the layout-stability matrix.

// scriptedOpponent plays a fixed sequence of actions, then falls back to
// check/call so an exhausted script can never wedge a hand.
type scriptedOpponent struct {
	name   string
	script []engine.Action
}

// Name implements Opponent.
func (o *scriptedOpponent) Name() string { return o.name }

// Act implements Opponent.
func (o *scriptedOpponent) Act(v *engine.PlayerView) engine.Action {
	if len(o.script) > 0 {
		a := o.script[0]
		o.script = o.script[1:]
		return a
	}
	return checkCallOpponent{name: o.name}.Act(v)
}

// tableScenario describes a reproducible session state: per-seat stacks,
// villain scripts, and the hero's key presses.
type tableScenario struct {
	stacks map[engine.Seat]engine.Chips
	script map[engine.Seat][]engine.Action
	keys   []string
	speed  Speed
	coach  CoachMode
	seed   uint64

	// prof lets a test watch profile side effects (teachable moments).
	// When nil, buildTable seats a fresh profile with every moment already
	// seen, so scenarios exercise the table, not the first-contact popups —
	// the moment tests opt in with an explicit fresh profile.
	prof *profile.Profile
}

// buildTable constructs a TableScreen, deals the first hand, and replays
// the scenario. At SpeedInstant everything settles synchronously.
func buildTable(t *testing.T, sc tableScenario, w, h int) *TableScreen {
	t.Helper()
	cfg := TableConfig{
		SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), CoachMode: sc.coach, Speed: sc.speed,
	}
	var stacks [engine.MaxSeats]engine.Chips
	for s := engine.Seat(0); s.Valid(); s++ {
		stacks[s] = cfg.Stack
	}
	for s, c := range sc.stacks {
		stacks[s] = c
	}
	var opps [engine.MaxSeats]Opponent
	for s := engine.Seat(1); s.Valid(); s++ {
		opps[s] = &scriptedOpponent{name: villainNames[s], script: sc.script[s]}
	}

	prof := sc.prof
	if prof == nil {
		prof = profileWithMomentsSeen()
	}
	m := newTableScreen(cfg, nil, prof, stacks, opps)
	m.seed, m.seeded = sc.seed, true
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Init()
	pumpTable(t, m)
	for _, k := range sc.keys {
		m.Update(key(k))
		pumpTable(t, m)
	}
	return m
}

// profileWithMomentsSeen returns a fresh profile with every teachable
// moment marked seen, so table scenarios settle without a popup waiting for
// a keypress.
func profileWithMomentsSeen() *profile.Profile {
	p := profile.NewProfile()
	for _, mo := range coach.Moments() {
		p.MarkMomentSeen(mo.ID)
	}
	return p
}

// pumpTable stands in for the Bubble Tea runtime's timers: it injects the
// tick messages the table is waiting on (with the table's own live seq)
// until the session needs the keyboard. At SpeedInstant it is a no-op —
// everything already settled — which is exactly the §6.3 property.
func pumpTable(t *testing.T, m *TableScreen) {
	t.Helper()
	for i := 0; i < 2000; i++ {
		if m.paused || m.handDone || m.bar.State() != ActionBarWaiting || m.hand == nil {
			return
		}
		m.Update(stepTickMsg{seq: m.seq})
		if m.paused || m.handDone || m.bar.State() != ActionBarWaiting {
			return
		}
		m.Update(villainTickMsg{seq: m.seq})
	}
	t.Fatal("table never settled: animation or villain loop is stuck")
}

// foldOut folds seats 3, 4 and 5 (the seats that act before the hero on the
// button in hand #1).
func foldOut() map[engine.Seat][]engine.Action {
	return map[engine.Seat][]engine.Action{
		3: {engine.Fold{S: 3}},
		4: {engine.Fold{S: 4}},
		5: {engine.Fold{S: 5}},
	}
}

// scenarioPreflop: hand #1, blinds posted, UTG/HJ/CO fold, hero to act on
// the button facing the big blind.
func scenarioPreflop() tableScenario {
	return tableScenario{seed: 17, speed: SpeedInstant, coach: CoachFull, script: foldOut()}
}

// scenarioFlopChoosing: hero calls on the button, blinds come along, the
// flop checks to the hero.
func scenarioFlopChoosing() tableScenario {
	sc := scenarioPreflop()
	sc.script[1] = []engine.Action{engine.Call{S: 1}, engine.Check{S: 1}}
	sc.script[2] = []engine.Action{engine.Check{S: 2}, engine.Check{S: 2}}
	sc.keys = []string{"c"}
	return sc
}

// scenarioFlopSizing: as above, then the hero opens bet sizing.
func scenarioFlopSizing() tableScenario {
	sc := scenarioFlopChoosing()
	sc.keys = append(sc.keys, "b")
	return sc
}

// scenarioSidePot: short stacks in the blinds, hero opens to 60, both
// blinds shove for different amounts — hero to act facing two all-ins with
// the pot already layered.
func scenarioSidePot() tableScenario {
	sc := scenarioPreflop()
	sc.stacks = map[engine.Seat]engine.Chips{0: 2000, 1: 300, 2: 700}
	sc.script[1] = []engine.Action{engine.Raise{S: 1, To: 300}}
	sc.script[2] = []engine.Action{engine.Raise{S: 2, To: 700}}
	// r opens sizing; 6-0 types 60 (digits 6-9 begin a typed amount); enter
	// confirms the open.
	sc.keys = []string{"r", "6", "0", "enter"}
	return sc
}

// scenarioShowdown: hero calls, the hand checks down to a three-way
// showdown, pots are awarded, the table rests between hands.
func scenarioShowdown() tableScenario {
	sc := scenarioPreflop()
	sc.script[1] = []engine.Action{engine.Call{S: 1}}
	sc.keys = []string{"c", "x", "x", "x"}
	return sc
}

func TestTableReachesHeroTurnPreflop(t *testing.T) {
	m := buildTable(t, scenarioPreflop(), 80, 24)
	if m.bar.State() != ActionBarChoosing {
		t.Fatalf("bar state = %v, want choosing", m.bar.State())
	}
	if !strings.Contains(m.status, "Your turn") {
		t.Errorf("status = %q, want a your-turn prompt", m.status)
	}
	if m.hand.CurrentSeat() != heroSeat {
		t.Errorf("engine says seat %d to act, want hero", m.hand.CurrentSeat())
	}
}

func TestTableFlopSizingState(t *testing.T) {
	m := buildTable(t, scenarioFlopSizing(), 80, 24)
	if m.bar.State() != ActionBarSizing {
		t.Fatalf("bar state = %v, want sizing", m.bar.State())
	}
	if m.hand.Street() != engine.Flop {
		t.Errorf("street = %v, want flop", m.hand.Street())
	}
	// The default sizing value is the half-pot preset.
	if got, want := m.bar.ConfirmAmount(), m.bar.presetAmounts()[1]; got != want {
		t.Errorf("initial sizing amount = %v, want half-pot preset %v", got, want)
	}
}

func TestTableSidePotsDisplayed(t *testing.T) {
	m := buildTable(t, scenarioSidePot(), 80, 24)
	view := stripANSI(m.View())
	if !strings.Contains(view, "MAIN") || !strings.Contains(view, "SIDE") {
		t.Errorf("side-pot spot must render layered pots in the open:\n%s", view)
	}
	// The engine's layers: 60x3 main, everyone; 240x2 side (Tara, Nia);
	// Nia's uncalled 400 as its own top layer.
	if !strings.Contains(view, "MAIN 180") || !strings.Contains(view, "SIDE 480") {
		t.Errorf("pot layers wrong:\n%s", view)
	}
	if m.bar.State() != ActionBarChoosing {
		t.Errorf("hero should be choosing, got %v", m.bar.State())
	}
}

func TestTableShowdownRevealsAndAwards(t *testing.T) {
	m := buildTable(t, scenarioShowdown(), 80, 24)
	if !m.handDone {
		t.Fatal("showdown scenario should end between hands")
	}
	if m.awardText == "" {
		t.Error("award text empty after a showdown")
	}
	revealed := 0
	for s := engine.Seat(0); s.Valid(); s++ {
		if m.revealed[s] {
			revealed++
		}
	}
	if revealed < 3 {
		t.Errorf("revealed %d seats, want all three showdown hands", revealed)
	}
	if m.winners == 0 {
		t.Error("no winning cards highlighted at showdown")
	}
}

func TestTableLearnSpeedPausesAfterFlop(t *testing.T) {
	sc := scenarioFlopChoosing()
	sc.speed = SpeedLearn
	sc.keys = nil // stop before the hero's preflop call
	m := buildTable(t, sc, 80, 24)

	// Hero calls; villains act (via pumped think ticks) and the flop lands —
	// at Learn the game must stop on the pause, not on the hero's turn.
	m.Update(key("c"))
	pumpTable(t, m)
	if !m.paused {
		t.Fatalf("Learn speed must pause after the flop; status %q", m.status)
	}
	if !strings.Contains(m.status, "space to continue") {
		t.Errorf("pause status = %q, want a space-to-continue prompt", m.status)
	}
	if m.boardShown != 3 {
		t.Errorf("boardShown = %d at the flop pause, want 3", m.boardShown)
	}

	// Space dismisses the pause and play resumes to the hero's flop turn.
	m.Update(key(" "))
	pumpTable(t, m)
	if m.bar.State() != ActionBarChoosing {
		t.Errorf("after the pause the flop should check to the hero, got %v", m.bar.State())
	}
}

func TestTableVillainOddsMatchEquityMath(t *testing.T) {
	m := buildTable(t, scenarioFlopSizing(), 80, 24)

	// Pot is 30 (three limps); the villain the math speaks about is Tara.
	pot := m.hand.PotTotal()
	if pot != 30 {
		t.Fatalf("flop pot = %v, want 30", pot)
	}
	amt := m.bar.ConfirmAmount() // half pot = 15
	potAfter := pot + amt
	want := "Tara calls " + amt.String()
	got := m.villainOddsText()
	if !strings.Contains(got, want) {
		t.Errorf("villain odds %q, want it to start %q", got, want)
	}
	if !strings.Contains(got, equity.OddsToOne(amt, potAfter)) {
		t.Errorf("villain odds %q must quote %s", got, equity.OddsToOne(amt, potAfter))
	}
	pct := int(equity.PotOdds(amt, potAfter)*100 + 0.5)
	if !strings.Contains(got, "needs "+engine.Chips(pct).String()+"%") {
		t.Errorf("villain odds %q must quote the required equity %d%%", got, pct)
	}
}

func TestTableTypedAmountIsEngineClamped(t *testing.T) {
	m := buildTable(t, scenarioFlopSizing(), 80, 24)
	opt, ok := m.hand.LegalActions().Find(engine.ActionBet)
	if !ok {
		t.Fatal("bet must be legal in the sizing scenario")
	}
	// Type an absurd over-max amount; confirm must submit the engine's Max.
	for _, k := range []string{"9", "9", "9", "9", "9"} {
		m.Update(key(k))
	}
	if got := m.bar.ConfirmAmount(); got != opt.Max {
		t.Errorf("over-max typed amount confirms %v, want engine max %v", got, opt.Max)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "all-in "+opt.Max.String()) {
		t.Errorf("over-max typed amount must show the clamp it will receive:\n%s", view)
	}
}

func TestTableInstantIsDeterministic(t *testing.T) {
	a := buildTable(t, scenarioShowdown(), 80, 24)
	b := buildTable(t, scenarioShowdown(), 80, 24)
	if a.View() != b.View() {
		t.Error("same seed and script must render byte-identical views")
	}
}

func TestTableSpaceDealsNextHand(t *testing.T) {
	m := buildTable(t, scenarioShowdown(), 80, 24)
	if m.handNo != 1 || !m.handDone {
		t.Fatalf("setup: handNo=%d handDone=%v", m.handNo, m.handDone)
	}
	m.Update(key(" "))
	if m.handNo != 2 {
		t.Errorf("space between hands should deal hand #2, got #%d", m.handNo)
	}
	if m.handDone {
		t.Error("new hand should clear the between-hands state")
	}
}

func TestTableReviewNavigatesWithReturnTo(t *testing.T) {
	m := buildTable(t, scenarioShowdown(), 80, 24)
	cmd, handled := m.handleAction(ActReview)
	if !handled || cmd == nil {
		t.Fatal("v between hands must produce a navigation command")
	}
	msg, ok := cmd().(NavigateMsg)
	if !ok || msg.Screen != ScreenHandReview {
		t.Fatalf("review should navigate to HandReview, got %#v", cmd())
	}
	req, ok := msg.Data.(ReviewRequest)
	if !ok || req.ReturnTo != ScreenTable {
		t.Errorf("review payload = %#v, want ReviewRequest returning to the table", msg.Data)
	}
}

func TestTableEscCancelsSizingThenLeaves(t *testing.T) {
	m := buildTable(t, scenarioFlopSizing(), 80, 24)
	cmd, _ := m.handleAction(ActBack)
	if cmd != nil {
		t.Fatal("esc in sizing mode must cancel, not navigate")
	}
	if m.bar.State() != ActionBarChoosing {
		t.Fatalf("esc should fall back to choosing, got %v", m.bar.State())
	}
	cmd, _ = m.handleAction(ActBack)
	if cmd == nil {
		t.Fatal("esc while choosing must navigate to the menu")
	}
	if msg, ok := cmd().(NavigateMsg); !ok || msg.Screen != ScreenMainMenu {
		t.Errorf("esc should go to the main menu, got %#v", cmd())
	}
}

func TestTableFooterReservesReviewSlot(t *testing.T) {
	live := buildTable(t, scenarioPreflop(), 80, 24)
	done := buildTable(t, scenarioShowdown(), 80, 24)

	liveView, doneView := stripANSI(live.View()), stripANSI(done.View())
	if strings.Contains(liveView, "v review") {
		t.Error("v review must not render mid-hand (v does nothing there)")
	}
	if !strings.Contains(doneView, "v review") {
		t.Error("v review must render between hands")
	}
	// The reserved slot keeps the rest of the footer anchored.
	if posOf(liveView, "esc menu") != posOf(doneView, "esc menu") {
		t.Error("footer anchor moved when the review hint appeared")
	}
}

func TestTableViewFillsEveryBreakpointExactly(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24}, {104, 30}, {60, 20}, {90, 26}, {120, 40}, {62, 21},
	}
	for _, sz := range sizes {
		m := buildTable(t, scenarioFlopChoosing(), sz.w, sz.h)
		sized(t, m, sz.w, sz.h)
		for i, line := range strings.Split(stripANSI(m.View()), "\n") {
			if got := len([]rune(line)); got > sz.w {
				t.Errorf("%dx%d: line %d is %d cells wide", sz.w, sz.h, i, got)
			}
		}
	}
}

func TestTableWideLayoutShowsCoachPanel(t *testing.T) {
	m := buildTable(t, scenarioFlopChoosing(), 104, 30)
	view := stripANSI(m.View())
	if !strings.Contains(view, "COACH") {
		t.Error("wide layout must render the coach side panel")
	}
}

// heroLineOf returns the hero's reserved action/grade line — row 15 of the
// 80x24 full layout — ANSI-stripped, when it carries the given action text;
// "" otherwise. Addressing the fixed row is the point: the budget promises
// the line is always exactly there.
func heroLineOf(view, action string) string {
	rows := strings.Split(stripANSI(view), "\n")
	const heroLineRow = 14 // row 15, zero-indexed
	if len(rows) <= heroLineRow || !strings.Contains(rows[heroLineRow], action) {
		return ""
	}
	return rows[heroLineRow]
}

// TestGradePersistsUntilNextDecision is the F2 regression: the frozen grade
// of the hero's last action must still be on screen when the hero's NEXT
// turn arms — at every speed, including Instant, where the old
// grade-while-villains-respond window never rendered a single frame.
func TestGradePersistsUntilNextDecision(t *testing.T) {
	for _, speed := range []Speed{SpeedLearn, SpeedNormal, SpeedFast, SpeedInstant} {
		t.Run(speed.String(), func(t *testing.T) {
			sc := scenarioFlopChoosing() // hero calls preflop, flop checks to hero
			sc.speed = speed
			m := buildTable(t, sc, 80, 24)
			if speed == SpeedLearn {
				if !m.paused {
					t.Fatal("Learn speed should pause on the flop")
				}
				m.Update(key(" "))
				pumpTable(t, m)
			}
			if m.bar.State() != ActionBarChoosing || m.hand.Street() != engine.Flop {
				t.Fatalf("setup: want the hero's flop turn, got %v on %v", m.bar.State(), m.hand.Street())
			}

			// The preflop call (an Inaccuracy against the coach's raise) is
			// still on the hero's reserved line, grade attached.
			line := heroLineOf(m.View(), "calls 10")
			if line == "" {
				t.Fatalf("hero's last action left the screen before his next decision:\n%s",
					stripANSI(m.View()))
			}
			if !strings.Contains(line, "Inaccuracy") {
				t.Errorf("hero line %q lost the frozen grade", line)
			}

			// The next decision replaces it — grades never stack or linger
			// past the action they judged.
			m.Update(key("x"))
			pumpTable(t, m)
			view := stripANSI(m.View())
			if got := heroLineOf(view, "checks"); got == "" || !strings.Contains(got, "Best") {
				t.Errorf("next decision's grade must take over the line, got %q", got)
			}
			if heroLineOf(view, "calls 10") != "" {
				t.Errorf("stale decision still on screen after the next one:\n%s", view)
			}
		})
	}
}

// TestGradeRespectsVerbosityOnHeroLine: the suffix is composed at render
// time from the LIVE mode — Off shows the action but never the opinion, and
// Mistakes withholds the grade when the decision was fine.
func TestGradeRespectsVerbosityOnHeroLine(t *testing.T) {
	sc := scenarioFlopChoosing()
	sc.coach = CoachOff
	m := buildTable(t, sc, 80, 24)
	if line := heroLineOf(m.View(), "calls 10"); line == "" || strings.Contains(line, "Inaccuracy") {
		t.Errorf("CoachOff must record silently; hero line = %q", line)
	}
	// Cycling to Full mid-hand reveals the already-frozen grade — display
	// is a live choice, the grade never was.
	m.coachMode = CoachFull
	if line := heroLineOf(m.View(), "calls 10"); !strings.Contains(line, "Inaccuracy") {
		t.Errorf("cycling to Full must reveal the recorded grade; hero line = %q", line)
	}

	sc = scenarioFlopChoosing()
	sc.coach = CoachMistakes
	sc.keys = append(sc.keys, "x") // flop check matches the coach: Best
	m = buildTable(t, sc, 80, 24)
	if line := heroLineOf(m.View(), "checks"); line == "" || strings.Contains(line, "Best") {
		t.Errorf("Mistakes mode must stay silent on a good decision; hero line = %q", line)
	}
}

// TestCompactShowsPotOddsFacingBet is the F6 regression: the 60x20 ledger
// must never drop the price of a decision — in any coach mode.
func TestCompactShowsPotOddsFacingBet(t *testing.T) {
	for _, mode := range []CoachMode{CoachFull, CoachMistakes, CoachOff} {
		t.Run(mode.String(), func(t *testing.T) {
			sc := scenarioPreflop() // hero on the button facing the blind: 10 into 15
			sc.coach = mode
			m := buildTable(t, sc, 60, 20)
			view := stripANSI(m.View())
			want := equity.PotOddsText(10, 15)
			if !strings.Contains(view, want) {
				t.Errorf("compact %v frame facing a bet has no price %q:\n%s", mode, want, view)
			}
		})
	}
}

// TestExpandOverlayShowsFullReasoning: the e key opens the §5.1 overlay
// with the reasoning the strip clipped, and any key restores the exact
// frame underneath.
func TestExpandOverlayShowsFullReasoning(t *testing.T) {
	m := buildTable(t, scenarioFlopChoosing(), 80, 24)
	if m.advice == nil {
		t.Fatal("setup: the hero's turn should carry advice")
	}
	base := m.View()
	if !strings.Contains(stripANSI(base), expandMarker) {
		t.Fatalf("this spot's reasoning should overflow the strip:\n%s", stripANSI(base))
	}

	m.Update(key("e"))
	view := stripANSI(m.View())
	// "(limped)" is in the clipped tail of the strip — the overlay must
	// carry the whole thought.
	if !strings.Contains(view, "(limped)") {
		t.Errorf("overlay must show the clipped reasoning in full:\n%s", view)
	}
	if !strings.Contains(view, "any key continues") {
		t.Error("overlay must say how to dismiss itself")
	}

	m.Update(key("z"))
	if m.View() != base {
		t.Error("dismissing the overlay must restore the exact previous frame")
	}
}

// TestExpandRespectsVerbosity: at Mistakes/Off the e key must not open a
// back door to the opinion the mode withholds.
func TestExpandRespectsVerbosity(t *testing.T) {
	for _, mode := range []CoachMode{CoachMistakes, CoachOff} {
		sc := scenarioFlopChoosing()
		sc.coach = mode
		m := buildTable(t, sc, 80, 24)
		before := m.View()
		m.Update(key("e"))
		if m.View() != before {
			t.Errorf("%v: e must do nothing — expanding would reveal the withheld opinion", mode)
		}
	}
}

func TestTableResumeAfterNavigationRearms(t *testing.T) {
	// Leave mid-hand at Learn speed (a villain think tick was pending and is
	// lost when the player navigates away), come back: Init must reissue it
	// rather than dead-air. Built without pumping so the villain is still
	// mid-think.
	cfg := TableConfig{SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), Speed: SpeedLearn}
	var stacks [engine.MaxSeats]engine.Chips
	for s := engine.Seat(0); s.Valid(); s++ {
		stacks[s] = 1000
	}
	var opps [engine.MaxSeats]Opponent
	for s := engine.Seat(1); s.Valid(); s++ {
		opps[s] = checkCallOpponent{name: villainNames[s]}
	}
	m := newTableScreen(cfg, nil, profile.NewProfile(), stacks, opps)
	m.seed, m.seeded = 17, true
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Learn-speed Init must schedule the first villain think")
	}
	if m.hand == nil || m.hand.CurrentSeat() == heroSeat {
		t.Fatal("setup: expected a villain to be thinking")
	}
	if cmd := m.Init(); cmd == nil { // what App.navigate runs on return
		t.Fatal("Init on a live session must resume the villain timer")
	}
}
