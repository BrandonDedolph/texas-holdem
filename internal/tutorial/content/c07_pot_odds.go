package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "pot-odds",
		Title:         "The Price of a Call",
		Goal:          "Compute required equity from pot and bet size in your head.",
		Order:         7,
		Prerequisites: []string{"position"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Every call has a price tag",
				Text: "When someone bets, they're offering you a deal: risk the call " +
					"amount to win what's in the middle. Whether the deal is good " +
					"depends on one number — how often you need to win to break even:\n\n" +
					"    required equity = call / (pot + call)\n\n" +
					"where \"pot\" is everything out there already, their bet included. " +
					"Call 50 into a pot of 150? You're paying 50 to play for 200 total, " +
					"so you need 50/200 = 25%.\n\n" +
					"That's the whole formula. If your chance of winning beats the " +
					"price, calling makes money IN THE LONG RUN even when this " +
					"particular call loses. The pot is laying you odds; you're just " +
					"deciding if they're fair.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "One picture of a price",
				Text: "Pot 45, you must call 20. Players say it as a ratio — " +
					"\"2.2 to 1\" — but the number you'll actually use is the " +
					"percentage: 20/(45+20) = 31%. Win more often than 31% and this " +
					"call prints money.",
				Visual: &tutorial.Visual{
					PotOdds: &tutorial.VisualPotOdds{Pot: 45, ToCall: 20},
					Caption: "Same price, two spellings: the ratio you say, the percent you compare.",
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "The table you'll memorize by accident",
				Text: "Bets come in pot fractions, so the same prices recur all day. " +
					"Learn these four and you can price almost any bet at a glance:\n\n" +
					"- Half-pot bet: you need 25%\n" +
					"- Two-thirds pot: you need ~29%\n" +
					"- Full pot: you need ~33%\n" +
					"- A third of the pot: you need 20%\n\n" +
					"Worth noticing: even a FULL-POT bet only asks you to win a third " +
					"of the time. Prices in poker are more forgiving than they feel — " +
					"which is exactly why folding every time you're unsure is its own " +
					"leak. Next lesson gives you the other half: how to estimate your " +
					"winning chances so you have something to compare against the price.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "The pot is 100. Your opponent bets 50, making it 150. What " +
						"percent of the time must you win for a call to break even?",
					Answer: tutorial.NumericAnswer{Value: 25, Tolerance: 1},
					Explain: "call / (pot + call) = 50 / (150 + 50) = 50/200 = 25%. " +
						"Do the sum in this order every time: what's out there (150), " +
						"what it costs (50), cost over total (200). A half-pot bet is " +
						"always 25% — this is the price you'll see most often.",
					Fact: tutorial.PotOddsFact{Pot: 150, Call: 50},
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Messier numbers, same recipe: the pot is 45 and you face a " +
						"bet of 20 (already counted in the 45). Required win percentage?",
					Answer: tutorial.NumericAnswer{Value: 31, Tolerance: 1},
					Explain: "20 / (45 + 20) = 20/65 ≈ 31%. Real pots are never round, " +
						"so approximate fearlessly at the table: \"20 into about 65 — " +
						"call it 30-ish percent\" is plenty accurate to make the right " +
						"decision, and you can do it in three seconds.",
					Fact: tutorial.PotOddsFact{Pot: 45, Call: 20},
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: pay the right price",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     1,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("9h 8h"),
						1: engine.Holes("Ac Ts"),
					},
					Board: engine.MustCards("Th 7c 2d 2c 3s"),
					Seed:  701,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Check{S: 0}, // preflop option
							engine.Check{S: 0}, // flop
							engine.Call{S: 0},  // flop, facing 20
							engine.Check{S: 0}, // turn
							engine.Check{S: 0}, // river
						),
						seat(1, "Rex",
							engine.Call{S: 1}, // limp
							engine.Bet{S: 1, Amount: 20},
							engine.Check{S: 1},
							engine.Check{S: 1},
						),
						seat(2, "Sana", engine.Fold{S: 2}),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 0,
							Teach: "Rex limped, the small blind folded, and you're in " +
								"the big blind with 9h 8h. Nobody raised, so you close the " +
								"action — check and see a free flop. Free cards with a " +
								"connected suited hand are a gift.",
							Expect: engine.Check{S: 0},
						},
						{
							AtDecision: 2,
							Teach: "Rex bets 20 into 45. Price check: 20 / (45 + 20) = " +
								"31%. Your 9-8 picked up an open-ended straight draw — any " +
								"six or jack completes it — which wins often enough to beat " +
								"31% (next lesson teaches you to count it: 8 outs, roughly " +
								"32%). The price is fair. Call.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Call{S: 0},
						},
					},
					Intro: "Three-handed, blinds 5/10. You'll defend your big blind with " +
						"a suited connector and face a flop bet. Bring the formula.",
					Debrief: "The draw missed and Rex's pair of tens took it — and the " +
						"call was still right. You paid 31% for a hand that wins about " +
						"32% of the time; make that trade forever and the money flows to " +
						"you. Judge every call by the price you paid, never by the river.",
				},
			},
		},
	})
}
