package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// Standard scripted-hand stakes: 5/10 blinds, 100bb stacks — the same
// classroom stakes the profile defaults to.
const (
	sb    engine.Chips = 5
	bb    engine.Chips = 10
	stack engine.Chips = 1_000
)

// The teaching ranges. These are deliberately simple, chart-shaped ranges —
// tight early, wide late — not solver output. The content tests verify the
// membership facts the drills quote (A5s opens on the button, 72o opens
// nowhere) and that the button range sits in the 38–46% band the design
// pins for RFI charts.
const (
	// RangeUTG is the under-the-gun opening range, ~9% of hands.
	RangeUTG = "77+, ATs+, KQs, AJo+, KQo"
	// RangeBTN is the button opening range, ~42% of hands.
	RangeBTN = "22+, A2s+, K2s+, Q4s+, J6s+, T6s+, 96s+, 86s+, 75s+, 64s+, 53s+, " +
		"A2o+, K8o+, Q9o+, J9o+, T9o"
)

// mustRange parses a range spec, panicking on error — content is compiled
// in, so a bad spec is a build-time authoring bug the tests surface.
func mustRange(spec string) equity.Range {
	r, err := equity.ParseRange(spec)
	if err != nil {
		panic(err)
	}
	return r
}

// seat builds a ScriptSeat concisely.
func seat(s engine.Seat, name string, actions ...engine.Action) tutorial.ScriptSeat {
	return tutorial.ScriptSeat{Seat: s, Name: name, Actions: actions}
}

// stacks6 seats six players with the standard stack.
func stacks6() map[engine.Seat]engine.Chips {
	m := make(map[engine.Seat]engine.Chips, 6)
	for s := engine.Seat(0); s < 6; s++ {
		m[s] = stack
	}
	return m
}

// stacks3 seats three players with the standard stack.
func stacks3() map[engine.Seat]engine.Chips {
	return map[engine.Seat]engine.Chips{0: stack, 1: stack, 2: stack}
}

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "preflop-ranges",
		Title:         "Starting Hands by Seat",
		Goal:          "Play a tight, position-widening range — read an RFI chart, fold the rest without regret.",
		Order:         5,
		Prerequisites: []string{"position"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Ranges, not hands",
				Text: "Strong players don't decide hand by hand — they decide, once, " +
					"which SET of hands each seat opens with, and then just consult the " +
					"set. That set is a range, and the grid on the next page is how " +
					"ranges are written everywhere in poker.\n\n" +
					"Reading the grid: pairs run down the diagonal. Above the diagonal " +
					"is suited (AKs = ace-king of the same suit), below is offsuit " +
					"(AKo). Suited versions of a hand are always worth more — flushes " +
					"are real money.\n\n" +
					"Folding 80% of your hands under the gun isn't timid, it's the " +
					"strategy. Every fold is a hand where position and card strength " +
					"were both against you, and the money you didn't lose is identical " +
					"to money won.",
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "The button opens wide",
				Text: "On the button you're last to act for the whole hand, so weak " +
					"hands become playable: every suited ace, most suited connectors, " +
					"any two cards ten-or-better. Roughly 4 hands in 10.",
				Visual: &tutorial.Visual{
					RangeGrid: &tutorial.VisualRangeGrid{
						Range: mustRange(RangeBTN),
						Title: "Button opening range (~42% of hands)",
					},
					Caption: "Bright cells raise first-in; dim cells fold.",
				},
			},
			{
				Kind:  tutorial.SectionVisual,
				Title: "Under the gun stays tight",
				Text: "Same game, same you, five seats earlier — and the range " +
					"collapses to about 9%: big pairs, big aces, and the very best " +
					"broadways. Every hand you open UTG has to survive five players " +
					"acting after you with position.\n\n" +
					"The seats in between widen step by step: HJ a little looser than " +
					"UTG, CO a little looser than HJ, button widest. One idea — later " +
					"seat, more hands — generates every chart.",
				Visual: &tutorial.Visual{
					RangeGrid: &tutorial.VisualRangeGrid{
						Range: mustRange(RangeUTG),
						Title: "Under-the-gun opening range (~9% of hands)",
					},
					Caption: "A5s is a fine button open and an easy UTG fold — same cards, different seat.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Folded to you on the button with A5s. Open or fold?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Raise (open)", "Fold"},
						Correct: 0,
					},
					Explain: "Open. A5s has a lot going for it in late position: it can " +
						"make the nut flush, it can make a wheel straight (A-2-3-4-5), and " +
						"holding an ace makes it a bit less likely the blinds have one. The " +
						"same hand is a fold UTG — it's not about the cards being \"good\", " +
						"it's about the seat.",
					Fact: tutorial.RangeFact{
						Spec:      RangeBTN,
						Hand:      "As 5s",
						InChoice:  0,
						OutChoice: 1,
					},
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Folded to you on the button — the loosest seat — with 72o. Open or fold?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Raise (open)", "Fold"},
						Correct: 1,
					},
					Explain: "Fold, always, everywhere. 72o is the worst starting hand " +
						"in the game: no pair, no straight possibility worth naming, no " +
						"suit, and both cards lose kicker fights. If the widest range in " +
						"the chart doesn't include a hand, no seat does. Folding trash " +
						"instantly, without a pang, is a skill — and it's one of the few " +
						"in poker you can perfect on day one.",
					Fact: tutorial.RangeFact{
						Spec:      RangeBTN,
						Hand:      "7s 2h",
						InChoice:  0,
						OutChoice: 1,
					},
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: open from the cutoff",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     1,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks6(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("Ks Qs"),
					},
					Seed: 501,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You", engine.Raise{S: 0, To: 25}),
						seat(1, "Rex", engine.Fold{S: 1}),
						seat(2, "Sana", engine.Fold{S: 2}),
						seat(3, "Bo", engine.Fold{S: 3}),
						seat(4, "Nia", engine.Fold{S: 4}),
						seat(5, "Ivan", engine.Fold{S: 5}),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 0,
							Teach: "Two players folded; you're in the cutoff with KQs. " +
								"KQs is comfortably inside the cutoff's opening range — big " +
								"cards, suited, and only three players left behind you. Open " +
								"to 2.5 big blinds. Raising (never limping) is the default: " +
								"it can win the blinds outright and builds a pot you " +
								"position-play from.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Raise{S: 0, To: 25},
						},
					},
					Intro: "Six-handed, blinds 5/10. It folds to you in the cutoff with " +
						"Ks Qs. Practice the open you'll make thousands of times.",
					Debrief: "Everyone folded — that's the most common result of a raise, " +
						"and it's a fine one: 15 chips for free. An opening range means " +
						"you never agonize over this spot again; the chart already decided.",
				},
			},
		},
	})
}
