package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Animation & pacing (docs/design-tui.md §6). The rule is euchre's: named
// tick messages per animation, one constants block for every duration, and a
// per-Speed factor multiplied in at the last moment. The poker-specific
// principle: animate information arrival, not decoration — every delay here
// exists to give the eye time to register a state change that matters.
//
// SpeedInstant short-circuits everything: scaled() returns zero and the
// table's queue drains synchronously without ever constructing a tea.Tick.
// That single property is what makes the golden renders deterministic.

// stepTickMsg advances the table's presentation queue by one step. seq
// guards against stale ticks: the table bumps its sequence number whenever
// it reschedules, so a tick from an abandoned timeline is ignored.
type stepTickMsg struct{ seq int }

// villainTickMsg fires when a villain's think time elapses and it should
// act. Same seq discipline as stepTickMsg.
type villainTickMsg struct{ seq int }

// Base durations, tuned for SpeedLearn (the slowest speed). Everything else
// is these times a factor.
const (
	durBoardCard  = 350 * time.Millisecond  // per board card flip
	durStreetBeat = 900 * time.Millisecond  // post-street beat when Learn's hard pause is off
	durActionBeat = 300 * time.Millisecond  // beat after an action label lands
	durReveal     = 900 * time.Millisecond  // per showdown reveal
	durAward      = 1300 * time.Millisecond // per pot awarded
	durThinkSmall = 1600 * time.Millisecond // villain think, trivial decision
	durThinkBig   = 2600 * time.Millisecond // villain think, facing chips
	durThinkFast  = 300 * time.Millisecond  // flat think time at SpeedFast
)

// speedFactor scales the base durations per Speed. SpeedInstant's zero is
// load-bearing: it is the test hook (§6.3).
func speedFactor(s Speed) float64 {
	switch s {
	case SpeedNormal:
		return 0.55
	case SpeedFast:
		return 0.35
	case SpeedInstant:
		return 0
	default: // SpeedLearn
		return 1
	}
}

// scaled applies the speed factor to a base duration.
func scaled(d time.Duration, s Speed) time.Duration {
	return time.Duration(float64(d) * speedFactor(s))
}

// thinkTime is a villain's decision delay. It is deliberately deterministic
// (no randomness — same seed, same replay, byte-identical goldens) but
// scaled by decision size: facing chips takes longer than an open fold,
// which models the tell-free rhythm of online play and gives the player
// reading time exactly when the situation is complex.
func thinkTime(s Speed, facingBet bool) time.Duration {
	if s == SpeedFast {
		return durThinkFast
	}
	base := durThinkSmall
	if facingBet {
		base = durThinkBig
	}
	return scaled(base, s)
}

// stepDelay is the pause after a presentation step lands, before the next
// one is applied.
func stepDelay(k stepKind, s Speed) time.Duration {
	var base time.Duration
	switch k {
	case stepBoard:
		base = durBoardCard
	case stepReveal:
		base = durReveal
	case stepAward:
		base = durAward
	default:
		base = durActionBeat
	}
	return scaled(base, s)
}

// tickMsgAfter schedules an arbitrary message after a delay.
func tickMsgAfter(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}
