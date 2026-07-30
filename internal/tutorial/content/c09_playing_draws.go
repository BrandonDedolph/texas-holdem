package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "playing-draws",
		Title:         "Playing Draws",
		Goal:          "Call, fold, or semi-bluff a draw based on price and fold equity.",
		Order:         9,
		Prerequisites: []string{"outs-equity"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Draws are a price question",
				Text: "You now hold both halves of the machine: the pot quotes a " +
					"price (lesson 7), your outs quote an equity (lesson 8). A draw is " +
					"just the spot where you run the comparison honestly:\n\n" +
					"- Equity clearly beats the price: continue.\n" +
					"- Price clearly beats your equity: fold — no matter how pretty " +
					"the draw looks.\n" +
					"- Close: lean on position and implied odds — the extra chips " +
					"you'll win AFTER you hit. A hidden straight gets paid; the " +
					"third flush card scares everyone, so flushes collect less.\n\n" +
					"And a warning that saves stacks: the SAME draw has a different " +
					"answer on different streets. The rule of 4 becomes the rule of 2 " +
					"on the turn — your 36% draw quietly becomes an 18% draw, and a " +
					"call that was automatic on the flop becomes a clear fold.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Open-ended on the flop. How many clean outs? (exact)",
					Visual: &tutorial.Visual{
						Board: &tutorial.VisualBoard{
							Board:    engine.MustCards("10h 7c 2d"),
							Hole:     engine.Holes("9h 8h"),
							ShowHole: true,
						},
					},
					Answer: tutorial.NumericAnswer{Value: 8},
					Explain: "Eight: four jacks and four sixes complete the straight. " +
						"Pairing your 9 or 8 is NOT counted — a middle pair doesn't beat " +
						"the top-pair hands betting into you. Flop: 8 × 4 ≈ 32%. Turn: " +
						"8 × 2 ≈ 16%. Same cards, half the hope.",
					Fact: tutorial.OutsFact{Hero: "9h 8h", Board: "Th 7c 2d"},
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Turn spot: the pot is 175 after your opponent bets 60. " +
						"What win percentage does a call require? (within 1)",
					Answer: tutorial.NumericAnswer{Value: 26, Tolerance: 1},
					Explain: "60 / (175 + 60) = 60/235 ≈ 26%. Now compare: a flush " +
						"draw on the TURN is 9 × 2 ≈ 18%. 18% equity against a 26% " +
						"price is a fold — and this exact spot, calling turn barrels " +
						"with one card to come, is among the most expensive habits in " +
						"low-stakes poker. The scripted hand walks you through it.",
					Fact: tutorial.PotOddsFact{Pot: 175, Call: 60},
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "The third option: semi-bluff",
				Text: "Calling isn't a draw's only line. When YOU bet or raise with a " +
					"draw, two ways to win appear: they fold now, or you hit later. " +
					"That's the semi-bluff, and it's why aggressive draws are so " +
					"strong — a flush draw that bets needs far less than 36% to " +
					"profit, because fold equity pays the difference.\n\n" +
					"When to prefer it: in position, against opponents who CAN fold, " +
					"and with draws to the best hand (nut flush draws, open-enders on " +
					"unpaired boards). When to just call: against players who never " +
					"fold — a semi-bluff's first way to win is worthless if they " +
					"always call. Lesson 13 finishes this thought.",
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: same draw, different streets",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     1,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("9s 8s"),
						1: engine.Holes("Ad Qd"),
					},
					Board: engine.MustCards("As Ks 2h 4d"),
					Seed:  901,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Call{S: 0},  // defend the big blind
							engine.Check{S: 0}, // flop
							engine.Call{S: 0},  // flop, facing 30
							engine.Check{S: 0}, // turn
							engine.Fold{S: 0},  // turn, facing 60
						),
						seat(1, "Rex",
							engine.Raise{S: 1, To: 25},
							engine.Bet{S: 1, Amount: 30},
							engine.Bet{S: 1, Amount: 60},
						),
						seat(2, "Sana", engine.Fold{S: 2}),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 2,
							Teach: "The flop gives you four spades — nine clean outs. " +
								"Rule of 4: 9 × 4 ≈ 36%. Rex bets 30 into 55, so the " +
								"price is 30 / (85 + 30) = 26%. 36% against 26% is a " +
								"comfortable call. (Raising as a semi-bluff is also " +
								"reasonable — remember it for lesson 13.)",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Call{S: 0},
						},
						{
							AtDecision: 4,
							Teach: "The 4d missed you, and Rex bets 60 into 115. Same " +
								"nine outs, but only ONE card comes: rule of 2 says 9 × 2 " +
								"≈ 18%. The price is 60 / (175 + 60) = 26%. 18% against " +
								"26% — the math that said call yesterday says fold today. " +
								"Folding a pretty draw at a bad price is a profit, not a " +
								"retreat.",
							Expect: engine.Fold{S: 0},
						},
					},
					Intro: "Three-handed, blinds 5/10. The classic: 9s 8s picks up the " +
						"flush draw on As Ks 2h. You'll face a bet on the flop AND the " +
						"turn — and the right answer changes between them.",
					Debrief: "One draw, two prices, two different correct answers. Flop: " +
						"36% equity vs 26% price — call. Turn: 18% vs 26% — fold. Players " +
						"who learn the first half but not the second pay for everyone " +
						"else's vacations. You just practiced the second half.",
				},
			},
		},
	})
}
