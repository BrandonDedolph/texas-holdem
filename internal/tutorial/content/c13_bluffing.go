package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "bluffing",
		Title:         "Bluffing That Makes Money",
		Goal:          "Fold equity, good bluff candidates (blockers-lite), and who never folds.",
		Order:         13,
		Prerequisites: []string{"reading-players"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Bluffs have a price too",
				Text: "A bluff is priced by the same arithmetic as a call, seen from " +
					"the other side: risk your bet to win the pot.\n\n" +
					"    required fold rate = bet / (pot + bet)\n\n" +
					"Bluff 50 into 100 and you need folds a third of the time to " +
					"break even — they can call you MOST of the time and the bluff " +
					"still profits. That surprises everyone the first time. Bluffing " +
					"isn't an act of courage; it's a bet that the fold happens often " +
					"enough at the price you chose.\n\n" +
					"The number that makes it work is FOLD EQUITY — how often this " +
					"opponent, on this board, actually lets go. No fold equity, no " +
					"bluff, no exceptions.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "You bluff 50 into a pot of 100. What percent of the time " +
						"must your opponent fold for the bluff to break even? (within 1)",
					Answer: tutorial.NumericAnswer{Value: 33, Tolerance: 1},
					Explain: "50 / (100 + 50) = 33%. When the fold happens you win 100; " +
						"when it doesn't you lose 50 — so one success pays for two " +
						"failures. Before you bluff, ask: does THIS player fold a third " +
						"of their range here? Against a nit on a scary river, easily. " +
						"Against a station, never — which is the whole next section.",
					Fact: tutorial.PotOddsFact{Pot: 100, Call: 50},
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "Pick bluffs like you pick value bets",
				Text: "Good bluffs aren't random aggression — they're chosen:\n\n" +
					"- SEMI-BLUFFS first: draws that can fold them now or get there " +
					"later (lesson 9). Most of your bluffing volume should live here.\n" +
					"- Blockers-lite: holding a card that makes their best hands less " +
					"likely. Holding the As on a three-spade board is the classic — " +
					"the nut flush they'd call with is impossible.\n" +
					"- The story must add up: a bluff claims a specific strong hand. " +
					"If the betting says \"I have the flush\", the board had better " +
					"offer one.\n\n" +
					"And choose the AUDIENCE harder than the cards: bluff players who " +
					"fold (nits, TAGs on bad boards), never players who don't " +
					"(stations, and most maniacs mid-tantrum). The best bluff spot in " +
					"poker is a scary card against a tight player who was only " +
					"calling with one thing.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Which opponent should you basically never bluff?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"The nit", "The TAG", "The calling station"},
						Correct: 2,
					},
					Explain: "The station — a bluff only works if folding is on their " +
						"menu, and it isn't. Bluff the nit (folds too much) and pick " +
						"spots against the TAG (folds correctly, which is still " +
						"exploitable on scary boards). Last lesson you learned to " +
						"value bet the station relentlessly; this is the same read " +
						"pointed the other way.",
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: the semi-bluff, twice",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     0,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("8h 7h"),
						2: engine.Holes("Kd Td"),
					},
					Board: engine.MustCards("Kc 5h 4h 2c"),
					Seed:  1301,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Raise{S: 0, To: 25},
							engine.Bet{S: 0, Amount: 35},
							engine.Bet{S: 0, Amount: 85},
						),
						seat(1, "Sana", engine.Fold{S: 1}),
						seat(2, "Nora",
							engine.Call{S: 2},
							engine.Check{S: 2}, engine.Call{S: 2},
							engine.Check{S: 2}, engine.Fold{S: 2},
						),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 1,
							Teach: "You raised 8h 7h on the button and the flop came " +
								"Kc 5h 4h: a flush draw plus a gutshot (any six). You " +
								"have two ways to win — Nora folds now, or you improve " +
								"later. That combination is the semi-bluff, the " +
								"safest aggressive play in poker. Bet.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Bet{S: 0, Amount: 35},
						},
						{
							AtDecision: 2,
							Teach: "Nora called — from a nit, that's one king-ish hand. " +
								"The turn 2c is a brick for you, but your bluff needs " +
								"85 / (125 + 85) ≈ 40% folds to break even, and you STILL " +
								"have your draw as a safety net when she calls. Against " +
								"a player this tight, second barrels on boards that keep " +
								"getting worse for one-pair hands print money. Fire again.",
							Expect: engine.Bet{S: 0, Amount: 85},
						},
					},
					Intro: "Three-handed, blinds 5/10. Nora the nit defends her big " +
						"blind. You flop a draw and a decision: passive or aggressive?",
					Debrief: "Nora folded top pair to the second barrel — the fold " +
						"equity paid before the draw ever needed to. Note everything " +
						"that made it work: a draw underneath, a tight opponent, and a " +
						"bet priced so that even 40% folds broke even. Bluffing that " +
						"makes money is engineering, not theater. Curriculum complete — " +
						"now go play, and let the coach argue with you in real time.",
				},
			},
		},
	})
}
