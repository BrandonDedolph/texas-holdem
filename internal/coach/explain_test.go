package coach

import (
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/ai/rulebased"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// TestExplainTruthfulness is the explanation-honesty gate (DESIGN.md §3):
// over many real decisions, every number that appears anywhere in the coach
// output — headline, body, chips — must be derivable from the rationale
// facts the decision consumed, or from the hero's own view (their cards,
// the pot, the amounts on offer). A template that computes or invents a
// figure of its own fails here.
func TestExplainTruthfulness(t *testing.T) {
	if testing.Short() {
		t.Skip("full-hand simulation")
	}
	table := engine.NewTable(cfg6)
	table.Eval = eval.Standard{}
	bots := make(map[engine.Seat]*rulebased.AI, 6)
	for s := engine.Seat(0); s < 6; s++ {
		bots[s] = rulebased.New("bot", ai.Baseline(), int64(s)*17+3)
		if err := table.Sit(s, "bot", 1000); err != nil {
			t.Fatalf("sit: %v", err)
		}
	}
	coach := New(nil, 5)

	checked := 0
	for i := 0; i < 15; i++ {
		h, err := table.StartHand(engine.NewDeck(uint64(40000 + i*101)))
		if err != nil {
			t.Fatalf("hand %d: %v", i, err)
		}
		for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != engine.NoSeat {
			seat := h.CurrentSeat()
			v := h.View(seat)
			adv := coach.Advise(v)
			checked++

			allowed := allowedTokens(adv, v)
			out := adv.Headline + " " + adv.Body
			for _, chip := range adv.Numbers {
				out += " " + chip.Value
			}
			for _, tok := range numberTokens(out) {
				if !allowed[tok] {
					t.Fatalf("hand %d %s: output cites %q, not derivable from the rationale\noutput: %s\nfacts: %+v",
						i, v.Street, tok, out, adv.Decision.Rationale.Facts)
				}
			}
			if n := sentenceCount(adv.Body); n > 4 {
				t.Fatalf("body has %d sentences (max 4): %q", n, adv.Body)
			}
			if len(adv.Headline) > 40 {
				t.Fatalf("headline over 40 chars: %q", adv.Headline)
			}

			mustApply(t, h, adv.Decision.Action)
		}
		if err := table.FinishHand(h); err != nil {
			t.Fatalf("finish: %v", err)
		}
		for s := engine.Seat(0); s < 6; s++ {
			if missing := 1000 - table.Stack(s); missing > 0 {
				if err := table.Rebuy(s, missing); err != nil {
					t.Fatalf("rebuy: %v", err)
				}
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d decisions checked; too small to mean anything", checked)
	}
	t.Logf("verified every number over %d decisions", checked)
}

// TestTemplateBindings enforces the per-template contract: a template may
// cite only numbers derivable from the fact kinds it declared in needs
// (plus the view's own givens). This is what fails if someone edits a
// template to quote, say, an equity while declaring only pot odds.
func TestTemplateBindings(t *testing.T) {
	v := heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0)

	// A full rationale with distinctive values, so cross-fact leakage
	// cannot hide behind coincidence.
	full := ai.Rationale{}
	full.Add(
		ai.PotOddsFact{ToCall: 50, Pot: 150, Required: 0.25},
		ai.EquityFact{Equity: 0.31, Method: "exact"},
		ai.EquityFact{Equity: 0.35, Method: "implied-odds credit"},
		ai.OutsFact{Report: equity.OutsReport{Count: 8, Discounted: 8.5, RuleOf4: 34, RuleOf2: 17}},
		ai.PositionFact{Pos: engine.PosBTN, InPosition: true},
		ai.RangeFact{Summary: "≈15% range (opened from UTG)"},
		ai.ChartFact{Chart: "BTN open", InRange: true},
		ai.ClassFact{Made: ai.Air, Draw: ai.FlushDraw},
		ai.ArchetypeFact{Note: "calling station: don't bluff"},
		ai.SizingFact{FractionOfPot: 0.66, Purpose: "value"},
		ai.InitiativeFact{HeroIsAggressor: true},
	)

	for _, tpl := range templates {
		out := tpl.render(full, v)
		if out == "" {
			continue
		}
		// Allowed: only the declared facts, plus the view's givens.
		allowed := viewTokens(v)
		for _, k := range tpl.needs {
			for _, f := range full.Facts {
				if f.Kind() == k {
					addFactTokens(allowed, f)
				}
			}
		}
		// The implied-odds addendum reads the second EquityFact; it
		// declares FactEquity, which covers both. Nothing special needed.
		for _, tok := range numberTokens(out) {
			if !allowed[tok] {
				t.Errorf("template %q cites %q outside its declared facts %v: %q",
					tpl.name, tok, tpl.needs, out)
			}
		}
		if len(tpl.needs) == 0 {
			t.Errorf("template %q declares no facts — nothing binds its text to the decision", tpl.name)
		}
	}
}

// TestExplainOmitsUnconsumedFacts: a rationale without an outs fact must
// produce a body with no outs talk, and one without equity no equity talk.
// The needs-scan makes this structural; this test keeps it that way.
func TestExplainOmitsUnconsumedFacts(t *testing.T) {
	v := heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0)

	r := ai.Rationale{}
	r.Add(ai.PotOddsFact{ToCall: 25, Pot: 75, Required: 0.25})
	d := ai.Decision{
		Action:     engine.Call{S: 0},
		Candidates: []ai.ScoredAction{sa(engine.Call{S: 0}, 1)},
		Rationale:  r,
	}
	_, body, chips := Explain(d, v)
	for _, banned := range []string{"outs", "equity against", "rule of"} {
		if strings.Contains(body, banned) {
			t.Errorf("body mentions %q without the backing fact: %q", banned, body)
		}
	}
	if len(chips) != 1 || chips[0].Label != "Pot odds" {
		t.Errorf("chips should carry pot odds only: %+v", chips)
	}
}

// TestExplainFlushDrawSpot pins the teaching voice on the canonical spot:
// the price, the equity, the outs arithmetic, all paired with their
// human-computable paths.
func TestExplainFlushDrawSpot(t *testing.T) {
	v := heroView(t, flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 25, 5), 0)
	adv := New(nil, 3).Advise(v)

	if adv.Headline != "Call 25" {
		t.Errorf("headline %q, want %q", adv.Headline, "Call 25")
	}
	body := adv.Body
	if !strings.Contains(body, "You need 25% to call (25 into 75)") {
		t.Errorf("body lacks the pot-odds derivation: %q", body)
	}
	if !strings.Contains(body, "flush draw") || !strings.Contains(body, "rule of 4") {
		t.Errorf("body lacks the outs lesson: %q", body)
	}
	var labels []string
	for _, chip := range adv.Numbers {
		labels = append(labels, chip.Label)
	}
	if got := strings.Join(labels, ","); got != "Pot odds,Your equity,Outs" {
		t.Errorf("chips %v", labels)
	}
	if adv.Numbers[0].Value != equity.PotOddsText(25, 75) {
		t.Errorf("pot odds chip %q", adv.Numbers[0].Value)
	}
}

// TestExplainArchetypeNoteLast: when the decision consumed an archetype
// read, it is the final sentence.
func TestExplainArchetypeNoteLast(t *testing.T) {
	h := bustedComboRiverSpot(t, 11)
	v := heroView(t, h, 0)
	c := New(nil, 4)
	c.SetRead(1, ai.Archetypes["station"])
	adv := c.Advise(v)

	if !adv.Decision.Rationale.Has(ai.FactArchetype) {
		t.Fatal("spot did not consume an archetype fact; test is mis-built")
	}
	if !strings.HasSuffix(adv.Body, "Table read — calling station: don't bluff.") {
		t.Errorf("archetype note is not the final sentence: %q", adv.Body)
	}
	if adv.Decision.Action.Type() != engine.ActionCheck {
		t.Errorf("coach bluffed a station: %v", adv.Decision.Action.Type())
	}
}

// --- Shared derivation helpers ------------------------------------------------

var numberRe = regexp.MustCompile(`\d+(?:\.\d+)?`)

// numberTokens extracts every numeric token from output text. Thousands
// separators are stripped first so "1,240" round-trips as one number.
func numberTokens(s string) []string {
	return numberRe.FindAllString(strings.ReplaceAll(s, ",", ""), -1)
}

func addTokens(set map[string]bool, ss ...string) {
	for _, s := range ss {
		for _, tok := range numberTokens(s) {
			set[tok] = true
		}
	}
}

// addFactTokens adds every number a template is entitled to quote from one
// fact, in the exact formats the templates use. This mirrors — and thereby
// pins — the derivation rules of design-learning.md §5.3.
func addFactTokens(set map[string]bool, f ai.Fact) {
	switch f := f.(type) {
	case ai.PotOddsFact:
		addTokens(set,
			f.ToCall.String(), f.Pot.String(), pctStr(f.Required),
			equity.OddsToOne(f.ToCall, f.Pot),
			equity.PotOddsText(f.ToCall, f.Pot),
			equity.RequiredEquityText(f.ToCall, f.Pot),
		)
	case ai.EquityFact:
		addTokens(set, pctStr(f.Equity))
	case ai.OutsFact:
		addTokens(set,
			strconv.Itoa(f.Report.Count),
			strconv.Itoa(len(f.Report.Tainted)),
			OutsText(f.Report.Discounted),
			PctText(f.Report.RuleOf4),
			PctText(f.Report.RuleOf2),
			"4", "2", // the rule-of-4-and-2 multipliers themselves
		)
	case ai.SizingFact:
		addTokens(set, pctStr(f.FractionOfPot), f.Purpose)
	case ai.RangeFact:
		addTokens(set, f.Summary)
	case ai.ChartFact:
		addTokens(set, f.Chart)
	case ai.ArchetypeFact:
		addTokens(set, f.Note)
	}
}

// viewTokens is the hero's own givens: their cards, the board, the pot,
// the price, the blinds, and the amounts legally on offer. These are what
// the hero can read off the table without the coach — quoting them is not
// invention.
func viewTokens(v *engine.PlayerView) map[string]bool {
	set := map[string]bool{}
	addTokens(set, v.ToCall.String(), v.Pot.String(),
		v.Blinds.Small.String(), v.Blinds.Big.String(),
		holeCombo(v.Hole),
		engine.CardsString(v.Hole[:]), engine.CardsString(v.Board))
	if v.Legal != nil {
		addTokens(set, v.Legal.CallAmount().String())
	}
	return set
}

// allowedTokens is the full sweep-test allowance: fact-derived numbers
// plus view givens plus the amounts of the scored candidates (the headline
// quotes the chosen action's size, which is one of them).
func allowedTokens(adv Advice, v *engine.PlayerView) map[string]bool {
	set := viewTokens(v)
	for _, f := range adv.Decision.Rationale.Facts {
		addFactTokens(set, f)
	}
	for _, cand := range adv.Decision.Candidates {
		if amt, ok := actionAmount(cand.Action); ok {
			addTokens(set, amt.String())
		}
	}
	if amt, ok := actionAmount(adv.Decision.Action); ok {
		addTokens(set, amt.String())
	}
	return set
}

// sentenceCount counts sentences the way the templates write them: a
// period ends a sentence; decimals and ratios ("2.2:1") do not.
func sentenceCount(body string) int {
	if body == "" {
		return 0
	}
	return strings.Count(body, ". ") + 1
}

// TestDrawNameCarriesItsTextbookCount guards the fact a learner memorizes.
// The discounted total folds in pair outs and half-credits tainted ones, so
// "your gutshot has about 8.5 outs" would teach a wrong canonical number for
// the exact concept it just named.
func TestDrawNameCarriesItsTextbookCount(t *testing.T) {
	cases := []struct {
		draw ai.DrawClass
		outs int
	}{
		{ai.Gutshot, 4},
		{ai.OESD, 8},
		{ai.FlushDraw, 9},
		{ai.ComboDraw, 15},
	}
	for _, tc := range cases {
		got, ok := canonicalOuts(tc.draw)
		if !ok {
			t.Errorf("%v has no canonical count", tc.draw)
			continue
		}
		if got != tc.outs {
			t.Errorf("%v canonical outs = %d, want %d", tc.draw, got, tc.outs)
		}
	}
	if _, ok := canonicalOuts(ai.NoDraw); ok {
		t.Error("NoDraw should have no canonical count")
	}
}

// TestOutsSentenceNeverMisattributes: whenever the body names a draw AND a
// count, the count next to the draw's name must be that draw's textbook
// figure, never the discounted total.
func TestOutsSentenceNeverMisattributes(t *testing.T) {
	c := New(profile.NewProfile(), 42)
	spots := []*engine.PlayerView{
		// Hero holds 9s8s in all three: a bare flush draw, an open-ender
		// (7-8-9-10), and a gutshot (8-9-J-Q needs the ten).
		heroView(t, flushDrawFacingBet(t, "As 7s 2h", "Kd Qc", 25, 3), 0),
		heroView(t, flushDrawFacingBet(t, "7h Td 2c", "Kd Qc", 25, 8), 0),
		heroView(t, flushDrawFacingBet(t, "Jh Qd 2c", "Kd 5c", 25, 4), 0),
	}
	for i, v := range spots {
		body := c.Advise(v).Body
		for draw, outs := range map[string]string{
			"gutshot": "4", "open-ended straight draw": "8", "flush draw": "9",
		} {
			// "A <draw> is N outs" is the only form allowed to put a number
			// immediately after a draw name.
			bad := "Your " + draw + " has about "
			if idx := strings.Index(body, bad); idx >= 0 {
				rest := body[idx+len(bad):]
				num := rest[:strings.IndexByte(rest, ' ')]
				if num != outs {
					t.Errorf("spot %d: body attributes %s outs to a %s (textbook %s): %q",
						i, num, draw, outs, body)
				}
			}
		}
	}
}
