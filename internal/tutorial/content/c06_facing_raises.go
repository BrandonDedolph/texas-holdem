package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// Range3Bet is the simple value 3-betting range taught in lesson 6, ~3% of
// hands.
const Range3Bet = "JJ+, AQs+, AKo"

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "facing-raises",
		Title:         "When Someone Raises First",
		Goal:          "3-bet / call / fold logic, and why dominated hands (KJ vs a raiser) bleed money.",
		Order:         6,
		Prerequisites: []string{"preflop-ranges"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "The bar just went up",
				Text: "Your opening chart assumed everyone before you folded. When " +
					"someone raises first, that assumption is gone: they claim a range " +
					"of real hands, and yours has to measure up against THEIRS, not " +
					"against random cards.\n\n" +
					"Three options, in the order you should consider them:\n\n" +
					"- 3-bet (re-raise): your hand plays great against their range — " +
					"big pairs, AK, AQs. Raise about 3x their open.\n" +
					"- Fold: the default. Most of the hands you'd happily open are not " +
					"good enough to continue against a raise.\n" +
					"- Call: the in-between, mostly in position or from the big blind " +
					"(where your blind discounts the price). Pairs hoping to flop a " +
					"set, suited broadways, good suited connectors.",
			},
			{
				Kind:  tutorial.SectionText,
				Title: "Domination: the silent leak",
				Text: "Why is KJo — a hand you'd open from most seats — a fold against " +
					"an early raise? Because raising ranges are full of AK, AQ, AJ, KQ " +
					"and big pairs, and against those, KJ is DOMINATED: when you flop a " +
					"king, AK has you outkicked; when you flop a jack, AJ has you " +
					"outkicked. Your good flops are their better flops.\n\n" +
					"Dominated hands don't lose spectacularly — they win small pots and " +
					"lose big ones, which is worse. The drill below puts a number on it.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "The raiser turns out to have AQ — a typical raising hand " +
						"that isn't even a pair yet. About what percent of the time " +
						"does your KJo win the pot? (within 5)",
					Visual: &tutorial.Visual{
						Showdown: &tutorial.VisualShowdown{
							Hands: []tutorial.ShownHand{
								{Label: "You", Hole: engine.Holes("Kh Jc")},
								{Label: "The raiser", Hole: engine.Holes("Ad Qs")},
							},
						},
					},
					Answer: tutorial.NumericAnswer{Value: 37, Tolerance: 5},
					Explain: "About 37%. Two live-looking broadway cards, and you're " +
						"still a 63:37 underdog before anyone has a pair — because when " +
						"both hands hit, AQ outkicks you, and its high cards win unimproved " +
						"showdowns. Against the raiser's whole range (which also has AK, " +
						"KQ, and big pairs that crush you) it gets uglier. That's " +
						"domination in one number.",
					Fact: tutorial.EquityFact{
						Hero:    "Kh Jc",
						Villain: "Ad Qs",
					},
				},
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "A simple 3-betting range",
				Text: "To start, 3-bet only for value: JJ+, AQs+, and AKo. It's about " +
					"3% of hands — tiny, honest, and profitable, because most opponents " +
					"call 3-bets with worse. Bluffy, \"balanced\" 3-betting is real but " +
					"it's a later skill; a beginner 3-betting only monsters makes money " +
					"and simplifies every following street.",
				Visual: &tutorial.Visual{
					RangeGrid: &tutorial.VisualRangeGrid{
						Range: mustRange(Range3Bet),
						Title: "Starter 3-bet range (~3% of hands)",
					},
					Caption: "3-bet these, fold most of the rest, call with the select middle.",
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: fold the pretty hand",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     4,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks6(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("Kc Jd"),
						4: engine.Holes("Ah Qs"),
					},
					Seed: 601,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You", engine.Fold{S: 0}),
						seat(1, "Rex", engine.Fold{S: 1}),
						seat(2, "Sana", engine.Fold{S: 2}),
						seat(3, "Bo", engine.Fold{S: 3}),
						seat(4, "Nia", engine.Raise{S: 4, To: 25}),
						seat(5, "Ivan", engine.Fold{S: 5}),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 0,
							Teach: "The button raised and you're in the big blind with " +
								"KJo. It LOOKS strong — two paint cards! — but a raiser's " +
								"range is heavy with AK, AQ, KQ, and pairs, and KJo is " +
								"dominated by nearly all of it. You'd be building pots for " +
								"the hands that outkick you. Let it go; the discount from " +
								"your blind isn't enough to fix domination.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Fold{S: 0},
						},
					},
					Intro: "Six-handed, blinds 5/10. You post the big blind and pick up " +
						"Kc Jd. The button — who as you now know opens plenty — raises to 25.",
					Debrief: "Nia had Ah Qs: exactly the domination scenario. Had the " +
						"flop come king-high or jack-high you'd have been out-kicked into " +
						"a big losing pot. Folding KJo preflop didn't cost you this pot — " +
						"it saved you the several bigger ones that follow.",
				},
			},
		},
	})
}
