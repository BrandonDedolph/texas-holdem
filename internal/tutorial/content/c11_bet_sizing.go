package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "bet-sizing",
		Title:         "Bet Sizing Speaks",
		Goal:          "Size as a fraction of the pot; what small/large sizes mean and offer.",
		Order:         11,
		Prerequisites: []string{"why-we-bet"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Sizes are fractions, not chip counts",
				Text: "A bet of 50 means nothing until you know the pot. Into 60 " +
					"it's a haymaker; into 500 it's a tickle. So sizing is always " +
					"spoken as a fraction of the pot — a third, a half, two-thirds, " +
					"full pot — and each fraction does two things at once:\n\n" +
					"- It sets the PRICE your opponent's draws and pairs must pay " +
					"(lesson 7's table, from the other side).\n" +
					"- It says something about your range — and listeners are taking " +
					"notes.\n\n" +
					"Small bets (a third) keep weak hands in and buy information " +
					"cheaply; they're at home on dry boards where nobody has much. " +
					"Big bets (two-thirds and up) charge draws properly and put real " +
					"pressure on medium hands; they belong on wet boards and with " +
					"strong value. The classic beginner tell is sizing by FEELINGS — " +
					"big when scared, small when strong. Size by the board and your " +
					"purpose instead, and you become unreadable.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "The pot is 100 and you bet 50 (half pot). What break-even " +
						"percentage are you offering your opponent's call? (within 1)",
					Answer: tutorial.NumericAnswer{Value: 25, Tolerance: 1},
					Explain: "They call 50 into 150, so 50 / (150 + 50) = 25%. This is " +
						"the deep point of sizing: YOU set the price of their draw. " +
						"Half pot lets an open-ender (32% on the flop) call profitably; " +
						"a full-pot bet (33% price) makes the same call roughly " +
						"break-even at best. Charging draws the wrong price is how " +
						"\"unlucky\" rivers keep happening to you.",
					Fact: tutorial.PotOddsFact{Pot: 150, Call: 50},
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "A strong overpair — but on this flop, straights and " +
						"flushes are drawing at you everywhere. Better default size?",
					Visual: &tutorial.Visual{
						Board: &tutorial.VisualBoard{
							Board:    engine.MustCards("Qh Jh 9s"),
							Hole:     engine.Holes("Kc Ks"),
							ShowHole: true,
						},
					},
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{
							"Small (about a third pot) — keep weak hands in",
							"Big (about three-quarters pot) — charge the draws",
						},
						Correct: 1,
					},
					Explain: "Go big. On a board this wet, half the deck helps someone; " +
						"every draw you undercharge is money out of your pocket. Save " +
						"the small sizes for dry boards like K-7-2 rainbow, where " +
						"nothing is drawing and a third pot does everything you need. " +
						"Wet board, strong hand, big bet — dry board, small bet. Let " +
						"the BOARD pick the number, not your pulse.",
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: two sizes, two sentences",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     0,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("Ac Ad"),
						2: engine.Holes("Kd Jc"),
					},
					Board: engine.MustCards("Kh 7h 2s 9c"),
					Seed:  1101,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Raise{S: 0, To: 25},
							engine.Bet{S: 0, Amount: 20},
							engine.Bet{S: 0, Amount: 70},
						),
						seat(1, "Sana", engine.Fold{S: 1}),
						seat(2, "Nora",
							engine.Call{S: 2},
							engine.Check{S: 2},
							engine.Call{S: 2},
							engine.Check{S: 2},
							engine.Fold{S: 2},
						),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 1,
							Teach: "Aces on K-7-2 with one suit — a mostly dry board " +
								"that your range owns. A THIRD of the pot (20 into 55) " +
								"does the job: worse kings and pairs call, and nothing " +
								"scary is drawing. Betting big here only folds out the " +
								"hands you beat.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Bet{S: 0, Amount: 20},
						},
						{
							AtDecision: 2,
							Teach: "Nora called — she has a piece, likely a king. The " +
								"turn 9c adds straight draws around the 9 and 7. Now " +
								"size UP: about three-quarters pot (70 into 95) charges " +
								"her king top dollar and any new draw the wrong price. " +
								"Small when dry, big when wet — same hand, two sizes, " +
								"each with a sentence behind it.",
							Expect: engine.Bet{S: 0, Amount: 70},
						},
					},
					Intro: "Three-handed, blinds 5/10. Pocket aces, and a board that " +
						"changes texture midway — so your size should change too.",
					Debrief: "20 on the flop, 70 on the turn, both for value, both " +
						"priced to the board. If both bets had been \"about half pot " +
						"because that's what I always do\", you'd have won less and " +
						"told everyone more. Sizes talk; make yours say something on " +
						"purpose.",
				},
			},
		},
	})
}
