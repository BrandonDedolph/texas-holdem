package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "why-we-bet",
		Title:         "Why We Bet",
		Goal:          "Every bet is value or bluff; c-betting; checking is fine.",
		Order:         10,
		Prerequisites: []string{"facing-raises", "playing-draws"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Two reasons, and only two",
				Text: "Every bet you ever make should answer one question: what am I " +
					"trying to get paid by, or what am I trying to fold out?\n\n" +
					"- VALUE: worse hands will call. You bet top pair because " +
					"middle pair pays it off.\n" +
					"- BLUFF: better hands will fold. You bet your missed draw " +
					"because king-high can't call.\n\n" +
					"If neither is true — worse hands all fold, better hands all call " +
					"— the bet accomplishes nothing except inflating the pot for " +
					"whoever's ahead. \"Seeing where I'm at\" and \"protecting my " +
					"hand\" are how players narrate bets that have no reason. Before " +
					"every bet, name the worse hands that call or the better hands " +
					"that fold. Can't name either? Check.",
			},
			{
				Kind:  tutorial.SectionText,
				Title: "The continuation bet",
				Text: "You raised preflop, one caller, and the flop comes down. " +
					"Betting again — the continuation bet — is strong by default, for " +
					"two honest reasons: flops miss everyone two times out of three " +
					"(so your bet often folds out their whiff), and the preflop " +
					"raiser's range holds more big pairs and big cards (so on boards " +
					"like A-7-2 or K-8-3, YOUR range hit and theirs didn't).\n\n" +
					"But c-betting every flop is a leak with a bullseye on it. On " +
					"low, connected boards that smash the caller's range — think " +
					"7-8-9 with two suits — your unimproved big cards are just " +
					"donating. And checking isn't surrender: it keeps the pot small " +
					"with medium hands, and it lets strong hands trap aggressive " +
					"opponents. Good players check a lot.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "You raised with Ad Kd and the flop is Ah 7c 2d. You bet " +
						"and get called by a worse ace. Your bet was...",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{
							"A value bet — worse hands called",
							"A bluff — you made better hands fold",
							"Neither, betting was a mistake",
						},
						Correct: 0,
					},
					Explain: "Textbook value. On A-7-2 you hold top pair, top kicker, " +
						"and the hands that call you — weaker aces, sevens, stubborn " +
						"pocket pairs — are all behind. This dry flop is also a classic " +
						"c-bet spot: it hit the raiser's range and missed the caller's. " +
						"Bet, get called by worse, repeat until rich.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "You raised with Ac 5c and the flop comes Jh Th 9s — low- " +
						"and-middle straight-and-flush city, and it smashed your " +
						"opponent's calling range. Best default?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{
							"C-bet anyway — you were the raiser",
							"Check, and be ready to give up",
							"Bet huge to protect your ace-high",
						},
						Correct: 1,
					},
					Explain: "Check. Ask the two questions: do worse hands call a bet " +
						"here? (No — anything worse than ace-high folds.) Do better " +
						"hands fold? (No — pairs, straights, and draws are going " +
						"nowhere.) A bet with no answer to either question is burning " +
						"chips. \"I raised preflop\" is a fact, not a reason.",
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: a c-bet with a purpose",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     0,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("Ad Kd"),
						2: engine.Holes("9c 9d"),
					},
					Board: engine.MustCards("Ah 7c 2d 5s"),
					Seed:  1001,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Raise{S: 0, To: 25},
							engine.Bet{S: 0, Amount: 30},
							engine.Bet{S: 0, Amount: 75},
						),
						seat(1, "Sana", engine.Fold{S: 1}),
						seat(2, "Cal",
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
							Teach: "You raised, Cal called, and the flop is Ah 7c 2d — " +
								"top pair, top kicker for you, and a board that misses " +
								"most of Cal's range. Name the worse hands that call: " +
								"weaker aces, pocket pairs like 88-99, maybe a stubborn " +
								"seven. That list is long, so this is a VALUE bet. Bet " +
								"around half pot.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Bet{S: 0, Amount: 30},
						},
						{
							AtDecision: 2,
							Teach: "Cal called the flop, so his range is exactly the " +
								"worse-hands list you named. The turn 5s changes nothing. " +
								"When the reason you bet still holds, bet again — value " +
								"hands make money by betting, not by admiring the pot.",
							Expect: engine.Bet{S: 0, Amount: 75},
						},
					},
					Intro: "Three-handed, blinds 5/10. You're on the button with Ad Kd " +
						"— this time the flop cooperates. Practice saying WHY before " +
						"you bet.",
					Debrief: "Cal's pocket nines called one street and gave up. Both " +
						"your bets had a spoken reason: \"worse aces and pairs call\". " +
						"The day a bet leaves your hand without a reason attached is " +
						"the day this lesson gets reopened.",
				},
			},
		},
	})
}
