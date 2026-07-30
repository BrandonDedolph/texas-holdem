package tutorial

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// headsUpScript is a minimal legal scripted hand: button (hero) limps,
// big blind checks, both check every street to showdown.
func headsUpScript() *ScriptedHand {
	return &ScriptedHand{
		Hero:       0,
		Button:     0,
		SmallBlind: 5,
		BigBlind:   10,
		Stacks:     map[engine.Seat]engine.Chips{0: 1000, 1: 1000},
		Holes: map[engine.Seat][2]engine.Card{
			0: engine.Holes("As Ah"),
			1: engine.Holes("Kd Kc"),
		},
		Board: engine.MustCards("Qc Jd 7h 3s 2d"),
		Seed:  1,
		Seats: []ScriptSeat{
			{Seat: 0, Name: "Hero", Actions: []engine.Action{
				engine.Call{S: 0},
				engine.Check{S: 0}, engine.Check{S: 0}, engine.Check{S: 0},
			}},
			{Seat: 1, Name: "Villain", Actions: []engine.Action{
				engine.Check{S: 1},
				engine.Check{S: 1}, engine.Check{S: 1}, engine.Check{S: 1},
			}},
		},
		Stops: []Stop{
			{AtDecision: 0, Teach: "limp behind", Expect: engine.Call{S: 0}},
		},
	}
}

func TestReplayLegalScript(t *testing.T) {
	s := headsUpScript()
	h, err := s.Replay()
	if err != nil {
		t.Fatalf("Replay() error: %v", err)
	}
	res, ok := h.Result()
	if !ok {
		t.Fatal("replayed hand has no result")
	}
	if !res.Showdown {
		t.Error("check-down hand did not reach showdown")
	}
	// Aces beat kings on a dry board; hero nets the villain's 10.
	if res.Net[0] != 10 || res.Net[1] != -10 {
		t.Errorf("net = %d/%d, want +10/-10", res.Net[0], res.Net[1])
	}
}

func TestReplayCatchesDrift(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ScriptedHand)
		want   string
	}{
		{
			name: "illegal action",
			mutate: func(s *ScriptedHand) {
				// Heads-up the button owes the small-blind difference; a
				// preflop Check is illegal.
				s.Seats[0].Actions[0] = engine.Check{S: 0}
			},
			want: "check",
		},
		{
			name: "script runs dry",
			mutate: func(s *ScriptedHand) {
				s.Seats[1].Actions = s.Seats[1].Actions[:1]
			},
			want: "no scripted action left",
		},
		{
			name: "leftover actions",
			mutate: func(s *ScriptedHand) {
				s.Seats[1].Actions = append(s.Seats[1].Actions, engine.Check{S: 1})
			},
			want: "left over",
		},
		{
			name: "stop past the hero's last decision",
			mutate: func(s *ScriptedHand) {
				s.Stops = []Stop{{AtDecision: 99}}
			},
			want: "decision",
		},
		{
			name: "stop expecting an action the script does not play",
			mutate: func(s *ScriptedHand) {
				s.Stops = []Stop{{AtDecision: 0, Expect: engine.Fold{S: 0}}}
			},
			want: "expects",
		},
		{
			name: "seat scripted twice",
			mutate: func(s *ScriptedHand) {
				s.Seats = append(s.Seats, ScriptSeat{Seat: 1})
			},
			want: "twice",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := headsUpScript()
			tc.mutate(s)
			_, err := s.Replay()
			if err == nil {
				t.Fatal("Replay() = nil error, want drift failure")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("Replay() error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestScriptedPlayerReplaysAndPanicsOnDrift(t *testing.T) {
	p := NewScriptedPlayer("Bot", []engine.Action{
		engine.Check{S: 1},
		engine.Bet{S: 1, Amount: 20},
	})
	if p.Name() != "Bot" {
		t.Errorf("Name() = %q", p.Name())
	}
	view := &engine.PlayerView{Seat: 1}
	if a := p.Act(view); a != (engine.Check{S: 1}) {
		t.Errorf("first Act = %v", a)
	}
	if p.Remaining() != 1 {
		t.Errorf("Remaining() = %d, want 1", p.Remaining())
	}
	if a := p.Act(view); a != (engine.Bet{S: 1, Amount: 20}) {
		t.Errorf("second Act = %v", a)
	}
	defer func() {
		if recover() == nil {
			t.Error("Act past the end of the script did not panic")
		}
	}()
	p.Act(view)
}

func TestScriptedPlayerPanicsOnWrongSeat(t *testing.T) {
	p := NewScriptedPlayer("Bot", []engine.Action{engine.Check{S: 1}})
	defer func() {
		if recover() == nil {
			t.Error("Act for the wrong seat did not panic")
		}
	}()
	p.Act(&engine.PlayerView{Seat: 2})
}
