package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "outs-equity",
		Title:         "Outs & the Rule of 2 and 4",
		Goal:          "Count clean outs and convert to equity fast.",
		Order:         8,
		Prerequisites: []string{"pot-odds"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Outs: the cards that rescue you",
				Text: "An out is a card still in the deck that turns your hand into " +
					"the winner. Not \"improves it\" — WINS with it. Counting outs is " +
					"how you turn \"I have a draw\" into a number you can compare " +
					"against the price of a call.\n\n" +
					"The counts worth knowing cold:\n\n" +
					"- Flush draw (four to a suit): 9 outs\n" +
					"- Open-ended straight draw: 8 outs\n" +
					"- Gutshot (inside straight): 4 outs\n" +
					"- Two overcards to the board: 6 outs (shakier — see below)\n" +
					"- Pocket pair hoping for a set: 2 outs\n\n" +
					"Count CLEAN outs only. If a heart completes your flush but also " +
					"pairs the board, someone drawing to a full house just got there " +
					"too. When an out is dirty, count it as half.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "Nine spades to glory",
				Text: "You hold 9s 8s on As Ks 2h: four spades, needing one more. " +
					"Thirteen spades exist, you can see four — nine remain. Every one " +
					"of them makes you a flush that beats anything one pair can do.",
				Visual: &tutorial.Visual{
					Board: &tutorial.VisualBoard{
						Board:    engine.MustCards("As Ks 2h"),
						Hole:     engine.Holes("9s 8s"),
						ShowHole: true,
					},
					Caption: "13 spades - 2 in your hand - 2 on the board = 9 outs.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Four spades, needing one more. How many clean outs do you " +
						"have? (exact)",
					Visual: &tutorial.Visual{
						Board: &tutorial.VisualBoard{
							Board:    engine.MustCards("As Ks 2h"),
							Hole:     engine.Holes("9s 8s"),
							ShowHole: true,
						},
					},
					Answer: tutorial.NumericAnswer{Value: 9},
					Explain: "Nine — the remaining spades. Note what you DON'T count: " +
						"pairing your 9 or 8 doesn't help, because a pair of nines still " +
						"loses to the aces and kings this board hits. An out must lift you " +
						"OVER the likely best hand, not just move you up a rung.",
					Fact: tutorial.OutsFact{Hero: "9s 8s", Board: "As Ks 2h"},
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "The rule of 4 and 2",
				Text: "Turning outs into a winning percentage at the table is one " +
					"multiplication:\n\n" +
					"- On the FLOP (two cards coming): outs × 4\n" +
					"- On the TURN (one card coming): outs × 2\n\n" +
					"Flush draw on the flop: 9 × 4 ≈ 36%. The exact answer is about " +
					"35% — the shortcut is off by a point or two, and decisions are " +
					"never that close. Same draw on the turn: 9 × 2 ≈ 18%, exact is " +
					"about 20%.\n\n" +
					"This is the other half of last lesson's formula. Price from the " +
					"pot, equity from your outs — compare, decide. That two-step is " +
					"the entire mathematical core of poker, and you can do it in your " +
					"head while holding a conversation.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Same flush draw on the flop, but now the opponent shows " +
						"top pair. Use the rule of 4: about what percent do you win? " +
						"(within 5)",
					Visual: &tutorial.Visual{
						Showdown: &tutorial.VisualShowdown{
							Board: engine.MustCards("As Ks 2h"),
							Hands: []tutorial.ShownHand{
								{Label: "You", Hole: engine.Holes("9s 8s")},
								{Label: "Opponent", Hole: engine.Holes("Ad Qd")},
							},
						},
					},
					Answer: tutorial.NumericAnswer{Value: 36, Tolerance: 5},
					Explain: "9 outs × 4 ≈ 36%; the exact enumeration says 37%. That's " +
						"the whole trick — a one-digit multiplication lands within a " +
						"couple points of the computer, every time, and it's fast enough " +
						"to use on every single decision. You never need the exact number " +
						"at a real table; you need to know which side of the price you're on.",
					Fact: tutorial.EquityFact{
						Hero:    "9s 8s",
						Villain: "Ad Qd",
						Board:   "As Ks 2h",
						Within:  3,
					},
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: count, price, call — then cash in",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     1,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("6h 5h"),
						1: engine.Holes("Kd Qd"),
					},
					Board: engine.MustCards("7c 8d Ks 9h 2s"),
					Seed:  801,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Call{S: 0},             // defend the big blind
							engine.Check{S: 0},            // flop
							engine.Call{S: 0},             // flop, facing 30
							engine.Bet{S: 0, Amount: 60},  // turn: straight
							engine.Bet{S: 0, Amount: 120}, // river
						),
						seat(1, "Rex",
							engine.Raise{S: 1, To: 25},
							engine.Bet{S: 1, Amount: 30},
							engine.Call{S: 1},
							engine.Fold{S: 1},
						),
						seat(2, "Sana", engine.Fold{S: 2}),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 2,
							Teach: "Rex bets 30 into 55. Count first: your 6-5 on 7-8-K " +
								"is an open-ended straight draw — any four or any nine, " +
								"eight clean outs. Rule of 4: 8 × 4 ≈ 32%. Price: 30 / " +
								"(85 + 30) = 26%. Equity 32% beats price 26% — call. Notice " +
								"the order: count, multiply, price, compare. Same four steps " +
								"every time.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Call{S: 0},
						},
						{
							AtDecision: 3,
							Teach: "The 9h completes your straight: 5-6-7-8-9. The draw " +
								"has done its job; now it's a value hand, and value hands " +
								"bet. Don't slowplay draws that get there — the pot you " +
								"fought for the right to win only grows if you grow it.",
							Expect: engine.Bet{S: 0, Amount: 60},
						},
					},
					Intro: "Three-handed, blinds 5/10. You defend the big blind with " +
						"6h 5h and flop an open-ender. Count your outs before every click.",
					Debrief: "Flop: 8 outs, 32% equity, 26% price — call. Turn: draw " +
						"completed, switch from calling to betting. Rex's fold on the " +
						"river doesn't change the math; the profit was locked in when " +
						"you took a 26% price on a 32% draw.",
				},
			},
		},
	})
}
