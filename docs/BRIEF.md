# Project Brief — texas-holdem

A terminal Texas Hold'em game in Go + Bubble Tea, built **primarily as a learning tool**.
The user (Brandon) is learning poker. Every design decision should be evaluated against
"does this teach the game?" before "is this a faithful poker sim?" — though it must also
be a correct poker sim, because wrong rules teach wrong habits.

## Confirmed product decisions

- **Repo**: `github.com/BrandonDedolph/texas-holdem` (public, MIT)
- **Format**: 6-max No-Limit Hold'em **cash game**. Fixed blinds, rebuy anytime, sit as
  long as you like. Chosen because hands are independent — no ICM/stack-pressure noise
  while learning fundamentals.
- **Learning features (all four are in scope):**
  1. **Live coach during play** — a coach panel that explains pot odds, outs, position,
     and grades each fold/call/raise as you make it.
  2. **Guided lessons** — structured curriculum: hand rankings → position → preflop
     ranges → pot odds & outs → betting patterns, with drills.
  3. **Equity / hand-strength trainer** — standalone drill mode: hand-ranking quizzes,
     outs counting, win-% vs. random hands.
  4. **Post-hand review** — replay the hand with AI hole cards revealed, showing where
     the player gained or lost EV.

## Reference implementation

`../euchre` (same author, same stack) is the template to learn from. Its structure:

```
cmd/euchre/          # entry point (urfave/cli v2 wrapper around the TUI)
internal/
  engine/            # stateless, action-driven game logic (card, deck, game, round, trick, rules)
  ai/                # Player + Strategy interfaces; ai/rule_based/ is the default impl
  app/               # Bubble Tea screens: app.go (root model + NavigateMsg), main_menu,
                     #   game_setup, game_play, quick_reference, learning_journey,
                     #   coach.go, tutorial_popups.go
  tutorial/          # guided lesson system (lesson.go, visual.go, content/)
  ui/components/     # reusable card/table/menu rendering
  ui/theme/          # colors and styles
  variants/          # rule variants behind a Variant interface + init() registry
```

Key euchre patterns worth carrying over:
- Engine is **stateless and action-based**: an `Action` interface with concrete types
  (`PassAction`, `PlayCardAction`, …) applied to game state. This makes the engine
  trivially testable and lets the coach evaluate hypothetical actions.
- The **same rule-based AI drives both the opponents and the coach's recommendation**,
  so there is one source of strategic truth.
- Root Bubble Tea `App` model owns screen navigation via a `NavigateMsg`.
- Teachable-moment popups fire the first time a concept comes up in real play.
- Tests live next to code, heavy on engine + AI decision thresholds.
- There is a layout-stability test and a keybind-legend test — the TUI is tested too.

## Stack

- Go 1.25 (`.tool-versions` pins golang 1.25.5)
- `charmbracelet/bubbletea` + `lipgloss` (euchre also uses `urfave/cli/v2`)
- Standard `go test`; Makefile targets build/run/test/test-race/coverage/lint/clean/deps
- GitHub Actions CI (build, vet, test) + goreleaser on `vX.Y.Z` tags + `install.sh`
- Assets: VHS `.tape` files producing demo GIFs

## What good output looks like

A design document another engineer (or Claude) could implement from directly: concrete Go
package layout, named types with their fields and methods, interfaces with signatures,
state machines with named states and transitions, and — critically — the *reasoning* about
why a shape was chosen. Do not write the implementation. Do not hand-wave the hard parts
(side pots, hand evaluation, betting-round legality, equity computation cost).
