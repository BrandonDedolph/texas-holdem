package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/review"
	tea "github.com/charmbracelet/bubbletea"
)

// Review-screen tests. The reviewed hand is always reached through the real
// engine (buildTable + scripted villains), recorded via lastHandArchive, and
// replayed through internal/review — the same pipeline the app runs when the
// player presses v between hands.

// sentinelBody is a string no computation would ever produce: if it shows up
// in the rendered frame, it got there by being rendered verbatim.
const sentinelBody = "SENTINEL-THEN needed 25%, had ~31% vs his likely range (9 outs)"

// buildReview plays scenarioShowdown to completion, attaches frozen sentinel
// grades to the hero's first and last decisions, and opens the review sized
// to w x h.
func buildReview(t *testing.T, w, h int) *HandReview {
	t.Helper()
	table := buildTable(t, scenarioShowdown(), w, h)
	rec, ann, ok := table.lastHandArchive()
	if !ok {
		t.Fatal("showdown scenario must yield a completed hand to archive")
	}

	var heroIdx []int
	for i, e := range rec.Events {
		if e.Kind == engine.EvAction && e.Seat == heroSeat {
			heroIdx = append(heroIdx, i)
		}
	}
	if len(heroIdx) < 2 {
		t.Fatalf("expected at least 2 hero decisions, got %d", len(heroIdx))
	}
	ann.Grades[heroIdx[0]] = review.FrozenGrade{
		Band: "Good", EVLossBB: 0, Body: sentinelBody, Recommended: "call",
	}
	ann.Grades[heroIdx[len(heroIdx)-1]] = review.FrozenGrade{
		Band: "Mistake", EVLossBB: 1.2, Body: "SENTINEL-LAST", Recommended: "bet 20",
	}

	r := NewHandReview(review.Replay(rec, ann), ScreenTable)
	r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return r
}

// TestHandReviewRendersFrozenGradeVerbatim: the frozen coach note appears in
// the frame exactly as it was handed in — the screen renders grades, it
// never rewrites them (DESIGN.md §1 principle 1).
func TestHandReviewRendersFrozenGradeVerbatim(t *testing.T) {
	r := buildReview(t, 80, 24)
	view := stripANSI(r.View())

	if !strings.Contains(view, sentinelBody) {
		t.Errorf("frozen Body must render verbatim on the Then line:\n%s", view)
	}
	if !strings.Contains(view, "Coach: call") {
		t.Errorf("frozen recommendation must render:\n%s", view)
	}
	if !strings.Contains(view, "Grade: Good") {
		t.Errorf("frozen band must render as handed in:\n%s", view)
	}
}

// TestHandReviewHindsightIsSeparateAndTagged: the hindsight line is tagged,
// carries the revealed hand and true equity, and lives on its own row —
// never merged into the frozen Then line.
func TestHandReviewHindsightIsSeparateAndTagged(t *testing.T) {
	r := buildReview(t, 80, 24)
	view := stripANSI(r.View())

	if !strings.Contains(view, "hindsight") {
		t.Fatalf("hindsight layer must be tagged:\n%s", view)
	}
	if !strings.Contains(view, "held") || !strings.Contains(view, "true equity") {
		t.Errorf("hindsight line must show the revealed hand and true equity:\n%s", view)
	}

	var thenLine, nowLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Then") && strings.Contains(line, "SENTINEL") {
			thenLine = line
		}
		if strings.Contains(line, "Now") && strings.Contains(line, "true equity") {
			nowLine = line
		}
	}
	if thenLine == "" || nowLine == "" {
		t.Fatalf("Then and Now must each render on their own line:\n%s", view)
	}
	if strings.Contains(thenLine, "true equity") {
		t.Error("the frozen Then line must not absorb hindsight numbers")
	}
	if strings.Contains(nowLine, "SENTINEL") {
		t.Error("the hindsight Now line must not absorb the frozen note")
	}
}

// TestHandReviewLedgerIsDecisionBased: the DECISIONS line reads the frozen
// EV losses, the OUTCOME line reads the chips, and both are on screen at
// once — deliberately unaligned scoreboards.
func TestHandReviewLedgerIsDecisionBased(t *testing.T) {
	r := buildReview(t, 80, 24)
	view := stripANSI(r.View())

	if !strings.Contains(view, "DECISIONS") || !strings.Contains(view, "EV lost: 1.2bb") {
		t.Errorf("decision ledger must sum frozen EV losses:\n%s", view)
	}
	if !strings.Contains(view, "OUTCOME") || !strings.Contains(view, "results don't grade decisions") {
		t.Errorf("outcome line must render separately, with the disclaimer:\n%s", view)
	}
}

// TestHandReviewStepsThroughDecisionsToResult: arrows walk every decision
// and end on the result frame, which highlights the winning five (board
// cards included) and names the payout.
func TestHandReviewStepsThroughDecisionsToResult(t *testing.T) {
	r := buildReview(t, 80, 24)
	n := len(r.model.Decisions)
	if n < 2 {
		t.Fatalf("scenario yields %d decisions, want >= 2", n)
	}

	// Left at the first decision is a no-op.
	r.handleAction(ActLeft)
	if r.cursor != 0 {
		t.Error("left at the first decision must not underflow")
	}
	for i := 0; i < n+3; i++ { // overshoot: right must clamp at the result
		r.handleAction(ActRight)
	}
	if r.cursor != n {
		t.Fatalf("cursor = %d, want the result frame %d", r.cursor, n)
	}

	view := stripANSI(r.View())
	if !strings.Contains(view, "RESULT") {
		t.Errorf("result frame must announce itself:\n%s", view)
	}
	if !strings.Contains(view, "Winning five:") {
		t.Errorf("showdown result must list the winning five:\n%s", view)
	}
	if !strings.Contains(view, "board cards included") {
		t.Errorf("the best-five-of-seven lesson line is missing:\n%s", view)
	}
	if len(r.model.Outcome.WinningFive) != 5 {
		t.Errorf("winning five = %v, want 5 cards", r.model.Outcome.WinningFive)
	}
	if !strings.Contains(view, "wins ") {
		t.Errorf("the winner's seat row must show its award:\n%s", view)
	}
}

// TestHandReviewQuadrantLineRendered: the decision-vs-outcome line for a
// graded decision appears in the panel — the juxtaposition is the lesson.
func TestHandReviewQuadrantLineRendered(t *testing.T) {
	r := buildReview(t, 80, 24)
	d := r.model.Decisions[0]
	if d.Frozen == nil {
		t.Fatal("first decision should carry the sentinel grade")
	}
	want := review.DecisionOutcomeNote(*d.Frozen, r.model.Summary.HeroNetBB)
	if view := stripANSI(r.View()); !strings.Contains(view, want) {
		t.Errorf("quadrant line %q missing from the frame:\n%s", want, view)
	}
}

// TestHandReviewLayoutStable: anchors hold and the height never changes as
// the player steps through frames, at every breakpoint — the review obeys
// the same fixed-budget discipline as the table.
func TestHandReviewLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		r := buildReview(t, bp.w, bp.h)
		sized(t, r, bp.w, bp.h)

		n := len(r.model.Decisions)
		assertAnchorsStable(t, r.View,
			[]string{"HAND REVIEW", "YOU", "DECISIONS", "OUTCOME", "esc back"},
			map[string]func(){
				"step to decision 2": func() { r.cursor = 1 },
				"step to result":     func() { r.cursor = n },
				"back to first":      func() { r.cursor = 0 },
				"ungraded decision":  func() { r.cursor = 1 }, // no frozen grade attached
			})

		for i, line := range strings.Split(stripANSI(r.View()), "\n") {
			if got := len([]rune(line)); got > bp.w {
				t.Errorf("%dx%d: line %d is %d cells wide", bp.w, bp.h, i, got)
			}
		}
	}
}

// TestHandReviewTooSmallFloor: below the compact floor the App root refuses
// to render the screen at all; the screen's own View also stays inside any
// box it is given.
func TestHandReviewTooSmallFloor(t *testing.T) {
	a := newTestApp(t)
	a.models[ScreenTable] = buildTable(t, scenarioShowdown(), 80, 24)
	drive(t, a, NavigateMsg{Screen: ScreenHandReview, Data: ReviewRequest{ReturnTo: ScreenTable}})
	drive(t, a, tea.WindowSizeMsg{Width: 50, Height: 12})
	if view := stripANSI(a.View()); !strings.Contains(view, "Terminal too small") {
		t.Errorf("below 60x20 the app must show the too-small floor:\n%s", view)
	}
}

// TestHandReviewHonorsReturnTo: esc navigates to the screen the review was
// opened from — the table for "v", Lessons later.
func TestHandReviewHonorsReturnTo(t *testing.T) {
	r := buildReview(t, 80, 24)
	cmd, handled := r.handleAction(ActBack)
	if !handled || cmd == nil {
		t.Fatal("esc must produce a navigation command")
	}
	if msg, ok := cmd().(NavigateMsg); !ok || msg.Screen != ScreenTable {
		t.Errorf("esc should return to the table, got %#v", cmd())
	}

	lessons := NewHandReview(nil, ScreenLessons)
	cmd, _ = lessons.handleAction(ActBack)
	if msg, ok := cmd().(NavigateMsg); !ok || msg.Screen != ScreenLessons {
		t.Errorf("esc should honor ReviewRequest.ReturnTo, got %#v", cmd())
	}
}

// TestAppRoutesReviewFromTable: pressing v's navigation path end to end —
// the App builds a HandReview over the cached session's last hand and honors
// the return target.
func TestAppRoutesReviewFromTable(t *testing.T) {
	a := newTestApp(t)
	a.models[ScreenTable] = buildTable(t, scenarioShowdown(), 80, 24)

	drive(t, a, NavigateMsg{Screen: ScreenHandReview, Data: ReviewRequest{ReturnTo: ScreenTable}})
	if a.current != ScreenHandReview {
		t.Fatalf("current screen = %v, want HandReview", a.current)
	}
	r, ok := a.models[ScreenHandReview].(*HandReview)
	if !ok {
		t.Fatalf("HandReview model is %T", a.models[ScreenHandReview])
	}
	if r.model == nil {
		t.Fatal("review of a finished session must carry a replay model")
	}
	if r.returnTo != ScreenTable {
		t.Errorf("returnTo = %v, want the table", r.returnTo)
	}
	if !strings.Contains(stripANSI(a.View()), "HAND REVIEW") {
		t.Error("routed review must render its header")
	}
}

// TestAppReviewWithoutSessionShowsEmptyState: navigating to the review with
// no table session must not crash or refuse — it renders the empty state.
func TestAppReviewWithoutSessionShowsEmptyState(t *testing.T) {
	a := newTestApp(t)
	drive(t, a, NavigateMsg{Screen: ScreenHandReview, Data: ReviewRequest{ReturnTo: ScreenMainMenu}})

	view := stripANSI(a.View())
	if !strings.Contains(view, "No completed hand to review yet") {
		t.Errorf("empty review state missing:\n%s", view)
	}
}

// TestHandReviewLegendMatchesKeys: the populated screen's keys act exactly
// as documented — arrows change the frame, undocumented keys change nothing
// (the keybind-legend invariant, on the review's live state).
func TestHandReviewLegendMatchesKeys(t *testing.T) {
	r := buildReview(t, 80, 24)

	before := r.View()
	r.Update(key("right"))
	if r.View() == before {
		t.Error("documented key right must step the frame")
	}
	r.Update(key("left"))
	if r.View() != before {
		t.Error("left must step back to the identical frame")
	}

	for _, k := range []string{"x", "b", "f", "v", "tab", "1", "+", "backspace", " "} {
		before := r.View()
		_, cmd := r.Update(key(k))
		if cmd != nil {
			t.Errorf("undocumented key %q produced a command", k)
		}
		if r.View() != before {
			t.Errorf("undocumented key %q changed the view", k)
		}
	}

	r.Update(key("?"))
	view := r.View()
	for _, b := range handReviewKeys {
		if !strings.Contains(view, b.Label) || !strings.Contains(view, b.Help) {
			t.Errorf("help overlay missing %q (%s)", b.Label, b.Help)
		}
	}
}

// TestHandReviewArchiveMatchesEngineTruth: the record built by the table
// bridge reconstructs the stacks the engine actually settled — the replay
// pipeline is faithful end to end.
func TestHandReviewArchiveMatchesEngineTruth(t *testing.T) {
	table := buildTable(t, scenarioShowdown(), 80, 24)
	rec, ann, ok := table.lastHandArchive()
	if !ok {
		t.Fatal("no archive from a finished hand")
	}
	m := review.Replay(rec, ann)

	for _, s := range table.hand.DealtIn().Seats() {
		if got, want := m.Outcome.Stacks[s], table.hand.Stack(s); got != want {
			t.Errorf("seat %d: replayed final stack %v, engine says %v", s, got, want)
		}
	}
	res, _ := table.hand.Result()
	if m.Summary.HeroNet != res.Net[heroSeat] {
		t.Errorf("hero net %v, engine says %v", m.Summary.HeroNet, res.Net[heroSeat])
	}
}
