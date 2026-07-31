package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// Tests for the seat dossier (the o key), the observed per-seat stats
// behind it, the quick-reference Reads tab, and the archetype-read
// teachable moments. The through-line is docs/design-learning.md lesson
// 12: the one-word seat-plate read is a claim, and the dossier's numbers
// — derived from what each seat actually did, never from its dials — are
// the evidence.

// profileWithMomentsSeenExcept is profileWithMomentsSeen with the given
// moments left unseen, so a scenario can exercise exactly one first
// encounter.
func profileWithMomentsSeenExcept(ids ...string) *profile.Profile {
	p := profileWithMomentsSeen()
	for _, id := range ids {
		delete(p.MomentsSeen, id)
	}
	return p
}

// TestDossierOpensCyclesAndDismisses drives the o key end to end: the
// panel opens on the first villain with the claim (label, read, blurb)
// and the evidence (the observed numbers), o walks to the next seat, and
// any other key restores the exact frame underneath.
func TestDossierOpensCyclesAndDismisses(t *testing.T) {
	m := buildTable(t, scenarioShowdown(), 80, 24)
	base := m.View()

	m.Update(key("o"))
	if m.overlay == nil || !m.overlay.dossier {
		t.Fatal("o must open the seat dossier")
	}
	view := stripANSI(m.View())
	for _, want := range []string{
		"Tara",                               // the seat, not the archetype in the abstract
		"The Nit",                            // the archetype label
		"\"tight\"",                          // the seat-plate word being decoded
		"Plays very few hands",               // the blurb: the claim
		"adjust",                             // the one-line exploit
		"VPIP",                               // the evidence
		"100% (1/1) entered the pot preflop", // Tara limped hand #1: observed, not asserted
		"PFR",
		"0% (0/1) raised preflop",
		"o next opponent", // how to keep reading the table
	} {
		if !strings.Contains(view, want) {
			t.Errorf("dossier missing %q:\n%s", want, view)
		}
	}

	// o again walks clockwise to the next seat.
	m.Update(key("o"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "Nia") || !strings.Contains(view, "Tight-Aggressive") {
		t.Errorf("second o must show the next opponent's dossier:\n%s", view)
	}
	if !strings.Contains(view, "\"solid\"") {
		t.Errorf("Nia's dossier must decode her read:\n%s", view)
	}

	// Any other key dismisses and restores the exact previous frame.
	m.Update(key("z"))
	if m.overlay != nil {
		t.Fatal("a non-o key must dismiss the dossier")
	}
	if m.View() != base {
		t.Error("dismissing the dossier must restore the exact previous frame")
	}
}

// TestDossierBeforeAnyHandCompletes: mid-hand #1 there is no committed
// evidence yet, and the dossier must say so rather than divide by zero or
// invent numbers.
func TestDossierBeforeAnyHandCompletes(t *testing.T) {
	m := buildTable(t, scenarioPreflop(), 80, 24)
	m.Update(key("o"))
	view := stripANSI(m.View())
	if !strings.Contains(view, "no completed hands yet") {
		t.Errorf("dossier with no history must say so:\n%s", view)
	}
	if strings.Contains(view, "VPIP") {
		t.Errorf("no observed numbers may render before a hand completes:\n%s", view)
	}
}

// TestDossierKeepsFrameGeometry: the dossier is an overlay — it replaces
// rows of the frozen frame and must never change the frame's height or
// overflow its width, at every breakpoint.
func TestDossierKeepsFrameGeometry(t *testing.T) {
	for _, bp := range breakpoints {
		m := buildTable(t, scenarioShowdown(), bp.w, bp.h)
		m.Update(key("o"))
		rows := strings.Split(m.View(), "\n")
		if len(rows) != bp.h {
			t.Errorf("%dx%d: dossier frame is %d rows, want %d", bp.w, bp.h, len(rows), bp.h)
		}
		for i, r := range rows {
			if w := len([]rune(stripANSI(r))); w > bp.w {
				t.Errorf("%dx%d: dossier row %d is %d cells wide", bp.w, bp.h, i, w)
			}
		}
	}
}

// TestObservedStatsFromRealPlay is the honesty test: seat the real
// archetypes, let them play a session, and assert the dossier's numbers —
// computed purely from observed actions — reproduce the ordering the
// archetypes were built to produce. The station (RangeScale 1.6) must
// voluntarily enter far more pots than the nit (0.6).
func TestObservedStatsFromRealPlay(t *testing.T) {
	cfg := TableConfig{
		SmallBlind: 5, BigBlind: 10, Stack: 1000,
		// station and nit in adjacent seats; the rest fill the table.
		Lineup:    []string{"station", "nit", "tag", "lag", "maniac"},
		CoachMode: CoachOff, Speed: SpeedInstant,
	}
	ts := NewTableScreen(cfg, DefaultPrefs(), profileWithMomentsSeen())
	ts.seed, ts.seeded = 99, true
	ts.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	ts.Init()
	var m tea.Model = ts
	for i := 0; i < 50; i++ {
		m = driveKeys(m, "f", " ", "\r") // hero folds; the villains play
	}

	// The station can bust before the session ends (calling with anything
	// does that); a dozen observed hands is plenty for the ordering.
	station, nit := ts.stats[1], ts.stats[2]
	if station.Hands < 12 || nit.Hands < 12 {
		t.Fatalf("too few observed hands (station %d, nit %d)", station.Hands, nit.Hands)
	}
	rate := func(n, hands int) float64 { return float64(n) / float64(hands) }
	if rate(station.VPIP, station.Hands) <= rate(nit.VPIP, nit.Hands) {
		t.Errorf("observed VPIP must expose the styles: station %d/%d vs nit %d/%d",
			station.VPIP, station.Hands, nit.VPIP, nit.Hands)
	}
	// The maniac raises more hands than the nit — the PFR side of the read.
	maniac := ts.stats[5]
	if rate(maniac.PFR, maniac.Hands) <= rate(nit.PFR, nit.Hands) {
		t.Errorf("observed PFR must expose the styles: maniac %d/%d vs nit %d/%d",
			maniac.PFR, maniac.Hands, nit.PFR, nit.Hands)
	}

	// And the dossier renders the observed numbers, not blanks.
	ts.Update(key("o"))
	if view := stripANSI(ts.View()); !strings.Contains(view, "VPIP") ||
		!strings.Contains(view, "hands this session") {
		t.Errorf("dossier after a real session must show the observed stats:\n%s", view)
	}
}

// TestReadsTabCoversEveryArchetype is the generality gate on the quick
// reference: every seatable archetype must have a note in opponentNotes
// and appear — word and label — in the rendered Reads tab. A sixth
// archetype cannot reach a seat plate undocumented without failing here.
func TestReadsTabCoversEveryArchetype(t *testing.T) {
	q := NewQuickReference()
	q.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	panel := stripANSI(q.renderReads())

	for key, p := range ai.Archetypes {
		if key == "coach" {
			continue // an advisor, not a seatable opponent
		}
		n, ok := opponentNotes[key]
		if !ok {
			t.Errorf("archetype %q has no opponentNotes entry", key)
		} else if n.meaning == "" || n.adjust == "" {
			t.Errorf("archetype %q note incomplete: %+v", key, n)
		}
		if !strings.Contains(panel, p.Read) {
			t.Errorf("Reads tab must explain the word %q (archetype %q):\n%s", p.Read, key, panel)
		}
		if !strings.Contains(panel, p.Label) {
			t.Errorf("Reads tab must name %q (archetype %q):\n%s", p.Label, key, panel)
		}
	}

	// Close the loop with the table itself: every word engraved on a seat
	// plate in the default lineup is a word this tab explains.
	m := buildTable(t, scenarioPreflop(), 80, 24)
	for s := engine.Seat(1); s.Valid(); s++ {
		read := m.reads[s]
		if read == "" {
			t.Errorf("seat %d has no read in the classroom lineup", s)
			continue
		}
		if !strings.Contains(panel, read) {
			t.Errorf("seat plate word %q is not explained in the Reads tab", read)
		}
	}
}

// scenarioStationCallsDown: the station (Ivy, seat 4 in the classroom
// lineup) calls the hero's pot-size flop bet, then checks the turn to the
// hero — the read's characteristic behaviour, on the table, right before
// the hero's next decision.
func scenarioStationCallsDown() tableScenario {
	return tableScenario{
		seed: 17, speed: SpeedInstant, coach: CoachFull,
		script: map[engine.Seat][]engine.Action{
			1: {engine.Fold{S: 1}},
			2: {engine.Check{S: 2}, engine.Check{S: 2}, engine.Fold{S: 2}},
			3: {engine.Fold{S: 3}},
			4: {engine.Call{S: 4}, engine.Check{S: 4}, engine.Call{S: 4}, engine.Check{S: 4}},
			5: {engine.Fold{S: 5}},
		},
		// Hero calls preflop, then pot-bets the flop (preset 4).
		keys: []string{"c", "b", "4", "enter"},
	}
}

// TestStationReadMomentFiresOnTheCallDown: the first time the station
// pays off a big bet, the word "sticky" gets its one explanation — on the
// hero's next turn, never on a villain's, and persisted forever.
func TestStationReadMomentFiresOnTheCallDown(t *testing.T) {
	sc := scenarioStationCallsDown()
	prof := profileWithMomentsSeenExcept("read_sticky")
	sc.prof = prof
	m := buildTable(t, sc, 80, 24)

	if m.bar.State() != ActionBarChoosing || m.hand.Street() != engine.Turn {
		t.Fatalf("setup: want the hero's turn on the turn, got %v on %v",
			m.bar.State(), m.hand.Street())
	}
	if m.overlay == nil {
		t.Fatal("the sticky read moment should fire after the station's call-down")
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "\"sticky\"") || !strings.Contains(view, "calling station") {
		t.Errorf("popup must decode the word on the plate:\n%s", view)
	}
	if _, seen := prof.MomentsSeen["read_sticky"]; !seen {
		t.Error("firing must persist to the profile immediately")
	}

	// Same spot, same profile: fired once, forever.
	sc2 := scenarioStationCallsDown()
	sc2.prof = prof
	if m2 := buildTable(t, sc2, 80, 24); m2.overlay != nil {
		t.Error("a seen read moment must never fire again for this profile")
	}
}

// TestNitReadMomentFiresFacingTheRaise: the nit's raise means the top of
// the deck — the moment lands exactly while the hero is deciding whether
// to pay it.
func TestNitReadMomentFiresFacingTheRaise(t *testing.T) {
	sc := tableScenario{
		seed: 17, speed: SpeedInstant, coach: CoachFull,
		script: map[engine.Seat][]engine.Action{
			1: {engine.Raise{S: 1, To: 60}}, // Tara, the nit, wakes up in the SB
			2: {engine.Fold{S: 2}},
			3: {engine.Fold{S: 3}},
			4: {engine.Fold{S: 4}},
			5: {engine.Fold{S: 5}},
		},
		keys: []string{"c"}, // hero limps the button; the raise comes back around
	}
	prof := profileWithMomentsSeenExcept("read_tight")
	sc.prof = prof
	m := buildTable(t, sc, 80, 24)

	if m.bar.State() != ActionBarChoosing || m.hand.ToCall(heroSeat) <= 0 {
		t.Fatalf("setup: hero should be facing the nit's raise (toCall %v)",
			m.hand.ToCall(heroSeat))
	}
	if m.overlay == nil {
		t.Fatal("the tight read moment should fire while facing the nit's raise")
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "\"tight\"") {
		t.Errorf("popup must decode the word on the plate:\n%s", view)
	}
	if _, seen := prof.MomentsSeen["read_tight"]; !seen {
		t.Error("firing must persist to the profile immediately")
	}
}

// TestReadMomentsRespectOnePerDecision: when a registry moment already
// fired for this decision, the read candidates wait for their own spot —
// at most one popup per decision, ever.
func TestReadMomentsRespectOnePerDecision(t *testing.T) {
	sc := tableScenario{
		seed: 17, speed: SpeedInstant, coach: CoachFull,
		script: map[engine.Seat][]engine.Action{
			1: {engine.Raise{S: 1, To: 60}},
			2: {engine.Fold{S: 2}},
			3: {engine.Fold{S: 3}},
			4: {engine.Fold{S: 4}},
			5: {engine.Fold{S: 5}},
		},
	}
	// Both a registry moment (the button-raise moment, whose trigger is
	// this exact seed-17 first decision) and the read moment are unseen;
	// each decision may surface only one.
	prof := profileWithMomentsSeenExcept("read_tight", "first_button_raise_hand")
	sc.prof = prof
	m := buildTable(t, sc, 80, 24)

	if m.overlay == nil {
		t.Fatal("setup: the button moment should be up on the first decision")
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "The button raise") {
		t.Fatalf("first decision belongs to the higher-priority registry moment:\n%s", view)
	}
	if len(prof.MomentsSeen) != len(profileWithMomentsSeen().MomentsSeen)-1 {
		t.Fatal("exactly one moment may be marked seen on the first decision")
	}
	m.Update(key("z")) // dismiss the popup (consumed)
	m.Update(key("c")) // now actually call
	pumpTable(t, m)
	if m.bar.State() != ActionBarChoosing || m.hand.ToCall(heroSeat) <= 0 {
		t.Fatalf("hero should be facing the nit's raise, got %v", m.bar.State())
	}
	if m.overlay == nil {
		t.Fatal("the read moment gets its own later decision")
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "\"tight\"") {
		t.Errorf("second decision's popup should be the read moment:\n%s", view)
	}
}
