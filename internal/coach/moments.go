package coach

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// Moment is one teachable moment: a named concept that fires the first
// time the player ever meets it, and never again. "First time you saw a
// flush draw" is a once-per-player event, so firing persists to the
// profile forever — not per session (design-learning.md §5.4).
type Moment struct {
	// ID is the persistence key in profile.MomentsSeen.
	ID    string
	Title string

	// Priority breaks ties when one decision triggers several moments:
	// the highest-priority unseen moment fires, the rest wait for their
	// own spot (they recur — a flush draw missed today comes back
	// tomorrow). At most one moment fires per decision.
	Priority int

	// Trigger reports whether this decision is the moment. It may read
	// the view and the advice's rationale freely — a trigger detects a
	// situation, it does not explain a decision, so the truthfulness gate
	// on Explain does not bind it. A nil Trigger marks a review-scoped
	// moment: it needs information the hero lacks at decision time
	// (showdown cards, results) and is fired by the review layer through
	// FireMoment instead.
	Trigger func(v *engine.PlayerView, adv Advice) bool

	// Body renders the popup text. It may cite live cards and numbers
	// from the spot. Review-scoped moments must tolerate (nil, Advice{}).
	Body func(v *engine.PlayerView, adv Advice) string
}

// PendingMoment returns the teachable moment this decision should fire, or
// nil. At most one fires per decision: the highest-priority moment whose
// trigger matches and which the player has never seen. The returned moment
// is immediately marked seen — with a profile attached that mark is
// permanent, so the caller only has to render it.
func (c *Coach) PendingMoment(v *engine.PlayerView, adv Advice) *Moment {
	var best *Moment
	for _, m := range registry {
		if m.Trigger == nil || c.momentSeen(m.ID) {
			continue
		}
		if !m.Trigger(v, adv) {
			continue
		}
		if best == nil || m.Priority > best.Priority {
			best = m
		}
	}
	if best != nil {
		c.markMomentSeen(best.ID)
	}
	return best
}

// FireMoment fires a moment by ID from outside the decision loop — the
// review layer uses it for the review-scoped moments (kicker showdowns,
// domination, split pots), which detect on information the grader never
// sees. Returns nil if the ID is unknown or the moment was already seen.
func (c *Coach) FireMoment(id string) *Moment {
	for _, m := range registry {
		if m.ID != id {
			continue
		}
		if c.momentSeen(id) {
			return nil
		}
		c.markMomentSeen(id)
		return m
	}
	return nil
}

// Moments returns the registry in declaration order, for the UI's concept
// index and for tests. Callers must not mutate the entries.
func Moments() []*Moment { return append([]*Moment(nil), registry...) }

func (c *Coach) momentSeen(id string) bool {
	if c.prof != nil {
		_, seen := c.prof.MomentsSeen[id]
		return seen
	}
	return c.seen[id]
}

func (c *Coach) markMomentSeen(id string) {
	if c.prof != nil {
		c.prof.MarkMomentSeen(id)
		return
	}
	c.seen[id] = true
}

// The registry (design-learning.md §5.4). Priorities put money-now
// warnings (pot commitment, bluffing a station, a 3-bet) above vocabulary
// introductions (naming a draw) — when both apply, the expensive lesson
// wins the popup and the cheap one recurs.
var registry = []*Moment{
	{
		ID: "first_pot_committed", Title: "Pot committed", Priority: 90,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			if v.ToCall <= 0 {
				return false
			}
			behind := v.Stacks[v.Seat] - v.ToCall
			if behind < 0 {
				behind = 0
			}
			return behind < v.Pot+v.ToCall
		},
		Body: staticBody("Calling here leaves you less than a pot behind — you are nearly pot-committed. " +
			"Decide now whether you are willing to play for the rest of your stack."),
	},
	{
		ID: "first_station_bluff_warning", Title: "Never bluff a calling station", Priority: 85,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			f, ok := adv.Decision.Rationale.Get(ai.FactArchetype)
			return ok && strings.Contains(f.(ai.ArchetypeFact).Note, "don't bluff")
		},
		Body: staticBody("This opponent calls with almost anything. A bluff only makes money when they can fold — " +
			"against a calling station, skip the bluffs and value-bet your good hands bigger instead."),
	},
	{
		ID: "first_facing_3bet", Title: "Facing a 3-bet", Priority: 80,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			return v.Street == engine.Preflop && v.ToCall > 0 && heroRaisedAndWasReraised(v)
		},
		Body: staticBody("You raised and got re-raised — a 3-bet. That usually means a premium hand: " +
			"continue with your strongest holdings and fold the rest without regret."),
	},
	{
		ID: "first_great_price", Title: "A great price", Priority: 75,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			f, ok := adv.Decision.Rationale.Get(ai.FactPotOdds)
			if !ok {
				return false
			}
			po := f.(ai.PotOddsFact)
			// 4:1 or better — required equity of 20% or less.
			return po.Required > 0 && po.Required <= 0.20
		},
		Body: func(v *engine.PlayerView, adv Advice) string {
			f, ok := adv.Decision.Rationale.Get(ai.FactPotOdds)
			if !ok {
				return "You are being offered four-to-one or better. Prices this good make even weak draws profitable calls."
			}
			po := f.(ai.PotOddsFact)
			return "You are getting " + equity.OddsToOne(po.ToCall, po.Pot) + " — you only need " +
				pctStr(po.Required) + " equity to call. Prices this good make even weak draws profitable."
		},
	},
	{
		ID: "first_counterfeit", Title: "Counterfeited", Priority: 70,
		Trigger: func(v *engine.PlayerView, adv Advice) bool { return counterfeited(v) },
		Body: staticBody("The board just paired over your two pair — your lower pair no longer plays. " +
			"That is a counterfeit: the board took back a piece of your hand."),
	},
	{
		ID: "first_semibluff", Title: "The semi-bluff", Priority: 65,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			f, ok := adv.Decision.Rationale.Get(ai.FactSizing)
			return ok && f.(ai.SizingFact).Purpose == "semi-bluff"
		},
		Body: staticBody("Betting a draw is a semi-bluff: you can win the pot right now, and when called " +
			"you still have outs to the best hand. Two ways to win beats one."),
	},
	{
		ID: "first_flush_draw", Title: "Your first flush draw", Priority: 60,
		Trigger: drawTrigger(ai.FlushDraw),
		Body: staticBody("Four cards to a flush — nine more of that suit complete it. From the flop, " +
			"a flush draw gets there by the river about 36% of the time (rule of 4: 9×4)."),
	},
	{
		ID: "first_oesd", Title: "Open-ended straight draw", Priority: 55,
		Trigger: drawTrigger(ai.OESD),
		Body: staticBody("Your straight can complete at either end — eight outs. " +
			"Rule of 4: from the flop that is about 32% by the river."),
	},
	{
		ID: "first_gutshot", Title: "The gutshot", Priority: 50,
		Trigger: drawTrigger(ai.Gutshot),
		Body: staticBody("Only one rank fills this straight — four outs, half an open-ender. " +
			"Rule of 4 says about 16%, so a gutshot usually needs a great price to continue."),
	},
	{
		ID: "first_cbet_spot", Title: "The continuation bet", Priority: 45,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			if v.Street != engine.Flop || v.ToCall != 0 {
				return false
			}
			f, ok := adv.Decision.Rationale.Get(ai.FactInitiative)
			return ok && f.(ai.InitiativeFact).HeroIsAggressor
		},
		Body: staticBody("You raised before the flop, so the story is yours to continue. " +
			"A continuation bet often wins the pot right here — most flops miss most hands."),
	},
	{
		ID: "first_button_raise_hand", Title: "The button raise", Priority: 40,
		Trigger: func(v *engine.PlayerView, adv Advice) bool {
			if v.Street != engine.Preflop {
				return false
			}
			f, ok := adv.Decision.Rationale.Get(ai.FactChart)
			return ok && f.(ai.ChartFact).InRange && f.(ai.ChartFact).Chart == "BTN open"
		},
		Body: staticBody("On the button you act last on every street after the flop. That advantage is " +
			"worth money, which is why the button raising chart is the widest in the game."),
	},

	// Archetype-read moments: the one-word seat-plate reads ("tight",
	// "sticky"), each explained once, the first time the behaviour behind
	// the word is visible at the table — the station's big call-down, the
	// nit's rare raise. Like the review-scoped moments below they carry no
	// decision-time trigger, for the same structural reason: detection
	// needs information this package does not hold (the seat-to-archetype
	// map), so the table fires them through FireMoment when the hero's
	// turn arms (table.go readMomentCandidates). IDs are "read_" + the
	// Personality.Read word; a test pins one moment per archetype read.
	{
		ID: "read_tight", Title: "The read: \"tight\" (the nit)", Priority: 39,
		Body: staticBody("That raise came from the tightest player at the table. \"tight\" means they " +
			"play very few hands - when they finally raise, it is the top of the deck. " +
			"Fold good hands without regret, and steal their blinds while they wait."),
	},
	{
		ID: "read_solid", Title: "The read: \"solid\" (tight-aggressive)", Priority: 38,
		Body: staticBody("\"solid\" marks a tight-aggressive player - the disciplined, positionally " +
			"aware game you are learning to play. Their raises mean what they say: " +
			"respect them, and pick your fights with the looser seats instead."),
	},
	{
		ID: "read_loose", Title: "The read: \"loose\" (loose-aggressive)", Priority: 37,
		Body: staticBody("\"loose\" means this player raises far more hands than the chart allows, " +
			"so each raise means less. Call down lighter than you would against a " +
			"solid player - but with a real hand, not out of stubbornness."),
	},
	{
		ID: "read_sticky", Title: "The read: \"sticky\" (the calling station)", Priority: 36,
		Body: staticBody("That call-down is the read made visible. \"sticky\" means a calling " +
			"station: they call with almost anything. A bluff only makes money when " +
			"they can fold, so never bluff them - value-bet thinner and bigger instead."),
	},
	{
		ID: "read_wild", Title: "The read: \"wild\" (the maniac)", Priority: 35,
		Body: staticBody("\"wild\" is the maniac: constant, oversized raises, cards optional. Do not " +
			"try to out-bluff chaos - tighten up, wait for a real hand, and let them " +
			"pay you off."),
	},

	// Review-scoped moments: these detect on showdown cards or results —
	// information the hero does not have at decision time — so they have
	// no decision-time trigger and are fired by the review layer.
	{
		ID: "first_kicker_showdown", Title: "The kicker decides", Priority: 30,
		Body: staticBody("When two players pair the same card, the side card — the kicker — decides it. " +
			"This is why big cards make big hands and weak kickers bleed money."),
	},
	{
		ID: "first_dominated", Title: "Dominated", Priority: 25,
		Body: staticBody("Your hand shared a card with a better hand — you were dominated. " +
			"Dominated hands make second-best hands, and second-best is where the money goes."),
	},
	{
		ID: "first_split_pot", Title: "The split pot", Priority: 20,
		Body: staticBody("Same five cards, same hand — the pot splits. " +
			"When the board plays for everyone, nobody wins more than their share back."),
	},
}

// staticBody wraps fixed popup text; review-scoped moments use it so a nil
// view can never panic.
func staticBody(s string) func(*engine.PlayerView, Advice) string {
	return func(*engine.PlayerView, Advice) string { return s }
}

// drawTrigger fires when the decision classified the hero on exactly the
// given draw. It reads the ClassFact the decision consumed, so the popup
// can never name a draw the strategy did not see.
func drawTrigger(d ai.DrawClass) func(*engine.PlayerView, Advice) bool {
	return func(v *engine.PlayerView, adv Advice) bool {
		f, ok := adv.Decision.Rationale.Get(ai.FactClass)
		return ok && f.(ai.ClassFact).Draw == d
	}
}

// heroRaisedAndWasReraised scans the visible preflop action for the 3-bet
// shape: the hero raised, and someone raised after.
func heroRaisedAndWasReraised(v *engine.PlayerView) bool {
	heroRaised := false
	for _, e := range v.History {
		if e.Kind != engine.EvAction || e.Street != engine.Preflop {
			continue
		}
		if e.Action != engine.ActionRaise && e.Action != engine.ActionBet {
			continue
		}
		if e.Seat == v.Seat {
			heroRaised = true
		} else if heroRaised {
			return true
		}
	}
	return false
}

// counterfeited detects two pair losing a piece to the board: both hole
// cards had paired the earlier board, and the newest card pairs the board
// at a rank above the hero's lower pair without touching the hero's ranks.
func counterfeited(v *engine.PlayerView) bool {
	n := len(v.Board)
	if n < 4 {
		return false
	}
	r1, r2 := v.Hole[0].Rank(), v.Hole[1].Rank()
	if r1 == r2 {
		return false
	}
	prev, last := v.Board[:n-1], v.Board[n-1]
	if !boardHasRank(prev, r1) || !boardHasRank(prev, r2) {
		return false
	}
	lr := last.Rank()
	if lr == r1 || lr == r2 {
		return false // the card improved the hero instead
	}
	pairs := 0
	for _, c := range v.Board {
		if c.Rank() == lr {
			pairs++
		}
	}
	lo := r1
	if r2 < lo {
		lo = r2
	}
	return pairs >= 2 && lr > lo
}

func boardHasRank(board []engine.Card, r engine.Rank) bool {
	for _, c := range board {
		if c.Rank() == r {
			return true
		}
	}
	return false
}
