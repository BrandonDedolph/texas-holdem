package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "position",
		Title:         "Position Is Power",
		Goal:          "Acting last is worth money; name the six seats and rank them.",
		Order:         4,
		Prerequisites: []string{"kickers-ties"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Information is the currency",
				Text: "Poker is a game of incomplete information, and position is how " +
					"much information you get for free. Acting last means everyone else " +
					"has already told you something — they checked (weak-ish), they bet " +
					"(strong-ish), they hesitated. Acting first means you decide blind " +
					"and five players get to react to you.\n\n" +
					"The button acts last on every street after the flop, forever, by " +
					"rule. That single fact is worth so much that the same cards are a " +
					"clear play on the button and a clear fold under the gun. Not " +
					"slightly different — opposite decisions, purely because of the seat.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "The six seats",
				Text: "Learn the names — every chart, every lesson, and the coach all " +
					"speak in them:\n\n" +
					"- UTG (under the gun): first to act preflop. Worst seat.\n" +
					"- HJ (hijack): one better.\n" +
					"- CO (cutoff): second-best; often attacks the button.\n" +
					"- BTN (button): acts last postflop. The best seat in poker.\n" +
					"- SB (small blind): paid half a blind, acts FIRST postflop. Bad seat.\n" +
					"- BB (big blind): paid a full blind, closes the preflop action. You " +
					"defend it, but you play the rest of the hand out of position.",
				Visual: &tutorial.Visual{
					TableSeats: &tutorial.VisualTableSeats{
						Highlight: []engine.Position{engine.PosBTN},
						Notes: map[engine.Position]string{
							engine.PosUTG: "worst seat: five players react to you",
							engine.PosCO:  "second best",
							engine.PosBTN: "best seat: always acts last postflop",
							engine.PosSB:  "half a blind in, first to act postflop",
							engine.PosBB:  "closes preflop action, out of position after",
						},
					},
					Caption: "Position is a function of the button, and the button moves every hand.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Which seat is the most profitable in the long run?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Under the gun", "The big blind", "The button", "The small blind"},
						Correct: 2,
					},
					Explain: "The button — guaranteed last action on the flop, turn, and " +
						"river. The big blind is a distant thought because of the discount " +
						"(you already paid), but you're out of position for the rest of the " +
						"hand. Databases of millions of real hands show the button is the " +
						"only seat that wins money for almost everyone who sits in it.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Rank these seats from best to worst:\n" +
						"  1. UTG\n  2. BTN\n  3. CO",
					Answer: tutorial.OrderAnswer{
						Items:   []string{"UTG", "BTN", "CO"},
						Correct: []int{1, 2, 0},
					},
					Explain: "BTN, then CO, then UTG. The pattern generalizes: value " +
						"increases as the action gets closer to the button. The next lesson " +
						"turns this straight into strategy — the later your seat, the more " +
						"hands you're allowed to play.",
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "What position buys you",
				Text: "Concretely, acting last lets you:\n\n" +
					"- Bluff better: you see weakness before committing chips.\n" +
					"- Value bet thinner: you know the price was checked to you.\n" +
					"- Control the pot: last action decides whether the street closes " +
					"cheap or expensive.\n" +
					"- Draw cheaper: nobody can raise behind you after you call.\n\n" +
					"Every one of those is worth real money, and none of them require " +
					"better cards. When a later lesson says a play \"needs position\", " +
					"this menu is what it means.",
			},
		},
	})
}
