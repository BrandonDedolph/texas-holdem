package content

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// curriculumIDs is the design's 13-lesson table (design-learning.md §8.4),
// in order. The registry must contain exactly these.
var curriculumIDs = []string{
	"hand-rankings",
	"hand-flow",
	"kickers-ties",
	"position",
	"preflop-ranges",
	"facing-raises",
	"pot-odds",
	"outs-equity",
	"playing-draws",
	"why-we-bet",
	"bet-sizing",
	"reading-players",
	"bluffing",
}

// TestCurriculumIntegrity checks the registry's shape: all 13 lessons,
// unique IDs in design order, resolvable acyclic prerequisites forming a
// valid topological order, 3–6 sections each, at least one drill each, and
// a closing scripted hand on lessons 5–13.
func TestCurriculumIntegrity(t *testing.T) {
	all := tutorial.All()
	if len(all) != len(curriculumIDs) {
		t.Fatalf("registry has %d lessons, want %d", len(all), len(curriculumIDs))
	}
	for i, want := range curriculumIDs {
		l := all[i]
		if l.ID != want {
			t.Errorf("lesson %d is %q, want %q", i+1, l.ID, want)
		}
		if l.Order != i+1 {
			t.Errorf("lesson %q Order = %d, want %d", l.ID, l.Order, i+1)
		}
		if l.Title == "" || l.Goal == "" {
			t.Errorf("lesson %q missing title or goal", l.ID)
		}
	}

	byID := make(map[string]*tutorial.Lesson, len(all))
	for _, l := range all {
		if byID[l.ID] != nil {
			t.Fatalf("duplicate lesson ID %q", l.ID)
		}
		byID[l.ID] = l
	}

	// Prerequisites resolve, are acyclic, and respect Order (a lesson's
	// prerequisite always sorts before it — the curriculum sequence is a
	// valid topological order of the prerequisite DAG).
	for _, l := range all {
		for _, pre := range l.Prerequisites {
			p, ok := byID[pre]
			if !ok {
				t.Errorf("lesson %q requires unknown lesson %q", l.ID, pre)
				continue
			}
			if p.Order >= l.Order {
				t.Errorf("lesson %q (order %d) requires %q (order %d): not topological",
					l.ID, l.Order, pre, p.Order)
			}
		}
	}
	assertAcyclic(t, byID)

	// The design's branch: the preflop track (5→6) and the math track
	// (7→8→9) may interleave — 7 must not require the preflop track — and
	// lesson 10 joins them by requiring both.
	if got := byID["pot-odds"].Prerequisites; len(got) != 1 || got[0] != "position" {
		t.Errorf("pot-odds prerequisites = %v, want [position] (math track branches at 4)", got)
	}
	join := byID["why-we-bet"].Prerequisites
	if len(join) != 2 || !contains(join, "facing-raises") || !contains(join, "playing-draws") {
		t.Errorf("why-we-bet prerequisites = %v, want both tracks", join)
	}

	for _, l := range all {
		if n := len(l.Sections); n < 3 || n > 6 {
			t.Errorf("lesson %q has %d sections, want 3–6", l.ID, n)
		}
		if len(l.Drills()) == 0 {
			t.Errorf("lesson %q has no drill", l.ID)
		}
		for i, sec := range l.Sections {
			if err := sectionShape(sec); err != nil {
				t.Errorf("lesson %q section %d: %v", l.ID, i, err)
			}
		}
		if l.Order >= 5 {
			last := l.Sections[len(l.Sections)-1]
			if last.Kind != tutorial.SectionScripted || last.Script == nil {
				t.Errorf("lesson %q (order %d) does not end with a scripted hand", l.ID, l.Order)
			}
		}
	}
}

// sectionShape checks that exactly the field matching Kind is populated.
func sectionShape(s tutorial.Section) error {
	switch s.Kind {
	case tutorial.SectionText:
		if s.Text == "" || s.Visual != nil || s.Drill != nil || s.Script != nil {
			return fmt.Errorf("text section malformed")
		}
	case tutorial.SectionVisual:
		if s.Visual == nil || s.Drill != nil || s.Script != nil {
			return fmt.Errorf("visual section malformed")
		}
	case tutorial.SectionDrill:
		if s.Drill == nil || s.Visual != nil || s.Script != nil {
			return fmt.Errorf("drill section malformed")
		}
		if s.Drill.Prompt == "" || s.Drill.Answer == nil || s.Drill.Explain == "" {
			return fmt.Errorf("drill missing prompt, answer, or explanation")
		}
	case tutorial.SectionScripted:
		if s.Script == nil || s.Visual != nil || s.Drill != nil {
			return fmt.Errorf("scripted section malformed")
		}
		if s.Script.Intro == "" || s.Script.Debrief == "" || len(s.Script.Stops) == 0 {
			return fmt.Errorf("scripted hand missing intro, debrief, or stops")
		}
	default:
		return fmt.Errorf("unknown section kind %d", s.Kind)
	}
	return nil
}

func assertAcyclic(t *testing.T, byID map[string]*tutorial.Lesson) {
	t.Helper()
	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(byID))
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case visiting:
			return false
		case done:
			return true
		}
		state[id] = visiting
		for _, pre := range byID[id].Prerequisites {
			if _, ok := byID[pre]; !ok {
				continue // reported elsewhere
			}
			if !visit(pre) {
				return false
			}
		}
		state[id] = done
		return true
	}
	for id := range byID {
		if !visit(id) {
			t.Fatalf("prerequisite cycle through lesson %q", id)
		}
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestScriptsReplayThroughEngine replays every scripted hand in the
// curriculum through the real engine. A script that drifts out of sync with
// engine legality fails here — never at runtime in front of the learner.
func TestScriptsReplayThroughEngine(t *testing.T) {
	scripted := 0
	for _, l := range tutorial.All() {
		for i, s := range l.Scripts() {
			scripted++
			t.Run(fmt.Sprintf("%s/%d", l.ID, i), func(t *testing.T) {
				h, err := s.Replay()
				if err != nil {
					t.Fatalf("script does not replay: %v", err)
				}
				if _, ok := h.Result(); !ok {
					t.Fatal("replayed hand has no result")
				}
				if s.Hero != 0 {
					t.Errorf("hero seat = %d; curriculum scripts seat the hero at 0", s.Hero)
				}
			})
		}
	}
	if scripted < 9 {
		t.Errorf("curriculum has %d scripted hands, want one for each of lessons 5–13", scripted)
	}
}

// TestDrillFactsVerified recomputes every fact-backed drill answer against
// eval/equity. A drill that asserts a poker fact must carry a Fact, and the
// fact must agree with the stated answer — a lesson that teaches a wrong
// number is worse than no lesson.
func TestDrillFactsVerified(t *testing.T) {
	verified := 0
	for _, l := range tutorial.All() {
		for i, d := range l.Drills() {
			if d.Fact == nil {
				continue
			}
			verified++
			if err := d.Fact.Verify(d); err != nil {
				t.Errorf("lesson %q drill %d teaches a wrong fact: %v", l.ID, i, err)
			}
		}
	}
	// Poker-fact drills must stay fact-backed; if this count drops, someone
	// removed a Fact instead of fixing a drill.
	if verified < 12 {
		t.Errorf("only %d drills are fact-verified; the curriculum ships at least 12", verified)
	}
}

// TestDrillsAcceptTheirOwnAnswers feeds every drill its stated correct
// answer (and one obviously wrong one) through Answer.Check, catching
// out-of-range indices and malformed answers.
func TestDrillsAcceptTheirOwnAnswers(t *testing.T) {
	for _, l := range tutorial.All() {
		for i, d := range l.Drills() {
			in, err := correctInput(d.Answer)
			if err != nil {
				t.Errorf("lesson %q drill %d: %v", l.ID, i, err)
				continue
			}
			if !d.Answer.Check(in) {
				t.Errorf("lesson %q drill %d rejects its own answer %q", l.ID, i, in)
			}
			if d.Answer.Check("") {
				t.Errorf("lesson %q drill %d accepts the empty answer", l.ID, i)
			}
		}
	}
}

// correctInput renders an Answer's correct response as learner input.
func correctInput(a tutorial.Answer) (string, error) {
	switch v := a.(type) {
	case tutorial.ChoiceAnswer:
		if v.Correct < 0 || v.Correct >= len(v.Choices) {
			return "", fmt.Errorf("choice index %d out of range (%d choices)", v.Correct, len(v.Choices))
		}
		return strconv.Itoa(v.Correct + 1), nil
	case tutorial.NumericAnswer:
		return strconv.FormatFloat(v.Value, 'f', -1, 64), nil
	case tutorial.OrderAnswer:
		if len(v.Correct) != len(v.Items) {
			return "", fmt.Errorf("order answer sequences %d of %d items", len(v.Correct), len(v.Items))
		}
		parts := make([]string, len(v.Correct))
		for k, idx := range v.Correct {
			if idx < 0 || idx >= len(v.Items) {
				return "", fmt.Errorf("order position %d indexes %d, out of range", k, idx)
			}
			parts[k] = strconv.Itoa(idx + 1)
		}
		return strings.Join(parts, " "), nil
	}
	return "", fmt.Errorf("unknown answer type %T", a)
}

// TestVisualsRenderAt80Cols renders every visual in the curriculum at the
// app's 80-column floor: non-empty, and no line wider than 80 cells.
func TestVisualsRenderAt80Cols(t *testing.T) {
	seen := 0
	for _, l := range tutorial.All() {
		for i, v := range l.Visuals() {
			seen++
			out := v.Render(80)
			if strings.TrimSpace(out) == "" {
				t.Errorf("lesson %q visual %d renders empty", l.ID, i)
				continue
			}
			for n, line := range strings.Split(out, "\n") {
				if w := lipgloss.Width(line); w > 80 {
					t.Errorf("lesson %q visual %d line %d is %d cells wide", l.ID, i, n, w)
				}
			}
		}
	}
	if seen == 0 {
		t.Error("curriculum has no visuals")
	}
}

// wholeHandRE matches three or more consecutive card codes — a HAND spelled
// out in prose. One or two codes are legitimate references to cards a visual
// already draws; a run of three is a hand, and hands belong in a Visual (the
// ladder, a board, a showdown, an order drill's drawn items), never in text.
var wholeHandRE = regexp.MustCompile(`(?:\b(?:10|[2-9AKQJT])[cdhs]\b[,;:]? +){2}\b(?:10|[2-9AKQJT])[cdhs]\b`)

// TestProseNeverSpellsWholeHands is the authoring-side half of the app's
// rendered-frame guard: no lesson text, prompt, explanation, caption or
// pre-deal intro may spell out a full hand as codes. In-hand Teach/Debrief
// texts are exempt — they run beside a live table that is already drawing
// the cards they mention.
func TestProseNeverSpellsWholeHands(t *testing.T) {
	check := func(id, field, s string) {
		t.Helper()
		if m := wholeHandRE.FindString(s); m != "" {
			t.Errorf("lesson %q %s spells a whole hand %q in prose; draw it as cards instead",
				id, field, m)
		}
	}
	for _, l := range tutorial.All() {
		for i, sec := range l.Sections {
			at := fmt.Sprintf("section %d %s", i+1, sec.Kind)
			check(l.ID, at+" text", sec.Text)
			if sec.Visual != nil {
				check(l.ID, at+" caption", sec.Visual.Caption)
			}
			if d := sec.Drill; d != nil {
				check(l.ID, at+" prompt", d.Prompt)
				check(l.ID, at+" explanation", d.Explain)
				if d.Visual != nil {
					check(l.ID, at+" caption", d.Visual.Caption)
				}
			}
			if s := sec.Script; s != nil {
				check(l.ID, at+" intro", s.Intro)
			}
		}
	}
}

// TestTeachingRangesStayInBand pins the shapes of the taught ranges: the
// button RFI range must sit in the design's 38–46% band, and under the gun
// must stay a single-digit share.
func TestTeachingRangesStayInBand(t *testing.T) {
	btn := mustRange(RangeBTN).Percent()
	if btn < 38 || btn > 46 {
		t.Errorf("button opening range is %.1f%%, want 38–46%%", btn)
	}
	utg := mustRange(RangeUTG).Percent()
	if utg < 7 || utg > 12 {
		t.Errorf("UTG opening range is %.1f%%, want 7–12%%", utg)
	}
	threeBet := mustRange(Range3Bet).Percent()
	if threeBet < 2 || threeBet > 5 {
		t.Errorf("3-bet range is %.1f%%, want 2–5%%", threeBet)
	}
}

// TestCurriculumCompletesThroughProfile walks lesson 1 to completion the way
// the UI would — viewing sections and answering drills — then persists the
// profile and reloads it, checking the Learning Journey resumes at lesson 2.
func TestCurriculumCompletesThroughProfile(t *testing.T) {
	store := profile.StoreAt(t.TempDir())
	p, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	first, ok := tutorial.Get("hand-rankings")
	if !ok {
		t.Fatal("hand-rankings not registered")
	}
	if next := tutorial.Next(p); next != first {
		t.Fatalf("fresh profile resumes at %v, want hand-rankings", next)
	}

	pr := tutorial.NewProgress(first)
	for i, sec := range first.Sections {
		pr.View(i)
		if sec.Drill == nil {
			continue
		}
		in, err := correctInput(sec.Drill.Answer)
		if err != nil {
			t.Fatal(err)
		}
		if !pr.Answer(i, in) {
			t.Fatalf("section %d rejected the correct answer %q", i, in)
		}
	}
	if !pr.Record(p) {
		t.Fatal("completed lesson did not record")
	}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}

	p2, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Completed(p2) {
		t.Error("lesson completion did not survive the profile round trip")
	}
	next := tutorial.Next(p2)
	if next == nil || next.ID != "hand-flow" {
		t.Errorf("Next after lesson 1 = %v, want hand-flow", next)
	}
	if !next.Unlocked(p2) {
		t.Error("hand-flow locked with its prerequisite complete")
	}
}
