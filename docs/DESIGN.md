# texas-holdem — Design Index & Canonical Contracts

Three design documents were produced independently and each is authoritative
within its own layer:

| Doc | Owns |
|---|---|
| [`design-engine.md`](design-engine.md) | `internal/engine/`, `internal/eval/` — cards, hand evaluation, betting rules, side pots, table lifecycle |
| [`design-learning.md`](design-learning.md) | `internal/ai/`, `internal/equity/`, `internal/coach/`, `internal/tutorial/`, `internal/trainer/`, `internal/review/`, `internal/profile/` |
| [`design-tui.md`](design-tui.md) | `internal/app/`, `internal/ui/` — screens, table layout, action bar, coach panel, theming |
| [`BRIEF.md`](BRIEF.md) | The product decisions all three were written against |

Because they were written in parallel, they disagree on some shared names and
types. **This document is the tiebreaker.** Where it contradicts a layer doc,
this document wins; everything it does not mention stands as written.

---

## 1. The three ideas that must survive implementation

Everything else is negotiable. These are not.

1. **Grade the decision, not the outcome.** A call with correct pot odds that
   loses is a good call; a bluff that worked with no fold equity was still bad.
   Grades are computed from the hero's information set *before the next card is
   dealt* and are immutable afterwards. The review layer knows the hole cards;
   the grader never does. The headline session statistic is **decision accuracy,
   not chips won**, so the stats screen can plot the two diverging — that
   divergence *is* the variance lesson.

2. **One source of strategic truth.** A single rule-based strategy powers the
   opponents *and* the coach. Opponent archetypes are parameter blocks over that
   one strategy, never separate implementations. If the coach and the bots can
   disagree, the app teaches contradictions.

3. **The strategy explains itself.** `Strategy.Decide` emits typed `Rationale`
   facts as part of deciding; the coach renders those facts and may not cite a
   number the decision did not actually consume. Reverse-engineering an
   explanation from a chosen action (euchre's `tipPlay` approach) does not
   survive NLHE's state space.

---

## 2. Canonical type contracts (conflict resolutions)

### 2.1 Scalars — engine doc wins

```go
type Chips int64   // NOT int
type Seat  int8    // NOT int; -1 means "no seat"
type SeatSet uint8 // NOT []bool
type Card uint8    // rank<<2 | suit, 0..51
type CardSet uint64
```

`design-learning.md` and `design-tui.md` both use plain `int` for chips and
seats throughout. Read those as `engine.Chips` and `engine.Seat`. The TUI must
not define its own chip type.

### 2.2 Hand evaluation — engine doc wins, entirely

`design-learning.md` §2 proposes `Rank7` over a Cactus-Kev 21-subset evaluator.
`design-engine.md` §2 proposes `Eval7` via a direct bitmask/histogram evaluator,
with an explicit budget analysis and a self-describing `HandRank`. **Use the
engine doc's.** Two reasons the learning doc could not see: its own coach
requires `HandRank.Describe()` ("Two Pair, Aces and Nines with a Queen kicker"),
which a dense perfect-hash index cannot provide without a reverse table; and the
engine doc's throughput analysis supports the learning doc's own equity budget.

Canonical names: `Eval5`, `Eval7`, `EvalHoldem`, `Best5`. **`Rank7` does not
exist** — every reference to it in `design-learning.md` means `Eval7`.

`eval` imports `engine` for the `Card` type only. The learning doc's claim that
`eval` has "zero deps" is superseded; the dependency edge is `eval → engine` and
it is acyclic.

### 2.3 Legal actions — one shape

```go
type ActionOption struct {
    Type     ActionType
    Min, Max Chips
}
func (h *Hand) LegalActions() []ActionOption
```

- `design-learning.md`'s `[]ActionSpec` → `[]ActionOption`.
- `design-tui.md`'s `engine.LegalActions` struct with `BetRange{Min,Max}` and
  `CallAmount` → helper methods over the slice:
  `opts.Find(ActionRaise) (ActionOption, bool)`, `opts.CallAmount() Chips`.
  The action bar still never computes legality itself; it just reads a slice
  instead of a struct.

### 2.4 `PlayerView` — a real gap, now filled

`design-learning.md` requires `engine.PlayerView` and rests a correctness claim
on it (bots provably cannot cheat, because opponent hole cards are physically
absent from the value). `design-engine.md` has only read-only accessors on
`Hand`, which *can* reach every seat's cards.

**Both exist, with different audiences:**

| API | Audience | May see opponents' cards |
|---|---|---|
| `Hand` accessors (`HoleCards(seat)`, `Events()`, …) | TUI, review, tests | yes |
| `func (h *Hand) View(seat Seat) *PlayerView` | `ai.Player`, `coach` | **no** |

`PlayerView` is the only thing `ai.Player.Act` ever receives. Its fields follow
`design-learning.md` §1, retyped per §2.1 above, with `Legal []ActionOption`.
A test asserts `PlayerView` contains no card outside `Hole` + `Board`.

### 2.5 Table vs. Session

`engine.Table` (engine doc §5) is canonical. `design-tui.md`'s
`engine.Session` is the same object — the TUI's `Table` *model* holds an
`*engine.Table`. The name collision between the TUI screen model `app.Table` and
the engine's `engine.Table` is tolerable given the package qualifiers, but the
TUI model may be renamed `app.TableScreen` at implementation time if it reads
badly in practice.

### 2.6 Hand history

The engine owns the event log (`design-engine.md` §5.2: `Event`, `EventKind`,
`Hand.Events()`). The persisted wrapper is:

```go
type HandRecord struct {
    ID     string        // ULID
    Time   time.Time
    Blinds Blinds
    Seats  []SeatInfo    // name, personality key ("" = human), starting stack
    Events []Event       // the engine's log, verbatim
}
```

`design-tui.md`'s `engine.HandHistory` → `engine.HandRecord`.
`design-learning.md`'s parallel `Event`/`EventKind` definitions are dropped in
favour of the engine's. Coach annotations ride alongside in `HandAnnotations`
keyed by event index, exactly as the learning doc specifies.

### 2.7 Scripted deals

Engine doc's `HandSetup` + `NewHandFromSetup` + `ScriptedDeck` are canonical.
`design-learning.md`'s `engine.ScriptedDeal` is a lower-level view of the same
capability — implement it as a helper that builds a `HandSetup`.

### 2.8 Coach `Advice` and `Grade`

`design-learning.md` §5 wins on structure (typed `Decision`, `Rationale`, the
five-band `Grade`, `GradedDecision` with `EVLossBB`).
`design-tui.md` §5.2's flatter `Advice`/`Grade` are dropped as types, but its
*rendering needs* are real, so `Advice` carries pre-formatted fields for the
panel:

```go
type Advice struct {
    Decision ai.Decision
    Headline string        // "3-bet to 90"      — ≤40 chars, imperative
    Body     string        // 2–4 sentences
    Numbers  []NumberChip  // {"Pot odds","31%"}, {"Your equity","~34%"}, {"Outs","9"}
}
```

The TUI renders `GradedDecision.Grade` (a `coach.Grade` band) rather than a
letter rune. Grade glyphs are a theme concern, not a data concern.

### 2.9 Coach verbosity — one vocabulary

`Full` / `Mistakes` / `Off`, persisted as `"full" | "mistakes" | "off"`.
`design-learning.md`'s `"grade-only"` is the same mode as `Mistakes`; use
`Mistakes`.

### 2.10 Settings persistence — one package

`internal/profile` holds everything (`design-learning.md` §10). There is **no
`internal/usersettings` package**; `design-tui.md`'s references to it mean
`profile.Profile`, which already carries `CoachMode` and `TableDefaults`.
Location: `$XDG_DATA_HOME/holdem/`, falling back to `~/.local/share/holdem/`.

### 2.11 Opponent selection

`design-tui.md`'s `ai.Difficulty` does not exist. Game setup chooses a
**lineup** of archetypes (`design-learning.md` §4.6); the default is the
"classroom mix": hero plus one each of nit, TAG, LAG, station, maniac.

### 2.12 Lessons package

`internal/tutorial` (learning doc). `design-tui.md`'s single reference to
`internal/lessons` means the same package.

### 2.13 `engine.Position`

Both upper layers reference `engine.Position` (BTN/SB/BB/UTG/HJ/CO); the engine
doc never defines it. It belongs in `internal/engine/table.go` as a derived
value: `func (h *Hand) Position(seat Seat) Position`, computed from the button
and the live-seat count. Position is a function of table state, never stored.

---

## 3. Cross-layer invariants worth a test

- **Chip conservation**: after every `Apply`, `Σ stacks + Σ committed == Σ starting stacks`.
- **No-cheat**: `PlayerView` never contains a card outside `Hole ∪ Board`.
- **Grade immutability**: the grade of a correct call that loses is byte-identical
  to the grade of the same call when it wins.
- **Explanation truthfulness**: every number in coach output is re-derivable from
  the `Rationale` facts the decision consumed.
- **Layout stability**: every anchor in the table view holds position across all
  state mutations, at all three breakpoints.
- **Keybind/legend correspondence**: keys the update loop accepts in a state are
  exactly the keys rendered in that state, from a shared `keymap` source.
- **Script legality**: every scripted lesson hand replays through the real engine
  without an illegal action.
- **Determinism**: same seed → identical bot actions, advice text, and equities.

---

## 4. Known open items

- **Terminal glyph width.** The 4×3 mini card (`│A♠│`) assumes `♠♥♦♣` render one
  cell wide. Some terminals treat them as ambiguous-width and render them two
  cells, which would break every layout in `design-tui.md`. Verify early against
  the target terminals; the `theme.Glyphs` ASCII fallback is the escape hatch,
  and a width probe may need to select it automatically.
- **Preflop equity table generation.** `design-learning.md` §3.3 ships a
  generated 169×169 table via `go generate`. The generator needs the evaluator,
  so it can only be built after `internal/eval` lands.
- **Errata:** `design-tui.md` mockup A originally quoted `3.2:1 (24%)` for a spot
  that is `2.2:1 (31%)` (call 20 into a pot of 45). Corrected in place. Treat
  every hand-written number in the mockups as suspect until a test computes it —
  in a teaching tool a wrong pot-odds figure is the most damaging possible bug.

---

## 5. Build order

Each stage is independently testable and leaves the repo green.

1. **`internal/eval` + engine card/deck primitives.** Exhaustive 5-card census
   and the 21-combo oracle cross-check. Nothing above this can be trusted until
   the evaluator is proven.
2. **`internal/engine` betting + pots.** Street state machine, `LegalActions`,
   `BuildPots`, `HandSetup`. Legality-driven fuzzing plus the worked side-pot
   example. This is the correctness-critical core.
3. **`internal/equity`.** Range parsing, exact enumeration, outs, pot odds; then
   the generated preflop table.
4. **`internal/ai`.** Charts, the baseline strategy, `Rationale`, archetypes.
5. **`internal/app` table screen** against a headless engine + AI — playable,
   coachless. First point the game is real.
6. **`internal/coach`.** Advice, grading, explanations, teachable moments.
7. **`internal/review` + `internal/profile`.** Hand records, replay, persistence.
8. **`internal/tutorial` + `internal/trainer`.** Curriculum and drills last —
   they consume every layer below, and lesson content is cheap to add once the
   scaffolding is proven.

A vertical slice through 1–2–5 is the first milestone worth playing.
