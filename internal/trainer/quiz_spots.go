package trainer

import (
	"math/rand/v2"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// Spot is the grading context of a "what's your action?" item. The drill's
// ChoiceAnswer names the coach's pick for display, but the real check runs
// coach.GradeAction against the frozen Advice — so a defensible second-best
// line (GradeGood) counts as correct instead of being marked wrong on a
// coin flip.
type Spot struct {
	// Advice is the coach's frozen recommendation, computed from the
	// hero's PlayerView the moment the generated hand reached the hero.
	Advice coach.Advice

	// Options are the concrete actions behind the drill's choices,
	// index-aligned. Sized options carry the strategy's own best-scored
	// size for their type, so the quiz grades the direction the player
	// chose, never a sizing they had no control over.
	Options []engine.Action
}

// Stakes for every generated spot: 5/10 blinds, 100bb stacks — the same
// classroom stakes the profile defaults to, so the numbers on screen match
// the game the player knows.
const (
	spotSmallBlind engine.Chips = 5
	spotBigBlind   engine.Chips = 10
	spotStack      engine.Chips = 1_000
)

// genSpots generates one decision spot through the real engine: a scripted
// hand is driven to the hero's turn, the coach advises from the hero's
// view, and the advice is frozen into the item. Levels:
//
//	spot.preflop     6-max, unopened or facing one raise — chart territory
//	spot.flop-price  the hero faces a flop lead on the button
//	spot.turn-river  a turn or river lead, with an archetype read shown
func genSpots(rng *rand.Rand, tag string) Item {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var (
			it Item
			ok bool
		)
		switch tag {
		case tagSpotPreflop:
			it, ok = buildPreflopSpot(rng)
		case tagSpotFlop:
			it, ok = buildStreetSpot(rng, tag, engine.Flop)
		default:
			street := engine.Turn
			if rng.IntN(2) == 0 {
				street = engine.River
			}
			it, ok = buildStreetSpot(rng, tag, street)
		}
		if ok {
			return it
		}
	}
	panic("trainer: spot generation exhausted attempts for " + tag)
}

// spotSetup deals a scripted hand at the classroom stakes with only the
// hero's cards forced; everything else falls out of the seed.
func spotSetup(rng *rand.Rand, seats int, hero, button engine.Seat, hole [2]engine.Card) (*engine.Hand, error) {
	stacks := map[engine.Seat]engine.Chips{}
	for s := 0; s < seats; s++ {
		stacks[engine.Seat(s)] = spotStack
	}
	return engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: spotSmallBlind, BigBlind: spotBigBlind, Seats: seats},
		Button: button,
		Stacks: stacks,
		Holes:  map[engine.Seat][2]engine.Card{hero: hole},
		Seed:   rng.Uint64(),
	})
}

// buildPreflopSpot drives a 6-max hand to the hero's first decision: either
// folded to the hero (an open-or-fold chart spot) or facing a single 3bb
// open (3-bet/call/fold territory).
func buildPreflopSpot(rng *rand.Rand) (Item, bool) {
	hero := engine.Seat(rng.IntN(6))
	button := engine.Seat(rng.IntN(6))
	h, err := spotSetup(rng, 6, hero, button, heroHole(rng))
	if err != nil {
		return Item{}, false
	}

	before := seatsBefore(h, hero)
	raiser := engine.Seat(-1)
	if len(before) > 0 && rng.IntN(2) == 0 {
		raiser = before[rng.IntN(len(before))]
	}
	openTo := 3 * spotBigBlind
	for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != hero {
		seat := h.CurrentSeat()
		var a engine.Action
		if seat == raiser {
			a = engine.Raise{S: seat, To: openTo}
		} else {
			a = engine.Fold{S: seat}
		}
		if h.Apply(a) != nil {
			return Item{}, false
		}
	}
	// "Everyone folds to the big blind" ends the hand before the hero ever
	// acts — no decision, no item; the caller resamples.
	if h.Phase() != engine.PhaseBetting || h.CurrentSeat() != hero {
		return Item{}, false
	}

	// "Folded to you" is only true if somebody actually folded. UTG acts
	// first, so nobody has, and telling a learner otherwise teaches broken
	// position awareness to the exact player trying to learn position.
	desc := "You are first to act"
	if len(before) > 0 {
		desc = "Folded to you"
	}
	if raiser >= 0 {
		desc = h.Position(raiser).String() + " raises to " + openTo.String() + ", folded to you"
	}
	return finishSpot(h, hero, tagSpotPreflop, coach.New(nil, rng.Int64()), desc, "")
}

// buildStreetSpot drives a 3-handed hand passively (calls and checks) to
// the target street, has the first villain lead for a pot fraction, folds
// the other, and hands the decision to the hero — who sits on the button so
// the lead always arrives before the hero acts.
func buildStreetSpot(rng *rand.Rand, tag string, target engine.Street) (Item, bool) {
	button := engine.Seat(rng.IntN(3))
	hero := button
	h, err := spotSetup(rng, 3, hero, button, heroHole(rng))
	if err != nil {
		return Item{}, false
	}

	for h.Phase() == engine.PhaseBetting && h.Street() < target {
		seat := h.CurrentSeat()
		var a engine.Action
		if _, ok := h.LegalActions().Find(engine.ActionCall); ok {
			a = engine.Call{S: seat}
		} else {
			a = engine.Check{S: seat}
		}
		if h.Apply(a) != nil {
			return Item{}, false
		}
	}
	if h.Phase() != engine.PhaseBetting || h.Street() != target {
		return Item{}, false
	}

	bettor := h.CurrentSeat()
	if bettor == hero {
		return Item{}, false
	}
	fracs := [...][2]engine.Chips{{1, 3}, {1, 2}, {2, 3}, {1, 1}}
	fr := fracs[rng.IntN(len(fracs))]
	amt := h.PotTotal() * fr[0] / fr[1]
	if amt < spotBigBlind {
		amt = spotBigBlind
	}
	if amt > h.Stack(bettor) {
		amt = h.Stack(bettor)
	}
	if h.Apply(engine.Bet{S: bettor, Amount: amt}) != nil {
		return Item{}, false
	}
	for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != hero {
		if h.Apply(engine.Fold{S: h.CurrentSeat()}) != nil {
			return Item{}, false
		}
	}
	if h.Phase() != engine.PhaseBetting || h.CurrentSeat() != hero {
		return Item{}, false
	}

	c := coach.New(nil, rng.Int64())
	read := ""
	if tag == tagSpotLate {
		// Level 3 shows an archetype read and tells the coach about it —
		// the read the player sees and the read the grader uses must be
		// the same fact, or the grade would be unfair.
		key := ai.ArchetypeKeys[rng.IntN(len(ai.ArchetypeKeys))]
		p := ai.Archetypes[key]
		c.SetRead(bettor, p)
		read = "Read: " + h.Position(bettor).String() + " plays " + p.Label + "."
	}
	desc := h.Position(bettor).String() + " bets " + amt.String()
	return finishSpot(h, hero, tag, c, desc, read)
}

// finishSpot freezes the coach's advice for the reached decision and
// packages the item. The prompt is assembled purely from engine state.
func finishSpot(h *engine.Hand, hero engine.Seat, tag string, c *coach.Coach, desc, read string) (Item, bool) {
	v := h.View(hero)
	if v == nil || len(v.Legal) == 0 {
		return Item{}, false
	}
	adv := c.Advise(v)
	options, labels, correct := spotChoices(adv)
	if len(options) < 2 {
		return Item{}, false
	}

	line1 := "On the " + v.Street.String() + ": " + desc + ". You are " +
		h.Position(hero).String() + " with " + engine.CardsString(v.Hole[:]) + "."
	if v.Street == engine.Preflop {
		line1 = desc + ". You are " + h.Position(hero).String() +
			" with " + engine.CardsString(v.Hole[:]) + "."
	}
	line2 := "Pot " + v.Pot.String() + ", to call " + v.ToCall.String() +
		", blinds " + v.Blinds.Small.String() + "/" + v.Blinds.Big.String() + "."
	if read != "" {
		line2 = read + " " + line2
	}

	return Item{
		Drill: tutorial.Drill{
			Prompt: line1 + "\n" + line2,
			Visual: &tutorial.Visual{Board: &tutorial.VisualBoard{
				Board: v.Board, Hole: v.Hole, ShowHole: true,
			}},
			Answer:  tutorial.ChoiceAnswer{Choices: labels, Correct: correct},
			Explain: spotExplain(adv),
		},
		SkillTag: tag,
		Spot:     &Spot{Advice: adv, Options: options},
		hero:     engine.CardsString(v.Hole[:]),
		board:    engine.CardsString(v.Board),
	}, true
}

// spotChoices turns the decision's scored candidates into the quiz's
// choices: the best-scoring candidate of each legal action type, in type
// order, with the coach's own pick substituted for its type — so choosing
// the recommended direction always grades Best, and choosing another
// direction is graded at its most charitable sizing.
func spotChoices(adv coach.Advice) (options []engine.Action, labels []string, correct int) {
	best := map[engine.ActionType]ai.ScoredAction{}
	for _, cand := range adv.Decision.Candidates {
		t := cand.Action.Type()
		if cur, ok := best[t]; !ok || cand.ScoreBB > cur.ScoreBB {
			best[t] = cand
		}
	}
	pickType := adv.Decision.Action.Type()
	order := [...]engine.ActionType{
		engine.ActionFold, engine.ActionCheck, engine.ActionCall,
		engine.ActionBet, engine.ActionRaise,
	}
	for _, t := range order {
		sc, ok := best[t]
		if !ok {
			continue
		}
		act := sc.Action
		if t == pickType {
			act, correct = adv.Decision.Action, len(options)
		}
		options = append(options, act)
		labels = append(labels, actionLabel(act))
	}
	return options, labels, correct
}

// actionLabel is the choice text for an action, sized actions included.
func actionLabel(a engine.Action) string {
	switch v := a.(type) {
	case engine.Bet:
		return "Bet " + v.Amount.String()
	case engine.Raise:
		return "Raise to " + v.To.String()
	}
	switch a.Type() {
	case engine.ActionFold:
		return "Fold"
	case engine.ActionCheck:
		return "Check"
	default:
		return "Call"
	}
}

// spotExplain renders the coach's frozen advice: headline, reasoning, and
// the number chips. Everything shown was consumed by the decision — the
// explanation truthfulness invariant, inherited from coach.Explain.
func spotExplain(adv coach.Advice) string {
	s := "Coach: " + adv.Headline
	if adv.Body != "" {
		s += " - " + adv.Body
	}
	if len(adv.Numbers) > 0 {
		parts := make([]string, len(adv.Numbers))
		for i, n := range adv.Numbers {
			parts[i] = n.Label + " " + n.Value
		}
		s += "\n" + strings.Join(parts, " | ")
	}
	return s
}

// seatsBefore lists the seats that act before the hero this round, in
// acting order.
func seatsBefore(h *engine.Hand, hero engine.Seat) []engine.Seat {
	var out []engine.Seat
	dealt := h.DealtIn()
	for s := h.CurrentSeat(); s != hero; s = dealt.Next(s) {
		out = append(out, s)
	}
	return out
}

// heroHole deals the hero's cards with a bias toward decision-rich
// holdings: a third pure random, a third big cards, a third suited
// connectors. Pure random alone deals too much trash whose every answer is
// "fold" — true, but a dull lesson.
func heroHole(rng *rand.Rand) [2]engine.Card {
	switch rng.IntN(3) {
	case 0:
		var used engine.CardSet
		return [2]engine.Card{randCard(rng, &used), randCard(rng, &used)}
	case 1:
		r1 := engine.Ten + engine.Rank(rng.IntN(int(engine.Ace-engine.Ten+1)))
		r2 := engine.Ten + engine.Rank(rng.IntN(int(engine.Ace-engine.Ten+1)))
		s1 := engine.Suit(rng.IntN(engine.NumSuits))
		s2 := engine.Suit(rng.IntN(engine.NumSuits))
		if r1 == r2 && s1 == s2 {
			s2 = engine.Suit((int(s2) + 1) % engine.NumSuits)
		}
		return [2]engine.Card{engine.MakeCard(r1, s1), engine.MakeCard(r2, s2)}
	default:
		r := engine.Two + engine.Rank(rng.IntN(int(engine.King-engine.Two+1)))
		s := engine.Suit(rng.IntN(engine.NumSuits))
		return [2]engine.Card{engine.MakeCard(r, s), engine.MakeCard(r+1, s)}
	}
}
