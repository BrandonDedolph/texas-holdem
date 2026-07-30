# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A terminal Texas Hold'em game in Go using Bubble Tea. Format: 6-max No-Limit
cash game vs. AI opponents.

**This is a learning tool first and a poker simulator second.** The owner is
learning the game. Every design decision is evaluated against "does this teach
poker?" before "is this a faithful sim?" — though it must also be a correct sim,
because wrong rules teach wrong habits.

The project is currently **design-complete, implementation-empty**. Read
`docs/DESIGN.md` first; it indexes the three layer designs and is the tiebreaker
where they disagree.

## Build Commands

```bash
make build      # Build binary to bin/holdem
make run        # Run the application
make test       # Run all tests with verbose output
make lint       # Run golangci-lint
make coverage   # Generate test coverage report
```

Run a single test:
```bash
go test -v -run TestName ./internal/engine/
```

## Documentation map

| File | Authority over |
|---|---|
| `docs/DESIGN.md` | **Read first.** Cross-layer contracts; wins over the layer docs where they conflict |
| `docs/BRIEF.md` | Product decisions (format, scope, learning features) |
| `docs/design-engine.md` | `internal/engine/`, `internal/eval/` |
| `docs/design-learning.md` | `internal/ai/`, `internal/equity/`, `internal/coach/`, `internal/tutorial/`, `internal/trainer/`, `internal/review/`, `internal/profile/` |
| `docs/design-tui.md` | `internal/app/`, `internal/ui/` |

## The three non-negotiable principles

1. **Grade the decision, not the outcome.** A call with correct pot odds that
   loses is a good call. Grades are computed from the hero's information set
   before the next card is dealt, and are immutable afterwards. The grader never
   sees opponents' hole cards; the review layer does, and renders hindsight as a
   visually distinct layer that never rewrites a grade.
2. **One source of strategic truth.** One rule-based strategy drives both the
   opponents and the coach. Archetypes are parameter blocks over it, never
   separate implementations.
3. **The strategy explains itself.** `Strategy.Decide` emits typed `Rationale`
   facts while deciding; the coach renders them and may not cite a number the
   decision did not consume. Never reverse-engineer an explanation from a chosen
   action.

## Architecture

Dependency direction is strictly upward; nothing below imports anything above.

```
internal/app, internal/ui          TUI (Bubble Tea screens, table layout, theming)
internal/tutorial, trainer, review lessons, drills, post-hand replay
internal/coach                     advice, grading, explanations, teachable moments
internal/ai                        Player/Strategy, rule-based baseline, archetypes
internal/equity                    ranges, enumeration, outs, pot odds
internal/engine  internal/eval     cards, betting rules, side pots  |  hand evaluation
internal/profile                   persistence (XDG), used by most layers
```

### Engine (`internal/engine/`)

Stateless and action-driven, like euchre's engine: `Action` interface,
`Hand.Apply(action) error`, and `LegalActions() []ActionOption` as the single
source of truth for legality. Key divergences from a naive implementation:

- `Card uint8` (rank<<2|suit), card sets are `uint64` bitmasks — the equity code
  needs the throughput.
- **Pots are derived, not maintained**: `BuildPots` is a pure function of per-seat
  contributions plus the folded set. Never mutate a pot tree incrementally.
- Blinds are engine events, not `Action`s — an `Action` is always a *choice*.
- `Raise{To:}` is raise-**to**, not raise-by.
- `HandSetup`/`NewHandFromSetup` builds an exact scenario (fixed holes, board,
  stacks) so lessons can force a teaching spot.

### Evaluation (`internal/eval/`)

Direct bitmask/histogram 7-card evaluator. `HandRank uint32` packs category plus
kickers so that integer comparison *is* hand comparison **and** the value decodes
back to English via `Describe()`. Do not swap in a dense perfect-hash index — the
coach needs the description.

### AI, coach, learning

See `docs/design-learning.md`. `ai.Player.Act` receives an `engine.PlayerView`
that physically excludes opponents' hole cards — this is a correctness property
with a test, not a convention.

### TUI (`internal/app/`)

Bubble Tea Elm architecture with euchre's `NavigateMsg` routing, plus
session-preserving navigation (the table model is cached so hand review can
return to a live game). Target 80×24; wide layout ≥104 cols; compact ledger
≥60×20; `renderTooSmall` below that.

## Conventions

- No hex color literal outside `internal/ui/theme/palette.go`; no raw Unicode
  glyph outside `glyphs.go`. Every glyph has a same-width ASCII fallback.
- Cards in tests and lesson content are written as `"As Kd"` strings, never raw
  byte values.
- Every region of the table view has a fixed row budget and renders blank when
  empty — the layout must never reflow.
- Chips are `engine.Chips` (int64) everywhere; seats are `engine.Seat` (int8).

## Testing

Standard `go test`. The load-bearing tests:

- Exhaustive 5-card census + 21-combo oracle cross-check for the evaluator.
- Chip conservation after every applied action.
- Legality-driven fuzzing of the betting engine (pick uniformly from
  `LegalActions` until the hand completes) — highest-value test in the repo.
- Anti-resulting: the grade of a correct call that loses must be byte-identical
  to the same call when it wins.
- Explanation truthfulness: no number in coach output that isn't derivable from
  the rationale.
- Layout stability and keybind/legend correspondence at all three breakpoints.

## Reference

`../euchre` is a sibling project by the same author on the same stack. Its
`internal/app` navigation, coach box, teachable popups, layout-stability tests,
and build tooling are the patterns this project follows. Where this project
diverges, `docs/DESIGN.md` says why.
