package tutorial_test

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// ScriptedPlayer deliberately does not import ai — a lesson seat should not
// drag the strategy engine in. That leaves its conformance structural, so
// this external test package makes it a compile-time assertion: if ai.Player
// ever gains a method, lessons stop compiling here rather than failing at
// runtime the first time someone opens lesson 5.
var _ ai.Player = (*tutorial.ScriptedPlayer)(nil)

func TestScriptedPlayerActsAsAnAIPlayer(t *testing.T) {
	var p ai.Player = tutorial.NewScriptedPlayer("Villain", []engine.Action{
		engine.Call{S: 1}, engine.Check{S: 1},
	})
	if p.Name() != "Villain" {
		t.Errorf("Name = %q, want %q", p.Name(), "Villain")
	}
}
