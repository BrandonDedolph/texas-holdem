package content

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

func init() {
	tutorial.Register(&tutorial.Lesson{
		ID:            "reading-players",
		Title:         "Reading Opponents",
		Goal:          "VPIP/PFR/AF, the five archetypes, and one exploit for each.",
		Order:         12,
		Prerequisites: []string{"bet-sizing"},
		Sections: []tutorial.Section{
			{
				Kind:  tutorial.SectionText,
				Title: "Three numbers describe a player",
				Text: "You don't need psychic reads — three stats sketch anyone:\n\n" +
					"- VPIP: how often they voluntarily put money in preflop. " +
					"Loose or tight.\n" +
					"- PFR: how often they RAISE preflop. Aggressive or passive " +
					"before the flop.\n" +
					"- AF (aggression factor): once the flop is out, do they bet and " +
					"raise, or check and call?\n\n" +
					"The gaps between numbers talk loudest. VPIP 50 / PFR 5 — plays " +
					"half their hands, almost never raises — is a caller. VPIP 22 / " +
					"PFR 18 — nearly every hand they play, they play with a raise — " +
					"is a disciplined aggressor. After a few orbits at any table you " +
					"can estimate all three from memory; this app's opponents each " +
					"wear theirs honestly.",
			},
			{
				Kind:  tutorial.SectionText,
				Title: "The five regulars",
				Text: "Five archetypes cover most players you'll meet, each with one " +
					"exploit worth memorizing:\n\n" +
					"- NIT (VPIP ~10): plays only monsters. Exploit: steal their " +
					"blinds forever, and fold when they raise big — they always " +
					"have it.\n" +
					"- TAG (VPIP ~22, high PFR): tight-aggressive, solid. Exploit: " +
					"barely any — respect their raises, fight for position. This is " +
					"also who YOU are becoming.\n" +
					"- LAG (VPIP ~35, high PFR): relentless pressure. Exploit: let " +
					"them bluff into your strong hands; call down lighter.\n" +
					"- STATION (VPIP ~50, tiny PFR, low AF): calls everything. " +
					"Exploit: value bet thinner and bigger, and NEVER bluff.\n" +
					"- MANIAC (VPIP ~60+, raises blind): chaos. Exploit: wait for a " +
					"real hand, then let them stack themselves.\n\n" +
					"One sentence each. The scripted hand drills the most profitable " +
					"one: milking the station.",
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "An opponent shows VPIP 52, PFR 4, and a low aggression " +
						"factor. Which archetype?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{"Nit", "TAG", "Calling station", "Maniac"},
						Correct: 2,
					},
					Explain: "Plays half their hands (loose), almost never raises " +
						"(passive), rarely bets postflop (calls instead): the calling " +
						"station. Nits show ~10/8, TAGs ~22/18, maniacs raise far more " +
						"than 4. Learn to read the gap: VPIP sky-high with PFR near " +
						"zero means every chip they give you must be pulled by a value " +
						"bet — they won't fold, but they'll pay.",
				},
			},
			{
				Kind: tutorial.SectionDrill,
				Drill: &tutorial.Drill{
					Prompt: "Against that calling station, which adjustment makes you " +
						"the most money?",
					Answer: tutorial.ChoiceAnswer{
						Choices: []string{
							"Bluff more — they play too many hands",
							"Value bet thinner and bigger, and stop bluffing",
							"Tighten up and only play aces and kings",
						},
						Correct: 1,
					},
					Explain: "Stations don't fold, so bluffs are donations — but for " +
						"the same reason, every value bet gets paid. Bet hands you'd " +
						"check elsewhere (second pair, weak top pair), size up, and " +
						"keep betting. Exploits are symmetric: the same stubbornness " +
						"that makes bluffing terrible makes value betting glorious.",
				},
			},
			{
				Kind:  tutorial.SectionScripted,
				Title: "Scripted hand: milk the station",
				Script: &tutorial.ScriptedHand{
					Hero:       0,
					Button:     0,
					SmallBlind: sb,
					BigBlind:   bb,
					Stacks:     stacks3(),
					Holes: map[engine.Seat][2]engine.Card{
						0: engine.Holes("Ks Qs"),
						2: engine.Holes("Qc 7c"),
					},
					Board: engine.MustCards("Qh 8c 3d 6s 2d"),
					Seed:  1201,
					Seats: []tutorial.ScriptSeat{
						seat(0, "You",
							engine.Raise{S: 0, To: 25},
							engine.Bet{S: 0, Amount: 35},
							engine.Bet{S: 0, Amount: 80},
							engine.Bet{S: 0, Amount: 150},
						),
						seat(1, "Sana", engine.Fold{S: 1}),
						seat(2, "Cal",
							engine.Call{S: 2},
							engine.Check{S: 2}, engine.Call{S: 2},
							engine.Check{S: 2}, engine.Call{S: 2},
							engine.Check{S: 2}, engine.Call{S: 2},
						),
					},
					Stops: []tutorial.Stop{
						{
							AtDecision: 1,
							Teach: "Top pair, king kicker against Cal — the table's " +
								"calling station. Against most players you'd bet three " +
								"streets here cautiously; against Cal you bet them " +
								"CONFIDENTLY, and bigger than usual. His calling range " +
								"includes any queen, any eight, any pair, and hands no " +
								"one can explain.",
							// TODO(wire-coach): live coach commentary could replace this
							// once internal/coach lands.
							Expect: engine.Bet{S: 0, Amount: 35},
						},
						{
							AtDecision: 3,
							Teach: "River. The board stayed harmless and Cal called " +
								"twice, which from a station means \"some pair, any " +
								"pair\". Most opponents fold weaker queens to a third " +
								"barrel — Cal won't. Bet again, solidly. Thin value is " +
								"the entire station exploit: you're not hoping he's " +
								"weak, you're charging him for being stubborn.",
							Expect: engine.Bet{S: 0, Amount: 150},
						},
					},
					Intro: "Three-handed, blinds 5/10. Cal (VPIP 52, PFR 4) calls your " +
						"raise from the big blind. You flop top pair. Do not slow down.",
					Debrief: "Cal called three streets with queen-seven and paid you " +
						"265 past the flop. Against a TAG that river bet is thin; " +
						"against a station it's routine. Same cards, different " +
						"opponent, different line — the read IS the strategy.",
				},
			},
		},
	})
}
