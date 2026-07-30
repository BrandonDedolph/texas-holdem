package trainer

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// TestNotationPolicy pins the one-spelling policy for teaching arithmetic
// (docs/ui-review.md F9): the coach's live prose and the trainer's answer
// screens must render the same rule-of-2/4 arithmetic byte-identically —
// "8×4 ≈ 32%", never "8 x 4 ~ 32%". The canonical spelling has a single
// home, coach.RuleShortcut; this test holds both packages to it for the
// same inputs, and sweeps generated sessions for the banned drift forms.
func TestNotationPolicy(t *testing.T) {
	report := equity.OutsReport{Count: 8, Discounted: 8, RuleOf4: 32, RuleOf2: 16}
	canonical := coach.RuleShortcut(report.Discounted, 4, report.RuleOf4)
	if canonical != "8×4 ≈ 32%" {
		t.Fatalf("canonical spelling = %q, want %q", canonical, "8×4 ≈ 32%")
	}

	// The trainer's outs and equity explanations quote the same arithmetic
	// in the same spelling.
	for name, text := range map[string]string{
		"outsExplain":   outsExplain(report, 3),
		"equityExplain": equityExplain(report, equity.Result{Equity: 0.31, Win: 0.31}, 3),
	} {
		if !strings.Contains(text, canonical) {
			t.Errorf("%s = %q, must contain the canonical %q", name, text, canonical)
		}
	}

	// The coach, explaining a decision that consumed the identical outs
	// report, spells it the same way.
	v := &engine.PlayerView{
		Seat:   0,
		Hole:   cards2(t, "9h", "8h"),
		Board:  []engine.Card{card(t, "7s"), card(t, "6d"), card(t, "2c")},
		Street: engine.Flop,
	}
	d := ai.Decision{Action: engine.Check{S: 0}}
	d.Rationale.Add(
		ai.ClassFact{Made: ai.Air, Draw: ai.OESD},
		ai.OutsFact{Report: report},
	)
	_, body, _ := coach.Explain(d, v)
	if !strings.Contains(body, canonical) {
		t.Errorf("coach body = %q, must contain the canonical %q", body, canonical)
	}

	// Half-out counts keep their decimal in both voices.
	half := equity.OutsReport{Count: 7, Discounted: 7.5, RuleOf4: 30, RuleOf2: 15}
	if got, want := coach.RuleShortcut(half.Discounted, 4, half.RuleOf4), "7.5×4 ≈ 30%"; got != want {
		t.Errorf("half-out spelling = %q, want %q", got, want)
	}

	// No generated session may leak the old drift spellings.
	banned := []string{" x 4", " x 2", " ~ "}
	for _, kind := range []QuizKind{QuizOuts, QuizEquity} {
		s := NewSessionSeeded(kind, profile.NewProfile(), 7)
		for i, it := range s.Items {
			for _, b := range banned {
				if strings.Contains(it.Drill.Explain, b) {
					t.Errorf("%v item %d explain %q contains banned spelling %q",
						kind, i, it.Drill.Explain, b)
				}
			}
		}
	}
}

// card parses one card code for test fixtures.
func card(t *testing.T, code string) engine.Card {
	t.Helper()
	cs, err := engine.ParseCards(code)
	if err != nil || len(cs) != 1 {
		t.Fatalf("bad card %q: %v", code, err)
	}
	return cs[0]
}

// cards2 parses a two-card holding.
func cards2(t *testing.T, a, b string) [2]engine.Card {
	t.Helper()
	return [2]engine.Card{card(t, a), card(t, b)}
}
