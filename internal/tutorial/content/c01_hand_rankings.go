// Package content is the lesson curriculum (docs/design-learning.md §8.4):
// thirteen lessons, one file each, registered into the tutorial registry at
// init time. Importing this package (for side effects) is what populates the
// Learning Journey screen.
//
// Authoring conventions: cards are always written as code strings ("As Kd"),
// never raw values; every drill that asserts a poker fact carries a
// tutorial.Fact so the content tests verify it against eval/equity; and every
// scripted hand must replay through the real engine — TestScriptsReplay
// fails the build if a script drifts from engine legality.
package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:    "hand-rankings",
		Title: "Hand Rankings",
		Goal:  "Know the ten hand categories cold, and that the best five of seven play.",
		Order: 1,
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Five cards out of seven",
				Text: "In Texas Hold'em you get two private cards (your hole cards) and " +
					"share five community cards with everyone else. At showdown, your hand " +
					"is the best FIVE cards you can pick from those seven — you may use " +
					"both hole cards, one, or neither.\n\n" +
					"That last part surprises people: if the board is the best five, you " +
					"\"play the board\" and your hole cards do nothing. Your hand is never " +
					"six or seven cards, and never fewer than five.\n\n" +
					"Everything else in poker sits on top of one question: whose five " +
					"cards are best? So the ranking order comes first.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "The ladder",
				Text: "Ten tiers, strongest at the top. A higher tier always beats a " +
					"lower one — the worst flush beats the best straight. Within a tier, " +
					"higher cards win (a pair of kings beats a pair of nines).\n\n" +
					"Notice the order tracks rarity: pairs are common, flushes are not, " +
					"straight flushes are famous for a reason.",
				Visual: &tutorial.Visual{
					HandLadder: &tutorial.VisualHandLadder{},
					Caption:    "Every tier beats every tier below it, no matter the cards.",
				},
			},
			{
				Kind:  tutorial.SectionText,
				Title: "Saying a hand out loud",
				Text: "Learn to name what you hold: \"two pair, aces and nines\", " +
					"\"king-high flush\", \"straight, nine to king\". Naming forces you to " +
					"actually find your best five instead of vaguely liking your cards.\n\n" +
					"Two traps to watch for while you practice:\n\n" +
					"- Three pairs is not a thing. With seven cards you can pair three " +
					"times, but only the best two pairs (plus the best kicker) fit in five " +
					"cards.\n" +
					"- A straight needs five in a row. Four in a row is nothing yet — " +
					"that's a draw, and draws are a later lesson.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Who wins at showdown?",
					Visual: &tutorial.Visual{
						Showdown: &tutorial.VisualShowdown{
							Board: engine.MustCards("Qs Jh 7d 4c 2h"),
							Hands: []tutorial.ShownHand{
								{Label: "Player A", Hole: engine.Holes("Ac Kd")},
								{Label: "Player B", Hole: engine.Holes("7h 8h")},
							},
						},
					},
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Player A", "Player B", "Split pot"},
						Correct: 1,
					},
					Explain: "Player B pairs the board: a pair of sevens (7h 7d Qs Jh 8h). " +
						"Player A has no pair at all — ace-king high is a strong START, but " +
						"it finished as high card, and any pair beats it. Big cards lose to " +
						"small pairs all day; the ladder doesn't care how pretty the losers are.",
					Fact: tutorial.WinnerFact{
						Board: "Qs Jh 7d 4c 2h",
						Hands: []string{"Ac Kd", "7h 8h", ""},
						Split: 2,
					},
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Order these five-card hands from strongest to weakest:",
					Answer: tutorial.OrderAnswer{
						Items:   []string{"Ad 10d 8d 6d 3d", "Kc Kh Kd 4s 4d", "9c 8d 7h 6s 5c"},
						Correct: []int{1, 0, 2},
					},
					Explain: "Full house (kings full of fours) beats flush (ace-high), " +
						"which beats straight (nine-high). If you had to think about it, " +
						"good — that's the ladder wiring itself in. It has to be instant " +
						"before the later lessons, because at the table you'll be comparing " +
						"hands while also thinking about money.",
					Fact: tutorial.RankOrderFact{
						Hands: []string{"Ad Td 8d 6d 3d", "Kc Kh Kd 4s 4d", "9c 8d 7h 6s 5c"},
					},
				},
			},
		},
	})
}
