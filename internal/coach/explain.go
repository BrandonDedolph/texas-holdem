package coach

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Explain renders a Decision's rationale into the coach panel's three
// pieces: an imperative headline, a 2–4 sentence body, and the number
// chips. It is a pure function of (decision, view) with no randomness, so
// the same advice always reads the same — replays and tests depend on it.
//
// The truthfulness gate (DESIGN.md §1 principle 3): every sentence comes
// from a template bound to the fact kinds it cites, and a template whose
// facts are absent from the rationale simply cannot fire. Explain never
// computes an equity, a price, or an out count of its own — if the
// decision didn't consume the number, the explanation cannot show it.
// TestTemplateBindings enforces the binding template by template.
//
// Style rules (design-learning.md §5.3): one idea per sentence; every
// number paired with its human-computable path ("call 50 into 150 → need
// 25%", "8×4 ≈ 32%"); the hardest numbers repeated in chips rather than
// buried in prose; archetype notes appended as the final sentence.
func Explain(d ai.Decision, v *engine.PlayerView) (headline, body string, chips []NumberChip) {
	r := d.Rationale

	// The archetype note owns the final sentence when present.
	limit := 4
	var archNote string
	if f, ok := r.Get(ai.FactArchetype); ok {
		archNote = "Table read — " + f.(ai.ArchetypeFact).Note + "."
		limit = 3
	}

	var sentences []string
	used := map[ai.FactKind]bool{}
	for _, t := range templates {
		if len(sentences) >= limit {
			break
		}
		if !hasAll(r, t.needs) || anyUsed(used, t.keys) {
			continue
		}
		s := t.render(r, v)
		if s == "" {
			continue
		}
		sentences = append(sentences, s)
		for _, k := range t.keys {
			used[k] = true
		}
	}
	if archNote != "" {
		sentences = append(sentences, archNote)
	}

	return headlineFor(d.Action, v), strings.Join(sentences, " "), chipsFor(r)
}

// template is one candidate sentence, bound to the fact kinds it cites.
//
//   - needs: every kind the template's text may draw a number or claim
//     from; the template is skipped unless ALL are present. This binding
//     is the contract that keeps explanations truthful — render receives
//     the whole rationale but is tested (TestTemplateBindings) to cite
//     nothing outside its declared needs and the view's own givens.
//   - keys: redundancy markers; the template is skipped if a key was
//     already spent by an earlier sentence, and spends its keys when it
//     fires. This is how "price and equity in one comparison sentence"
//     suppresses the separate price and equity sentences.
type template struct {
	name   string
	needs  []ai.FactKind
	keys   []ai.FactKind
	render func(r ai.Rationale, v *engine.PlayerView) string
}

// templates in rendering priority order. The order encodes what matters
// most in a body capped at four sentences: the price-vs-equity comparison
// is the whole game when facing a bet; the chart is the whole game
// preflop; hand class before abstractions postflop.
var templates = []template{
	{
		name:  "price-vs-equity",
		needs: []ai.FactKind{ai.FactPotOdds, ai.FactEquity},
		keys:  []ai.FactKind{ai.FactPotOdds, ai.FactEquity},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			po := mustFact(r, ai.FactPotOdds).(ai.PotOddsFact)
			eq := mustFact(r, ai.FactEquity).(ai.EquityFact)
			return "You need " + pctStr(po.Required) + " to call (" +
				po.ToCall.String() + " into " + po.Pot.String() + ") and have about " +
				pctStr(eq.Equity) + " against his likely range."
		},
	},
	{
		name:  "chart",
		needs: []ai.FactKind{ai.FactChart},
		keys:  []ai.FactKind{ai.FactChart},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			cf := mustFact(r, ai.FactChart).(ai.ChartFact)
			if cf.InRange {
				return holeCombo(v.Hole) + " is in the " + cf.Chart + " chart."
			}
			return holeCombo(v.Hole) + " is not in the " + cf.Chart + " chart — folding here costs nothing."
		},
	},
	{
		name:  "price",
		needs: []ai.FactKind{ai.FactPotOdds},
		keys:  []ai.FactKind{ai.FactPotOdds},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			po := mustFact(r, ai.FactPotOdds).(ai.PotOddsFact)
			return "The price is " + equity.OddsToOne(po.ToCall, po.Pot) + " — " +
				equity.RequiredEquityText(po.ToCall, po.Pot) + "."
		},
	},
	{
		name:  "class",
		needs: []ai.FactKind{ai.FactClass},
		keys:  []ai.FactKind{ai.FactClass},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			cf := mustFact(r, ai.FactClass).(ai.ClassFact)
			if cf.Made == ai.Air {
				// A draw's story belongs to the outs sentence; bare air
				// has nothing worth a sentence of its own.
				if cf.Draw == ai.NoDraw {
					return "You have no pair and no draw."
				}
				return ""
			}
			// Describe decodes the hero's own hand — cards the hero can
			// see, in the teaching vocabulary ("Two Pair, Aces and Nines
			// with a Queen kicker"). The colon form reads correctly for
			// every category ("Your hand: Royal Flush", "Your hand: Pair
			// of Nines"), where "You have ..." would need articles.
			return "Your hand: " + eval.EvalHoldem(v.Hole, v.Board).Describe() + "."
		},
	},
	{
		name:  "outs",
		needs: []ai.FactKind{ai.FactOuts, ai.FactClass},
		keys:  []ai.FactKind{ai.FactOuts},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			of := mustFact(r, ai.FactOuts).(ai.OutsFact)
			cf := mustFact(r, ai.FactClass).(ai.ClassFact)
			if cf.Draw == ai.NoDraw || of.Report.Discounted <= 0 {
				return ""
			}
			n := outsStr(of.Report.Discounted)
			// The rule of 4 prices a flop draw over two cards; the rule
			// of 2 prices a turn draw over one. River draws don't exist,
			// and the strategy never emits an OutsFact there.
			if v.Street == engine.Turn {
				return "Your " + cf.Draw.String() + " has about " + n + " outs — rule of 2: " +
					n + "×2 ≈ " + roundPct(of.Report.RuleOf2) + "."
			}
			return "Your " + cf.Draw.String() + " has about " + n + " outs — rule of 4: " +
				n + "×4 ≈ " + roundPct(of.Report.RuleOf4) + "."
		},
	},
	{
		name:  "equity",
		needs: []ai.FactKind{ai.FactEquity},
		keys:  []ai.FactKind{ai.FactEquity},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			eq := mustFact(r, ai.FactEquity).(ai.EquityFact)
			return "You have about " + pctStr(eq.Equity) + " equity against his likely range."
		},
	},
	{
		name:  "implied-odds",
		needs: []ai.FactKind{ai.FactEquity},
		keys:  nil, // an addendum, not a competitor to the equity sentence
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			adj, ok := impliedEquity(r)
			if !ok {
				return ""
			}
			return "Money behind adds implied odds — count your draw as about " + pctStr(adj) + "."
		},
	},
	{
		name:  "sizing",
		needs: []ai.FactKind{ai.FactSizing},
		keys:  []ai.FactKind{ai.FactSizing},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			// Preflop sizes are multiples of the blind, not pot
			// fractions; the chart sentence carries preflop, so a pot
			// percentage here would only confuse.
			if v.Street == engine.Preflop {
				return ""
			}
			sf := mustFact(r, ai.FactSizing).(ai.SizingFact)
			return "The size is about " + pctStr(sf.FractionOfPot) + " of the pot — " +
				purposePhrase(sf.Purpose) + "."
		},
	},
	{
		name:  "range",
		needs: []ai.FactKind{ai.FactRange},
		keys:  []ai.FactKind{ai.FactRange},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			rf := mustFact(r, ai.FactRange).(ai.RangeFact)
			return "I put him on " + rf.Summary + "."
		},
	},
	{
		name:  "position",
		needs: []ai.FactKind{ai.FactPosition},
		keys:  []ai.FactKind{ai.FactPosition},
		render: func(r ai.Rationale, v *engine.PlayerView) string {
			pf := mustFact(r, ai.FactPosition).(ai.PositionFact)
			if pf.InPosition {
				return "From the " + pf.Pos.Long() + " you act last — position is money."
			}
			return "From the " + pf.Pos.Long() + " you act out of position."
		},
	},
}

// headlineFor renders the imperative recommendation, ≤40 chars.
func headlineFor(a engine.Action, v *engine.PlayerView) string {
	switch a.Type() {
	case engine.ActionFold:
		return "Fold"
	case engine.ActionCheck:
		return "Check"
	case engine.ActionCall:
		amt := v.Legal.CallAmount()
		if amt <= 0 {
			amt = v.ToCall
		}
		return "Call " + amt.String()
	case engine.ActionBet:
		amt, _ := actionAmount(a)
		return "Bet " + amt.String()
	default:
		amt, _ := actionAmount(a)
		return "Raise to " + amt.String()
	}
}

// chipsFor extracts the labelled numbers the panel shows beside the prose.
// Chips draw from the same facts the sentences do — a chip can no more
// invent a number than a template can.
func chipsFor(r ai.Rationale) []NumberChip {
	var chips []NumberChip
	if f, ok := r.Get(ai.FactPotOdds); ok {
		po := f.(ai.PotOddsFact)
		chips = append(chips, NumberChip{"Pot odds", equity.PotOddsText(po.ToCall, po.Pot)})
	}
	if f, ok := r.Get(ai.FactEquity); ok {
		eq := f.(ai.EquityFact)
		chips = append(chips, NumberChip{"Your equity", "~" + pctStr(eq.Equity)})
	}
	if f, ok := r.Get(ai.FactOuts); ok {
		if rep := f.(ai.OutsFact).Report; rep.Discounted > 0 {
			chips = append(chips, NumberChip{"Outs", outsStr(rep.Discounted)})
		}
	}
	return chips
}

// impliedEquity finds the stated implied-odds adjustment, if the decision
// made one. The strategy emits it as a second EquityFact with a
// distinguishing method string, never as a silent bump.
func impliedEquity(r ai.Rationale) (float64, bool) {
	for _, f := range r.Facts {
		if eq, ok := f.(ai.EquityFact); ok && eq.Method == "implied-odds credit" {
			return eq.Equity, true
		}
	}
	return 0, false
}

// purposePhrase expands a SizingFact purpose into the teaching clause.
func purposePhrase(purpose string) string {
	switch purpose {
	case "value":
		return "a value bet, charging worse hands"
	case "semi-bluff":
		return "a semi-bluff: win it now, or improve when called"
	case "bluff":
		return "a bluff, and it only works if he can fold"
	default:
		return purpose
	}
}

// holeCombo renders hole cards in range notation: "AJs", "98o", "TT".
func holeCombo(hole [2]engine.Card) string {
	hi, lo := hole[0].Rank(), hole[1].Rank()
	if hi < lo {
		hi, lo = lo, hi
	}
	s := string(hi.Letter()) + string(lo.Letter())
	switch {
	case hi == lo:
		return s
	case hole[0].Suit() == hole[1].Suit():
		return s + "s"
	default:
		return s + "o"
	}
}

// pctStr renders a 0..1 fraction as a whole percentage: "31%". Whole
// points only — the coach quotes "~31%", and false precision would teach
// the wrong lesson about how exactly these numbers can be known.
func pctStr(x float64) string { return strconv.Itoa(pctRound(x)) + "%" }

func pctRound(x float64) int { return int(x*100 + 0.5) }

// roundPct rounds an already-scaled percentage (RuleOf4 is "per hundred").
func roundPct(x float64) string { return strconv.Itoa(int(x+0.5)) + "%" }

// outsStr renders a discounted out count: "8", or "8.5" when tainted outs
// contribute a half.
func outsStr(discounted float64) string {
	return strconv.FormatFloat(discounted, 'f', -1, 64)
}

func hasAll(r ai.Rationale, kinds []ai.FactKind) bool {
	for _, k := range kinds {
		if !r.Has(k) {
			return false
		}
	}
	return true
}

func anyUsed(used map[ai.FactKind]bool, kinds []ai.FactKind) bool {
	for _, k := range kinds {
		if used[k] {
			return true
		}
	}
	return false
}

// mustFact fetches a fact the template declared in needs; Explain's scan
// guarantees presence, so a miss here is a template wiring bug.
func mustFact(r ai.Rationale, k ai.FactKind) ai.Fact {
	f, ok := r.Get(k)
	if !ok {
		panic("coach: template cited undeclared fact kind " + k.String())
	}
	return f
}
