package review

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// truthSnap is the engine's own view of the table at one hero decision,
// captured while the fixture hand is played. Replay must reproduce it from
// the event log alone.
type truthSnap struct {
	street engine.Street
	board  []engine.Card
	pot    engine.Chips
	toCall engine.Chips
	stacks [engine.MaxSeats]engine.Chips
}

// playFixtureHand deals a fully scripted 3-handed hand through the real
// engine and returns the completed hand plus the engine-truth snapshot taken
// before each hero action.
//
// Button seat 2; hero seat 0 in the small blind with AhKh vs QsQd (seat 1,
// big blind) on Kd 9h 2s 5c 5d. Seat 2 folds preflop. The hero calls
// preflop, bets 20 on the flop (called), checks the turn through, bets 40 on
// the river (called), and wins the 140 showdown with kings and fives.
func playFixtureHand(t *testing.T) (*engine.Hand, []truthSnap) {
	t.Helper()
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: 5, BigBlind: 10, Seats: engine.MaxSeats},
		Button: 2,
		Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000, 2: 1000},
		Holes: map[engine.Seat][2]engine.Card{
			0: engine.Holes("Ah Kh"),
			1: engine.Holes("Qs Qd"),
			2: engine.Holes("7c 2d"),
		},
		Board: engine.MustCards("Kd 9h 2s 5c 5d"),
		Seed:  1,
		Eval:  eval.Evaluator,
	})
	if err != nil {
		t.Fatalf("NewHandFromSetup: %v", err)
	}

	script := []engine.Action{
		engine.Fold{S: 2},
		engine.Call{S: 0},
		engine.Check{S: 1},
		engine.Bet{S: 0, Amount: 20},
		engine.Call{S: 1},
		engine.Check{S: 0},
		engine.Check{S: 1},
		engine.Bet{S: 0, Amount: 40},
		engine.Call{S: 1},
	}
	var snaps []truthSnap
	for _, a := range script {
		if a.Seat() == 0 {
			var stacks [engine.MaxSeats]engine.Chips
			for s := engine.Seat(0); s.Valid(); s++ {
				stacks[s] = h.Stack(s)
			}
			snaps = append(snaps, truthSnap{
				street: h.Street(),
				board:  h.Board(),
				pot:    h.PotTotal(),
				toCall: h.ToCall(0),
				stacks: stacks,
			})
		}
		if err := h.Apply(a); err != nil {
			t.Fatalf("Apply(%v): %v", a, err)
		}
	}
	if h.Phase() != engine.PhaseComplete {
		t.Fatalf("fixture hand did not complete (phase %v)", h.Phase())
	}
	return h, snaps
}

// fixtureRecord wraps the fixture hand in a HandRecord the way the table
// screen would.
func fixtureRecord(t *testing.T) (engine.HandRecord, []truthSnap, *engine.Hand) {
	t.Helper()
	h, snaps := playFixtureHand(t)
	rec := NewRecord(h, "test-hand-1", time.Unix(0, 0).UTC(), []engine.SeatInfo{
		{Seat: 0, Name: "YOU", Personality: "", StartingStack: 1000},
		{Seat: 1, Name: "Vera", Personality: "tag", StartingStack: 1000},
		{Seat: 2, Name: "Nils", Personality: "nit", StartingStack: 1000},
	})
	return rec, snaps, h
}

// heroActionIndices returns the event indices of the hero's EvAction events
// — the keys HandAnnotations grades by.
func heroActionIndices(rec engine.HandRecord) []int {
	var out []int
	for i, e := range rec.Events {
		if e.Kind == engine.EvAction && e.Seat == 0 {
			out = append(out, i)
		}
	}
	return out
}

// TestReplayReconstructsDecisionsFromLogAlone: board, pot, stacks and price
// at every hero decision must match the engine's own view at that moment —
// proven from the persisted events only, with no live Hand available.
func TestReplayReconstructsDecisionsFromLogAlone(t *testing.T) {
	rec, snaps, _ := fixtureRecord(t)
	m := Replay(rec, HandAnnotations{HandID: rec.ID})

	if got, want := len(m.Decisions), len(snaps); got != want {
		t.Fatalf("decisions = %d, want %d", got, want)
	}
	for i, d := range m.Decisions {
		want := snaps[i]
		if d.Street != want.street {
			t.Errorf("decision %d: street %v, want %v", i, d.Street, want.street)
		}
		if engine.CardsString(d.Board) != engine.CardsString(want.board) {
			t.Errorf("decision %d: board %v, want %v", i, d.Board, want.board)
		}
		if d.PotBefore != want.pot {
			t.Errorf("decision %d: pot %v, want %v", i, d.PotBefore, want.pot)
		}
		if d.ToCall != want.toCall {
			t.Errorf("decision %d: toCall %v, want %v", i, d.ToCall, want.toCall)
		}
		if d.Stacks != want.stacks {
			t.Errorf("decision %d: stacks %v, want %v", i, d.Stacks, want.stacks)
		}
	}

	if m.HeroSeat != 0 {
		t.Errorf("hero seat = %d, want 0", m.HeroSeat)
	}
	if m.Button != 2 {
		t.Errorf("button = %d, want 2 (derived from deal order)", m.Button)
	}
}

// TestReplayReconstructsStreets: one frame per street with the pot and
// stacks at each street's close, before any payout touches them.
func TestReplayReconstructsStreets(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	m := Replay(rec, HandAnnotations{})

	wantPots := []engine.Chips{20, 60, 60, 140}
	wantBoard := []int{0, 3, 4, 5}
	if got := len(m.Streets); got != len(wantPots) {
		t.Fatalf("streets = %d, want %d", got, len(wantPots))
	}
	for i, f := range m.Streets {
		if f.Street != engine.Street(i) {
			t.Errorf("frame %d: street %v, want %v", i, f.Street, engine.Street(i))
		}
		if f.PotAfter != wantPots[i] {
			t.Errorf("frame %d: pot %v, want %v", i, f.PotAfter, wantPots[i])
		}
		if len(f.Board) != wantBoard[i] {
			t.Errorf("frame %d: %d board cards, want %d", i, len(f.Board), wantBoard[i])
		}
	}
	// River close: both live stacks are down their full 70.
	last := m.Streets[len(m.Streets)-1]
	if last.Stacks[0] != 930 || last.Stacks[1] != 930 || last.Stacks[2] != 1000 {
		t.Errorf("river-close stacks = %v, want 930/930/1000", last.Stacks[:3])
	}
	if !last.Folded.Has(2) {
		t.Error("seat 2 must be folded by the river frame")
	}
}

// TestReplayOutcome: awards, final stacks, and the winning five (board cards
// included) come straight from the log and eval.Best5.
func TestReplayOutcome(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	m := Replay(rec, HandAnnotations{})

	if !m.Outcome.Showdown {
		t.Fatal("fixture reaches showdown")
	}
	if m.Outcome.WinnerSeat != 0 {
		t.Fatalf("winner = %d, want hero", m.Outcome.WinnerSeat)
	}
	if m.Outcome.Stacks[0] != 1070 || m.Outcome.Stacks[1] != 930 {
		t.Errorf("final stacks = %v, want 1070/930", m.Outcome.Stacks[:2])
	}
	if len(m.Outcome.WinningFive) != 5 {
		t.Fatalf("winning five = %v, want 5 cards", m.Outcome.WinningFive)
	}
	// Kings and fives with the ace kicker: Kh Kd 5c 5d Ah — two of the five
	// are hero cards, three are board cards.
	want := engine.NewCardSet(engine.MustCards("Kh Kd 5c 5d Ah")...)
	if got := engine.NewCardSet(m.Outcome.WinningFive...); got != want {
		t.Errorf("winning five = %s, want %s", got, want)
	}
	if m.Outcome.WinnerDesc == "" {
		t.Error("winner description empty")
	}
	if m.Summary.HeroNet != 70 {
		t.Errorf("hero net = %v, want +70", m.Summary.HeroNet)
	}
	if m.Summary.HeroNetBB != 7.0 {
		t.Errorf("hero net bb = %v, want 7.0", m.Summary.HeroNetBB)
	}
}

// TestReplayRendersFrozenGradesVerbatim is the package's reason to exist:
// the grade values Replay is handed are the grade values it returns, byte
// for byte. This package knows the hole cards; using them to touch a grade
// would destroy the teaching claim, so the test feeds deliberately odd
// values no recomputation would reproduce.
func TestReplayRendersFrozenGradesVerbatim(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	idx := heroActionIndices(rec)
	if len(idx) != 4 {
		t.Fatalf("hero action events = %d, want 4", len(idx))
	}

	in := map[int]FrozenGrade{
		idx[0]: {Band: "Good", EVLossBB: 0, Body: "THEN-SENTINEL-ALPHA needed 25%, had ~31%", Recommended: "call"},
		idx[3]: {Band: "Blunder", EVLossBB: 3.7251, Body: "THEN-SENTINEL-BETA", Recommended: "check"},
	}
	m := Replay(rec, HandAnnotations{HandID: rec.ID, Grades: in})

	for i, d := range m.Decisions {
		want, graded := in[d.EventIdx]
		if !graded {
			if d.Frozen != nil {
				t.Errorf("decision %d: grade invented for an ungraded decision", i)
			}
			continue
		}
		if d.Frozen == nil {
			t.Fatalf("decision %d: frozen grade dropped", i)
		}
		gotJSON, _ := json.Marshal(*d.Frozen)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("decision %d: frozen grade rewritten:\n got %s\nwant %s", i, gotJSON, wantJSON)
		}
	}
}

// TestSummaryLedgerIsDecisionBased: EVLossBB sums the frozen losses and
// KeyDecision picks the largest — a hand the hero WON still shows the leak,
// because the ledger reads decisions, never results.
func TestSummaryLedgerIsDecisionBased(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	idx := heroActionIndices(rec)
	m := Replay(rec, HandAnnotations{Grades: map[int]FrozenGrade{
		idx[0]: {Band: "Good", EVLossBB: 0.5},
		idx[1]: {Band: "Mistake", EVLossBB: 1.25},
		idx[2]: {Band: "Best", EVLossBB: 0},
	}})

	if got, want := m.Summary.EVLossBB, 1.75; got != want {
		t.Errorf("EVLossBB = %v, want %v", got, want)
	}
	if got, want := m.Summary.KeyDecision, 1; got != want {
		t.Errorf("KeyDecision = %d, want %d (the 1.25bb leak)", got, want)
	}
	if m.Summary.Graded != 3 {
		t.Errorf("Graded = %d, want 3", m.Summary.Graded)
	}
	if m.Summary.GradeCounts["Mistake"] != 1 || m.Summary.GradeCounts["Good"] != 1 {
		t.Errorf("GradeCounts = %v", m.Summary.GradeCounts)
	}
	// The hero won 70 chips; the ledger must not care.
	if m.Summary.HeroNet <= 0 {
		t.Fatal("fixture hero should have won the hand")
	}

	// No grades at all: the ledger is empty, not zero-filled nonsense.
	empty := Replay(rec, HandAnnotations{})
	if empty.Summary.EVLossBB != 0 || empty.Summary.KeyDecision != -1 || empty.Summary.Graded != 0 {
		t.Errorf("ungraded summary = %+v, want empty ledger", empty.Summary)
	}
}

// TestHindsightLayer: revealed cards and true equity are computed with
// perfect information — and live in Hindsight, apart from any grade.
func TestHindsightLayer(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	m := Replay(rec, HandAnnotations{})

	// Preflop decision: seat 2 already folded, so the only live villain is
	// seat 1 with QsQd. AhKh vs QQ preflop is about 46%.
	d := m.Decisions[0]
	if len(d.Hindsight.VillainHands) != 1 {
		t.Fatalf("preflop hindsight villains = %v, want just seat 1", d.Hindsight.VillainHands)
	}
	if got := d.Hindsight.VillainHands[1]; got != engine.Holes("Qs Qd") {
		t.Errorf("villain hand = %v, want QsQd", got)
	}
	if !d.Hindsight.HasEquity || d.Hindsight.TrueEquity < 0.40 || d.Hindsight.TrueEquity > 0.52 {
		t.Errorf("preflop true equity = %v, want ~0.46", d.Hindsight.TrueEquity)
	}

	// Flop decision: top pair top kicker vs an underpair — hero is far ahead.
	f := m.Decisions[1]
	if !f.Hindsight.HasEquity || f.Hindsight.TrueEquity < 0.75 {
		t.Errorf("flop true equity = %v, want well ahead", f.Hindsight.TrueEquity)
	}
	if f.Hindsight.Note == "" {
		t.Error("hindsight note empty")
	}

	// Determinism: the same record annotates to the same numbers.
	again := Replay(rec, HandAnnotations{})
	if again.Decisions[0].Hindsight.TrueEquity != d.Hindsight.TrueEquity {
		t.Error("hindsight equity must be deterministic across replays")
	}
}

// TestQuadrantTemplates: all four decision/outcome quadrants produce
// distinct, sensible teaching lines — and the two counterintuitive quadrants
// carry their message.
func TestQuadrantTemplates(t *testing.T) {
	good := FrozenGrade{Band: "Good"}
	bad := FrozenGrade{Band: "Mistake"}

	notes := map[string]string{
		"good-won":  DecisionOutcomeNote(good, 4.5),
		"good-lost": DecisionOutcomeNote(good, -4.5),
		"bad-won":   DecisionOutcomeNote(bad, 40),
		"bad-lost":  DecisionOutcomeNote(bad, -40),
	}
	seen := map[string]string{}
	for name, note := range notes {
		if note == "" {
			t.Errorf("%s: empty template", name)
		}
		if prev, dup := seen[note]; dup {
			t.Errorf("%s and %s share a template: %q", name, prev, note)
		}
		seen[note] = name
	}

	// The two quadrants that matter most say exactly the hard thing.
	if want := "prints money"; !contains(notes["good-lost"], want) {
		t.Errorf("good-lost template %q must contain %q", notes["good-lost"], want)
	}
	if want := "still loses money"; !contains(notes["bad-won"], want) {
		t.Errorf("bad-won template %q must contain %q", notes["bad-won"], want)
	}
	// The winning-mistake line quotes the win so the contrast is concrete.
	if want := "40bb"; !contains(notes["bad-won"], want) {
		t.Errorf("bad-won template %q must quote the win (%s)", notes["bad-won"], want)
	}

	if !GoodBand("Best") || !GoodBand("good") || GoodBand("Inaccuracy") || GoodBand("Blunder") {
		t.Error("GoodBand must accept Best/Good only")
	}
}

// TestArchiveRoundTrip: a {record, annotations} pair survives the profile
// store's JSONL append/read cycle and replays to the identical model —
// frozen grades, hindsight numbers and all.
func TestArchiveRoundTrip(t *testing.T) {
	rec, _, _ := fixtureRecord(t)
	idx := heroActionIndices(rec)
	ann := HandAnnotations{
		HandID: rec.ID,
		Grades: map[int]FrozenGrade{
			idx[0]: {Band: "Good", EVLossBB: 0, Body: "needed 25%, had ~31%", Recommended: "call"},
			idx[1]: {Band: "Best", EVLossBB: 0, Body: "bet for value", Recommended: "bet 20"},
		},
	}

	store := profile.StoreAt(t.TempDir())
	day := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	if err := store.AppendHandRecord(day, Archive{Record: rec, Annotations: ann}); err != nil {
		t.Fatalf("AppendHandRecord: %v", err)
	}
	lines, err := store.LastHandRecords(1)
	if err != nil {
		t.Fatalf("LastHandRecords: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d records, want 1", len(lines))
	}
	var back Archive
	if err := json.Unmarshal(lines[0], &back); err != nil {
		t.Fatalf("unmarshal archive: %v", err)
	}

	direct, _ := json.Marshal(Replay(rec, ann))
	roundtrip, _ := json.Marshal(Replay(back.Record, back.Annotations))
	if string(direct) != string(roundtrip) {
		t.Error("replay of the round-tripped archive differs from the direct replay")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
