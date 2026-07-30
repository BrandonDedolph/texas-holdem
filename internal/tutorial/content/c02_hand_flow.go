package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "hand-flow",
		Title:         "How a Hand Works",
		Goal:          "Blinds, four streets, showdown — who acts when and why the button moves.",
		Order:         2,
		Prerequisites: []string{"hand-rankings"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "The blinds start the fight",
				Text: "Before any cards are dealt, two players are forced to put chips in: " +
					"the small blind and the big blind, the two seats after the dealer " +
					"button. The blinds exist so there is always something to win — " +
					"without them, everyone could fold forever for free and the correct " +
					"strategy would be to wait for aces.\n\n" +
					"The button moves one seat clockwise every hand, so the blinds rotate " +
					"and everyone pays the same tax over an orbit. Position, blinds, and " +
					"the deal order all key off the button — it is the table's clock.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "Who acts when",
				Text: "Preflop, the player after the big blind acts first (\"under the " +
					"gun\") and the big blind acts last — they already paid, so they get " +
					"the last word. On every later street the order shifts: the first " +
					"player after the button still in the hand acts first, and the button " +
					"acts last. Acting last is a big deal; lesson 4 is entirely about why.",
				Visual: &tutorial.Visual{
					TableSeats: &tutorial.VisualTableSeats{
						Highlight: []engine.Position{engine.PosUTG},
						Notes: map[engine.Position]string{
							engine.PosUTG: "acts first preflop",
							engine.PosBTN: "acts last after the flop",
							engine.PosSB:  "posts the small blind",
							engine.PosBB:  "posts the big blind, acts last preflop",
						},
					},
					Caption: "Six-max seats. The button moves one seat clockwise each hand.",
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "Four streets, one river",
				Text: "A hand is up to four rounds of betting:\n\n" +
					"- Preflop: you have only your two hole cards. First decisions.\n" +
					"- Flop: three community cards arrive at once. Most of your hand's " +
					"identity is settled here — five of seven cards are now visible.\n" +
					"- Turn: one more card, and bets tend to double in seriousness.\n" +
					"- River: the last card. No more draws, no more \"maybe\" — every " +
					"hand is exactly what it is.\n\n" +
					"Betting on each street ends when everyone still in has matched the " +
					"biggest bet (or checked). If at any point only one player is left, " +
					"they win immediately and never have to show their cards — which is " +
					"why bluffing can exist at all. If two or more reach the end of river " +
					"betting, it's a showdown: best five cards takes the pot.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Put the streets in order, first to last:\n" +
						"  1. River\n  2. Flop\n  3. Preflop\n  4. Turn",
					Answer: tutorial.OrderAnswer{
						Items:   []string{"River", "Flop", "Preflop", "Turn"},
						Correct: []int{2, 1, 3, 0},
					},
					Explain: "Preflop, flop, turn, river. Two hole cards, then 3 + 1 + 1 " +
						"community cards. A betting round on each — four chances to win the " +
						"pot without a showdown, or to get away from a losing hand cheaply.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Six players, preflop. Who must act first?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{
							"The small blind",
							"The button",
							"The player after the big blind (under the gun)",
							"The big blind",
						},
						Correct: 2,
					},
					Explain: "The blinds already have chips in, so the action starts with " +
						"the first player who hasn't paid anything: under the gun. Acting " +
						"first with five players behind you is the worst spot at the table " +
						"— remember this seat's name, because the next lessons keep " +
						"charging it extra.",
				},
			},
		},
	})
}
