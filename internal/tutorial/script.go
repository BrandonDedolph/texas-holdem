package tutorial

import (
	"fmt"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// ScriptedHand is a lesson hand that runs in the REAL engine: it builds an
// engine.HandSetup so the exact teaching spot is dealt every time (lesson 9
// deals the hero 9♠8♠ on A♠K♠2♥, always), and every seat's actions replay
// through Hand.Apply. There is no parallel simulation — the lesson runs on
// the same rendering and the same rules as a live game.
//
// The hero's ScriptSeat carries the lesson's recommended line; in the app the
// hero acts freely (or is gated by Stop.Expect), but the scripted line is
// what Replay validates, so a script that drifts out of sync with engine
// legality fails the content test rather than falling back at runtime.
type ScriptedHand struct {
	Hero   engine.Seat
	Button engine.Seat

	SmallBlind, BigBlind engine.Chips

	Stacks map[engine.Seat]engine.Chips
	Holes  map[engine.Seat][2]engine.Card // seats without an entry get seeded cards
	Board  []engine.Card                  // the runout prefix, 0..5 cards
	Seed   uint64                         // fills every unscripted card

	Seats []ScriptSeat // every dealt-in seat, hero included
	Stops []Stop       // where the lesson pauses and teaches

	Intro   string // shown before the hand is dealt
	Debrief string // shown at hand end
}

// ScriptSeat is one seat's fixed action list, consumed in engine turn order.
// A mismatch with the engine's turn sequence is a test failure, never a
// runtime fallback (docs/DESIGN.md §3, script legality).
type ScriptSeat struct {
	Seat    engine.Seat
	Name    string
	Actions []engine.Action
}

// Stop pauses the lesson at the hero's Nth decision (0-based) and shows
// Teach in the coach box instead of normal advice. If Expect is non-nil,
// only that action advances (drill-style); otherwise any legal action is
// accepted and graded normally.
//
// TODO(wire-coach): once internal/coach lands, a Stop without Teach text
// could fall through to live coach commentary; today Teach is the lesson's
// own voice.
type Stop struct {
	AtDecision int
	Teach      string
	Expect     engine.Action
}

// Setup builds the engine.HandSetup this scripted hand deals from.
func (s *ScriptedHand) Setup() engine.HandSetup {
	stacks := make(map[engine.Seat]engine.Chips, len(s.Stacks))
	for seat, c := range s.Stacks {
		stacks[seat] = c
	}
	holes := make(map[engine.Seat][2]engine.Card, len(s.Holes))
	for seat, h := range s.Holes {
		holes[seat] = h
	}
	board := make([]engine.Card, len(s.Board))
	copy(board, s.Board)
	return engine.HandSetup{
		Config: engine.TableConfig{
			SmallBlind: s.SmallBlind,
			BigBlind:   s.BigBlind,
			Seats:      engine.MaxSeats,
		},
		Button: s.Button,
		Stacks: stacks,
		Holes:  holes,
		Board:  board,
		Seed:   s.Seed,
		Eval:   eval.Evaluator,
	}
}

// Deal builds the hand in preflop betting, blinds posted, teaching cards laid.
func (s *ScriptedHand) Deal() (*engine.Hand, error) {
	return engine.NewHandFromSetup(s.Setup())
}

// Players builds one ScriptedPlayer per scripted seat, for wiring the lesson
// into the game screen. The hero's seat is included; the app substitutes the
// human for it.
func (s *ScriptedHand) Players() map[engine.Seat]*ScriptedPlayer {
	out := make(map[engine.Seat]*ScriptedPlayer, len(s.Seats))
	for _, ss := range s.Seats {
		out[ss.Seat] = NewScriptedPlayer(ss.Name, ss.Actions)
	}
	return out
}

// Replay deals the hand and applies every scripted action in the engine's
// turn order until the hand completes, then validates the stops. It returns
// the completed hand.
//
// Any divergence is an error: an illegal action, a seat asked to act with no
// scripted action left, actions left over after the hand ends, a stop
// pointing past the hero's last decision, or a stop whose Expect disagrees
// with the scripted hero line. The content test calls this for every
// scripted hand, so engine changes that break a script are caught at test
// time.
func (s *ScriptedHand) Replay() (*engine.Hand, error) {
	h, err := s.Deal()
	if err != nil {
		return nil, err
	}

	queues := make(map[engine.Seat][]engine.Action, len(s.Seats))
	for _, ss := range s.Seats {
		if _, dup := queues[ss.Seat]; dup {
			return nil, fmt.Errorf("tutorial: seat %d scripted twice", ss.Seat)
		}
		queues[ss.Seat] = ss.Actions
	}

	var heroLine []engine.Action
	for h.Phase() == engine.PhaseBetting {
		seat := h.CurrentSeat()
		if seat == engine.NoSeat {
			break
		}
		q := queues[seat]
		if len(q) == 0 {
			return nil, fmt.Errorf("tutorial: seat %d to act on the %s with no scripted action left",
				seat, h.Street())
		}
		a := q[0]
		queues[seat] = q[1:]
		if a.Seat() != seat {
			return nil, fmt.Errorf("tutorial: scripted action for seat %d while seat %d acts",
				a.Seat(), seat)
		}
		if seat == s.Hero {
			heroLine = append(heroLine, a)
		}
		if err := h.Apply(a); err != nil {
			return nil, fmt.Errorf("tutorial: seat %d %s on the %s: %w",
				seat, a.Type(), h.Street(), err)
		}
	}

	if h.Phase() != engine.PhaseComplete {
		return nil, fmt.Errorf("tutorial: script ended with the hand in phase %d", h.Phase())
	}
	for seat, q := range queues {
		if len(q) > 0 {
			return nil, fmt.Errorf("tutorial: seat %d has %d scripted action(s) left over", seat, len(q))
		}
	}
	for i, stop := range s.Stops {
		if stop.AtDecision < 0 || stop.AtDecision >= len(heroLine) {
			return nil, fmt.Errorf("tutorial: stop %d at decision %d, hero made %d decisions",
				i, stop.AtDecision, len(heroLine))
		}
		if stop.Expect != nil && stop.Expect != heroLine[stop.AtDecision] {
			return nil, fmt.Errorf("tutorial: stop %d expects %s but the scripted line plays %s",
				i, stop.Expect.Type(), heroLine[stop.AtDecision].Type())
		}
	}
	return h, nil
}

// ScriptedPlayer replays a fixed action list. It satisfies ai.Player
// structurally (Act(*engine.PlayerView) engine.Action, Name() string) without
// importing the ai package, so a lesson seat plugs into the game loop exactly
// like a bot.
type ScriptedPlayer struct {
	name    string
	actions []engine.Action
	next    int
}

// NewScriptedPlayer builds a player that plays actions in order.
func NewScriptedPlayer(name string, actions []engine.Action) *ScriptedPlayer {
	return &ScriptedPlayer{name: name, actions: actions}
}

// Name returns the seat's display name.
func (p *ScriptedPlayer) Name() string { return p.name }

// Remaining returns how many scripted actions are left.
func (p *ScriptedPlayer) Remaining() int { return len(p.actions) - p.next }

// Act returns the next scripted action. Exhausting the script or acting for
// the wrong seat means the script has drifted from engine legality — a
// programmer error the content tests exist to catch — so it panics rather
// than inventing a fallback action.
func (p *ScriptedPlayer) Act(v *engine.PlayerView) engine.Action {
	if p.next >= len(p.actions) {
		panic(fmt.Errorf("tutorial: %s (seat %d) asked to act with no scripted action left",
			p.name, v.Seat))
	}
	a := p.actions[p.next]
	if a.Seat() != v.Seat {
		panic(fmt.Errorf("tutorial: %s holds an action for seat %d but acts as seat %d",
			p.name, a.Seat(), v.Seat))
	}
	p.next++
	return a
}
