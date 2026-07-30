package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "kickers-ties",
		Title:         "Kickers & Ties",
		Goal:          "Read a board and say who wins — kickers, counterfeits, and split pots.",
		Order:         3,
		Prerequisites: []string{"hand-flow"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "The fifth card decides",
				Text: "When two players make the same pair, the pot is decided by the " +
					"leftover cards that fill out the five — the kickers. Both of you " +
					"have a pair of aces? Then it's your next-best card against theirs.\n\n" +
					"This is why AK is a premium hand and A4 is a trap: they make the " +
					"same pair of aces, but when both connect, the king kicker beats the " +
					"four every time. Beginners lose more money with a weak-kicked ace " +
					"than with any other holding — you make top pair, it looks great, and " +
					"you are drawing nearly dead against a better ace.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "Finding your best five",
				Text: "You hold Ac Kd on this board. Pair of aces — and then the best " +
					"three side cards available. The app highlights the five that play; " +
					"train yourself to see them without help.",
				Visual: &tutorial.Visual{
					Board: &tutorial.VisualBoard{
						Board:    engine.MustCards("Ah 9d 6c 3s 2d"),
						Hole:     engine.Holes("Ac Kd"),
						ShowHole: true,
						ShowBest: true,
					},
					Caption: "The 3s and 2d never play: your K and the board's 9 and 6 outkick them.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Board: Ah 9d 6c 3s 2d. Player A holds Ac Kd. Player B holds " +
						"As Qh. Who wins?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Player A", "Player B", "Split pot"},
						Correct: 0,
					},
					Explain: "Both have a pair of aces, so go to the kickers: king beats " +
						"queen, done. Note how thin the margin is — B played a strong hand " +
						"and lost the maximum. When you both hit the same pair, kicker " +
						"fights are where stacks quietly leak.",
					Fact: tutorial.WinnerFact{
						Board: "Ah 9d 6c 3s 2d",
						Hands: []string{"Ac Kd", "As Qh", ""},
						Split: 2,
					},
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "Counterfeits and playing the board",
				Text: "Sometimes the board takes your hand away. You hold 3h 3c, the " +
					"board runs out with two pairs bigger than yours — now your threes " +
					"don't fit in the best five anymore. That's being counterfeited: " +
					"nothing was \"taken\", the board just made your cards irrelevant.\n\n" +
					"And when the board itself is the best five — say it shows a straight " +
					"— everyone still in the hand plays that same five and the pot " +
					"splits, no matter who \"had more\" on the side, unless someone's hole " +
					"card improves on the board's five. Always ask: does my card actually " +
					"make the five better, or does it just sit in my hand looking pretty?",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Board: 7c 8d 9h Tc Js — a jack-high straight on the board. " +
						"Player A holds Ad 2c. Player B holds Kh 3d. Who wins?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Player A", "Player B", "Split pot"},
						Correct: 2,
					},
					Explain: "Neither hole card improves the board's straight — an ace or " +
						"a king doesn't extend 7-8-9-T-J (only a queen would, by making a " +
						"queen-high straight). Both players play the board and split. The " +
						"ace is doing nothing; five cards, best five, every time.",
					Fact: tutorial.WinnerFact{
						Board: "7c 8d 9h Tc Js",
						Hands: []string{"Ad 2c", "Kh 3d", ""},
						Split: 2,
					},
				},
			},
		},
	})
}
