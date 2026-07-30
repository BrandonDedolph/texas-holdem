# Design: Learning System & AI Opponents

Scope: `internal/ai/`, `internal/equity/`, `internal/coach/`, `internal/tutorial/`,
`internal/trainer/`, `internal/review/`, `internal/profile/`, plus the small engine
surfaces they require (`PlayerView`, scripted deals, the hand-history event log).

This document assumes the engine design (actions, betting legality, side pots,
showdown) exists separately; where this doc needs an engine type it states the
required shape and why.

---

## 0. Principles (read these before arguing with any decision below)

1. **One source of strategic truth.** The same rule-based strategy that drives the
   opponents drives the coach's recommendation, exactly as in euchre. The coach is
   literally "what would the baseline bot do in your seat, and why". If the coach
   and the bots disagree, the app teaches contradictions.

2. **Grade the decision, not the outcome.** A call with correct pot odds that loses
   is a good call. A bluff that got a fold but had no fold equity was still a bad
   bluff. Every grading path in this design is computed *before the next card is
   dealt*, from the hero's information set only, and is immutable afterwards. The
   post-hand review then deliberately juxtaposes grade vs. result to hammer the
   lesson home. This is the single most valuable thing the app can teach and it is
   structural, not cosmetic: grades are stored on the decision event at decision
   time, and the review renderer is forbidden (by design and by test) from
   recomputing them with hole-card knowledge.

3. **The strategy explains itself.** Euchre's coach reverse-engineers a narrative
   from the card the AI picked (`tipPlay` pattern-matches the choice). That worked
   for 5-card euchre; it will not survive NLHE's state space. Here the strategy
   emits a structured `Rationale` (typed facts: pot odds, outs, position, range
   read) *as part of deciding*, and the coach only renders it. No second guessing,
   no drift between what the bot did and what the coach says.

4. **Numbers everywhere, but always with the shortcut.** Every equity number the
   coach shows is paired with the human-computable path to it ("9 outs × 4 ≈ 36%").
   The app's job is to make Brandon able to do this at a real table without the app.

5. **Bots must not cheat, provably.** AI players receive a `PlayerView` that
   physically cannot contain opponents' hole cards. This is both fairness and a
   teaching claim we can make honestly in the UI.

---

## 1. Package & file layout

```
internal/
  eval/                     # 7-card hand evaluator (pure, zero deps)
    eval.go                 #   Rank7, HandRank, Category
    tables_gen.go           #   generated lookup tables (go:generate)
    eval_test.go
  equity/                   # numeric backbone
    range.go                #   Range type, combos, weights
    parse.go                #   "JJ+, AQs+, KQo" -> Range
    equity.go               #   HandVsHand, HandVsRange, entry points + budget logic
    enumerate.go            #   exact enumeration backends
    montecarlo.go           #   sampled backend (deterministic seed)
    preflop_table_gen.go    #   generated 169x169 preflop matchup table
    outs.go                 #   outs counting + draw classification
    odds.go                 #   pot odds / required equity / rule of 2&4 helpers
  ai/
    player.go               #   Player interface, seat wiring
    strategy.go             #   Strategy interface, Decision, ScoredAction
    rationale.go            #   typed fact structs shared with coach
    personality.go          #   Personality parameter block
    archetypes.go           #   Nit/TAG/LAG/Station/Maniac/Coach presets
    rulebased/
      ai.go                 #   glue: Personality + Strategy -> Player
      charts.go             #   preflop range charts (RFI, defend, 3bet) as data
      preflop.go            #   preflop decision logic
      handclass.go          #   postflop made-hand + draw classification
      postflop.go           #   postflop decision logic (cbet, value, bluff, call)
      sizing.go             #   bet-sizing tables + human rounding
      perception.go         #   villain range assignment from the action line
  coach/
    coach.go                #   Advise() — recommendation for hero's spot
    grade.go                #   Grade() — graded scale + EV-loss bands
    explain.go              #   Rationale -> English templates
    moments.go              #   teachable-moment registry + detectors
  tutorial/
    lesson.go               #   Lesson/Section types, registry (euchre pattern)
    script.go               #   ScriptedDeal + ScriptedPlayer for forced spots
    drill.go                #   in-lesson checked exercises
    visual.go               #   poker visuals (board, range grid, table seats)
    content/
      c01_hand_rankings.go … c12_bluffing.go   # one file per lesson
  trainer/
    trainer.go              #   drill session loop, scoring
    quiz_rankings.go        #   hand-ranking speed quiz
    quiz_outs.go            #   outs counting quiz
    quiz_equity.go          #   equity estimation quiz
    quiz_spots.go           #   "what's your action?" spot quiz
    difficulty.go           #   level gates + adaptive weighting
  review/
    replay.go               #   HandRecord -> street-by-street replay model
    annotate.go             #   hindsight layer: revealed cards, exact equities, EV deltas
  profile/
    profile.go              #   Profile struct (lessons, moments, drill stats, grades)
    store.go                #   XDG paths, atomic JSON read/write, JSONL history
```

Engine surfaces required (designed in the engine doc, restated here as contracts):

```go
// engine.PlayerView — everything a seat may legally know. No opponent hole cards.
type PlayerView struct {
    Seat      int
    Hole      [2]Card
    Board     []Card        // 0, 3, 4, or 5 cards
    Street    Street        // Preflop, Flop, Turn, River
    Pot       int           // chips already in the middle (incl. current street)
    ToCall    int           // 0 when checking is legal
    Stacks    []int         // by seat; folded seats retain stack
    Committed []int         // this street, by seat
    InHand    []bool        // by seat
    Button    int
    Blinds    Blinds        // SB, BB in chips
    History   []ActionEvent // full visible action log for this hand
    Legal     []ActionSpec  // engine-computed legal actions with min/max amounts
}

// engine.ScriptedDeal — deterministic deal for lessons/tests.
type ScriptedDeal struct {
    Holes [][2]Card // by seat; zero value = deal randomly
    Board [5]Card   // zero cards dealt randomly
}

// engine.HandRecord / ActionEvent — append-only event log, see §6.
```

---

## 2. `internal/eval` — the hand evaluator (foundation, not negotiable)

Everything downstream needs millions of 7-card evaluations per second.

```go
type Category int // HighCard .. StraightFlush (10 values incl. RoyalFlush display alias)

// HandRank orders all 7-card hands totally. Higher is better.
// Encodes category in high bits, tiebreak kickers in low bits.
type HandRank uint32

func (r HandRank) Category() Category
func Rank7(c [7]Card) HandRank
func Rank5(c [5]Card) HandRank
func Best5(hole [2]Card, board []Card) ([5]Card, HandRank) // for UI: which 5 play
```

Implementation guidance (not code): evaluate the 21 five-card subsets against a
5-card perfect-hash table (Cactus-Kev style, ~130KB generated), or a direct 7-card
table if benchmarks demand it. A straightforward Go version of the 21-subset
approach does 10–30M evals/sec on any modern machine — that number is used in the
budget math below and is comfortably sufficient. The two-plus-two 124MB table is
explicitly rejected: binary size matters for a `go install` tool.

`Best5` exists for teaching: the review and the showdown UI must always show *which
five cards play* — beginners' single most common confusion.

---

## 3. `internal/equity` — the numeric backbone

### 3.1 Range representation

Two layers, both public, converted explicitly:

```go
// Combo is one of the 1326 unordered two-card starting hands.
type Combo uint16 // index into canonical C(52,2) ordering

// Range is a weighted set of combos. Weight 0 = not in range, 1 = always,
// fractional weights supported (e.g. "flats AQo half the time").
type Range struct {
    W [1326]float32
}

func (r Range) Combos() []WeightedCombo          // nonzero entries
func (r Range) Without(dead ...Card) Range        // card removal (hero cards, board)
func (r Range) CountCombos() float64              // weighted combo count
func (r Range) Percent() float64                  // % of all 1326
func (r Range) Contains(hole [2]Card) bool
func (r *Range) Add(spec string, weight float32) error

func ParseRange(s string) (Range, error)
func (r Range) String() string                    // canonical re-serialization
```

Grammar for `ParseRange` (comma-separated terms, whitespace ignored):

| Term      | Meaning                                    |
|-----------|--------------------------------------------|
| `JJ`      | one pocket pair (6 combos)                 |
| `JJ+`     | JJ, QQ, KK, AA                             |
| `66-99`   | pair run                                   |
| `AQs`     | suited combos only (4)                     |
| `KQo`     | offsuit combos only (12)                   |
| `AQ`      | both (16)                                  |
| `AQs+`    | AQs, AKs (walk the kicker up to the rank below the first card) |
| `T9s-54s` | suited-connector run                       |
| `A2s+`    | all suited aces A2s..AKs                   |
| `Ah Kh`   | one exact combo                            |
| `22+, A2s+, K9s+, ATo+, ...` | union; duplicate terms take max weight |
| `[50]AQo` | optional weight prefix, 0–100 percent      |

The 169-cell grid (13×13 pairs/suited/offsuit matrix) is a *view* of `Range`, used
by the range-grid visual in lessons and the AI charts; the 1326-combo weights are
the computational truth, because card removal ("he can't have AA when I hold an
ace") only works at combo granularity.

### 3.2 Core queries

```go
type Result struct {
    Win, Tie, Lose float64 // fractions, Win+Tie+Lose == 1
    Equity         float64 // Win + Tie/2 — the number the coach quotes
    Samples        int     // 0 when exact
    Exact          bool
}

type Options struct {
    MaxExactEvals int         // default 2_000_000; above this -> Monte Carlo
    MCSamples     int         // default 10_000
    Seed          int64       // deterministic sampling; 0 = derive from inputs
    Deadline      time.Duration // hard budget; degrade samples to meet it
}

func HandVsHand(hero, villain [2]Card, board []Card, opt Options) Result
func HandVsRange(hero [2]Card, villain Range, board []Card, opt Options) Result
func HandVsRanges(hero [2]Card, villains []Range, board []Card, opt Options) Result // multiway
func RangeVsRange(a, b Range, board []Card, opt Options) Result                     // lessons only
```

### 3.3 Exhaustive vs. Monte Carlo — the actual arithmetic

Assume 10M evals/sec (conservative for the §2 evaluator). "Eval" = one `Rank7`.

| Spot                          | Exact work                                   | Verdict |
|-------------------------------|----------------------------------------------|---------|
| River, hand vs hand           | 2 evals                                      | exact, ~0 |
| River, hand vs range (~200 combos) | ~400 evals                              | exact, <1ms |
| Turn, hand vs hand            | 44 boards × 2 = 88                           | exact, ~0 |
| Turn, hand vs range           | 44 × 200 × 2 ≈ 17.6k                         | exact, ~2ms |
| Flop, hand vs hand            | C(45,2)=990 × 2 ≈ 2k                         | exact, <1ms |
| Flop, hand vs range           | 990 × ~200 × 2 ≈ 400k                        | exact, ~40ms — inside budget, borderline |
| Preflop, hand vs hand         | C(48,5)=1,712,304 × 2 ≈ 3.4M                 | too slow live → precomputed table |
| Preflop, hand vs range        | table lookup per row, weighted sum           | exact, <1ms |
| Any 3+ way spot               | multiplicative blowup                        | Monte Carlo |

Decisions:

- **Preflop is a generated table, not a computation.** `preflop_table_gen.go` ships
  a 169×169 matrix of (win, tie) computed offline by `go generate` (exhaustive, run
  once, takes minutes, nobody cares). ~28,561 entries × 2 float32 ≈ 230KB of Go
  array. Preflop hand-vs-range is a weighted average over one row — instant and
  *exact*. This kills the biggest Monte Carlo consumer outright. (The table is
  suit-isomorphic; exact-combo queries with suit interactions preflop are
  irrelevant at coaching precision.)
- **Postflop heads-up is always exact.** Worst case (flop, wide range) is ~40ms;
  the coach's hot path caches the villain-range equity per (street, range) so it is
  computed once per decision, not once per candidate action.
- **Monte Carlo is the fallback, not the default**: multiway pots and any query
  whose exact cost exceeds `MaxExactEvals`. 10k samples gives ±1% at 95% confidence
  — more precision than the coach ever quotes (it says "~31%"). Sampling is seeded
  from the hand ID so a replayed hand shows identical numbers.
- **Budget conclusion:** every coach hot-path query lands well under 100ms; the
  common ones are under 5ms. No goroutine juggling needed; the TUI calls the coach
  synchronously in `Update` via a `tea.Cmd` and it returns before the next frame in
  practice. (Still run it as a `Cmd`, not inline, so a pathological spot can never
  freeze input.)

### 3.4 Outs and odds

```go
type DrawType int // FlushDraw, OESD, Gutshot, TwoOvercards, PairToTrips, PairToTwoPair, BackdoorFlush, ...

type OutsReport struct {
    Clean      []Card              // improve hero AND likely make best hand
    Tainted    []Card              // improve hero but also improve villain's likely holding
    ByDraw     map[DrawType][]Card // e.g. FlushDraw: 9 spades
    Count      int                 // len(Clean)
    Discounted float64             // Clean + 0.5×Tainted — what the coach quotes as "~N outs"
    RuleOf4    float64             // Discounted × 4 (flop), the teaching estimate
    RuleOf2    float64             // Discounted × 2 (turn)
}

// Outs counts cards that improve hero against a reference range on the next card.
// vsRange defaults to "top pair or better" when nil — the standard beginner frame.
func Outs(hero [2]Card, board []Card, vsRange *Range) OutsReport
```

Definition (mechanical, so the quiz and the coach always agree): a card `c` is an
out if `Rank7(hero+board+c)` beats the best `Rank7(villain+board+c)` for the
majority (weighted) of the reference range, and hero was not already ahead. A card
is *tainted* if it improves hero but the fraction of the range it loses to rises
(e.g. the 4♣ completes your straight and his flush). This is computed by exact
enumeration — at most 47 candidate cards × range, trivially cheap.

```go
func PotOdds(toCall, pot int) float64        // toCall / (pot + toCall) — required equity
func OddsRatio(toCall, pot int) (int, int)   // "you're getting 3:1" display form
func RequiredEquityText(toCall, pot int) string // "risk 50 to win 150 → need 25%"
```

`PotOdds` is defined as **required equity** (call / (pot after your call)), because
that is the number that gets compared to hand equity everywhere. The ratio form
exists only for display, since players at live tables say "3 to 1".

---

## 4. `internal/ai` — opponents that feel like people

### 4.1 Interfaces

NLHE has exactly one decision point (unlike euchre's bid/play/discard split), so
`Player` is small; the richness lives in `Decision`.

```go
// Player is a seat-occupying agent. It sees only its legal PlayerView.
type Player interface {
    Act(v *engine.PlayerView) engine.Action
    Name() string
}

// Strategy is the pluggable brain. It returns not just an action but the full
// scored candidate set and the typed reasoning — this is what makes the same
// object usable as an opponent (take Decision.Action) and as the coach
// (render Decision.Rationale, grade against Decision.Candidates).
type Strategy interface {
    Decide(v *engine.PlayerView) Decision
}

type Decision struct {
    Action     engine.Action   // the chosen action, amount included
    Candidates []ScoredAction  // every legal action class, scored (see grading)
    Rationale  Rationale       // typed facts, see §4.5
}

// ScoredAction scores one candidate. Score is an EV proxy in big blinds —
// not a true EV (that would require solving the game) but a consistent,
// monotone heuristic: equity × pot share minus risk, from the strategy's
// own arithmetic. Comparable only within one Decision.
type ScoredAction struct {
    Action  engine.Action
    ScoreBB float64
}
```

Candidate discretization: fold, check/call, and up to three raise sizes (small /
standard / large from the sizing table, clamped to legal min/max, plus all-in when
stack < 2× standard). Five-ish candidates, each needing at most one cached equity
query — the whole `Decide` stays under ~50ms worst case, usually ~5ms.

### 4.2 Personality: one baseline, parameterized

There is **one** rule-based strategy. Archetypes are parameter blocks over it, not
separate implementations — otherwise five bots means five bug surfaces and the
coach's "truth" fragments.

```go
type Personality struct {
    Key         string  // "tag", "nit", ...
    Label       string  // "Tight-Aggressive"
    Blurb       string  // one-line table read shown to the player
    RangeScale  float64 // 1.0 = chart as printed; 0.7 = plays 70% of chart (nit); 1.6 = LAG
    Aggression  float64 // 1.0 baseline; scales bet/raise preference over call
    BluffFreq   float64 // 0..1 multiplier on baseline bluffing frequency
    CallDown    float64 // required-equity discount when calling; station ≈ 0.6 (calls needing only 60% of the true price)
    CBetFreq    float64 // continuation-bet frequency multiplier
    Patience    float64 // resistance to tilting sizing upward; maniac ≈ 0
}
```

```go
// rulebased.New builds a Player from a personality. seed makes the bot's
// mixed decisions (bluff-or-not) reproducible per hand for tests and replay.
func rulebased.New(name string, p ai.Personality, seed int64) *rulebased.AI
```

All frequency decisions ("bluff this river 30% of the time") draw from a
`rand.PCG` seeded with `hash(handID, seat, street)` — the same hand replays
identically, and tests can pin behavior.

### 4.3 The baseline strategy (what "Coach" plays)

**Preflop — chart-driven.** `charts.go` holds data, `preflop.go` holds logic.
Charts are `equity.Range` literals per position for a 6-max game, positions
`UTG, HJ, CO, BTN, SB, BB`:

- `RFI[pos]` — raise-first-in range. Approximate targets (standard 6-max
  starting charts): UTG ~15%, HJ ~18%, CO ~26%, BTN ~42%, SB ~40% (raise-or-fold).
- `DefendCall[pos][vsPos]` and `ThreeBet[pos][vsPos]` — facing an open.
  3-bet range is value-weighted (JJ+/AK plus a few suited-ace bluffs at baseline).
- `VsThreeBet` — continue/4-bet/fold facing a 3-bet.
- Limp behind and open-limp exist **only** as personality behaviors (station,
  nit-in-SB); the baseline never open-limps, because the coach must never
  recommend it.

`RangeScale` transforms a chart by ranking its combos by preflop strength (row
order in the chart literal is the ranking — charts are written strongest-first)
and taking the first `scale × count` combos (or extending into a wider fallback
chart for scale > 1). This gives believable nit/LAG ranges without hand-tuning
five full chart sets.

**Postflop — classify, then decide.** `handclass.go`:

```go
type HandClass int
// Air, WeakPair, MiddlePair, TopPair, Overpair, TwoPair, TripsPlus  (made hands)
type DrawClass int
// NoDraw, Gutshot, OESD, FlushDraw, ComboDraw
type Classification struct {
    Made   HandClass
    Draw   DrawClass
    Report equity.OutsReport
}
func Classify(hole [2]Card, board []Card) Classification
```

`postflop.go` decision skeleton (deliberately simple, transparently teachable —
each branch maps 1:1 to a lesson):

- **Facing a bet:** compute required equity from pot odds; compute hero equity vs
  the perceived villain range (§4.4). Raise with TwoPair+ (value) or ComboDraw
  (semi-bluff, frequency-gated by `BluffFreq`); call when
  `equity ≥ required × CallDown`; else fold. Implied-odds fudge: draws to the
  nuts get a fixed +4% equity credit when stacks behind ≥ 3× pot — stated in the
  rationale as "implied odds", never silent.
- **Unraised, we were the preflop aggressor:** c-bet `CBetFreq`-gated — always
  with TopPair+, ~60% baseline with air on dry boards (board dryness = no flush
  draw possible + unpaired + max one card 9+ connected), check wet-board air.
- **Unraised, no initiative:** bet value hands TopPair+ ("beginner honesty" —
  baseline does not slowplay, because the coach shouldn't teach slowplaying
  first), semi-bluff strong draws at `BluffFreq`, otherwise check.
- **River:** no draws exist; value-bet TopPair+ sized by strength, bluff only
  busted ComboDraws at `BluffFreq × 0.5`, and **never into a station**
  (`CallDown < 0.8` kills bluff branches — this is exactly the "stop bluffing
  seat 4" lesson made executable).

**Sizing** (`sizing.go`): opens 2.5bb + 1bb per limper; 3-bets 3× the open (4× out
of position); c-bet 50% pot dry / 66% wet; value bets 66–75% pot; river value 75%.
All amounts rounded to "human" chips (nearest big blind, favoring round numbers)
— bots betting 137 into 400 reads as a computer instantly.

### 4.4 Perception: what range does a bot/coach put you on?

`perception.go` — deliberately coarse in v1, and honest about it:

```go
// PerceivedRange reconstructs a seat's range from its visible line.
// v1 rules: preflop range = the chart range for the line taken (an UTG open
// = baseline RFI[UTG], scaled by that seat's known-or-assumed RangeScale);
// each postflop bet/raise filters the range to combos with (made ≥ TopPair
// or draw ≥ OESD) on the current board; checks cap it (remove TripsPlus).
func PerceivedRange(v *engine.PlayerView, seat int, assumed ai.Personality) equity.Range
```

The coach uses the villain's *actual* personality when perceiving bots (the app
knows it; a human would learn it — and the hero is told the read via archetype
surfacing, so this isn't cheating, it's the lesson). Bots perceiving the hero
assume the baseline TAG personality.

### 4.5 Rationale: typed facts, not strings

```go
type Rationale struct {
    Facts []Fact // ordered by salience; coach renders top 2–3
}

type Fact interface{ factKind() FactKind }

type PotOddsFact   struct{ ToCall, Pot int; Required float64 }
type EquityFact    struct{ Equity float64; Method string /* "exact"|"sampled"|"table" */ }
type OutsFact      struct{ Report equity.OutsReport }
type PositionFact  struct{ Pos engine.Position; InPosition bool }
type RangeFact     struct{ Seat int; Range equity.Range; Summary string /* "≈15%, JJ+/AQ+ type hands" */ }
type ChartFact     struct{ Chart string /* "BTN open" */; InRange bool }
type ClassFact     struct{ Class rulebased.Classification }
type ArchetypeFact struct{ Seat int; Key string; Note string /* "station: don't bluff" */ }
type SizingFact    struct{ FractionOfPot float64; Purpose string /* "value"|"deny equity"|... */ }
type InitiativeFact struct{ HeroIsAggressor bool }
```

Every branch in `preflop.go`/`postflop.go` appends the facts it actually used.
This is the contract that keeps coach output truthful: the explanation can only
cite facts the decision consumed.

### 4.6 Archetypes as a teaching device

`archetypes.go` presets:

| Key       | Label              | Parameters (Range/Aggr/Bluff/CallDown/CBet) | The lesson it exists to teach |
|-----------|--------------------|---------------------------------------------|-------------------------------|
| `nit`     | The Nit            | 0.6 / 0.9 / 0.2 / 1.1 / 0.7                 | When this player raises, fold good hands. Steal their blinds. |
| `tag`     | Tight-Aggressive   | 1.0 / 1.0 / 1.0 / 1.0 / 1.0 (the baseline & the Coach) | This is what *you* are learning to play. |
| `lag`     | Loose-Aggressive   | 1.5 / 1.3 / 1.8 / 0.95 / 1.3                | Their raises mean less; call down lighter, don't get run over. |
| `station` | The Calling Station| 1.6 / 0.6 / 0.1 / 0.6 / 0.8                 | Never bluff them; value-bet thinner and bigger. |
| `maniac`  | The Maniac         | 1.9 / 1.8 / 2.5 / 0.8 / 1.6                 | Tighten up, let them hang themselves, stack them with big hands. |

Default table ("classroom mix"): hero + one of each. Table setup lets you choose
the lineup; a "mystery table" toggle hides labels for graduates.

**Surfacing** (three layers, all driven by the same data):

1. **Seat label:** bots get human names ("Marge", "Deke", …); in learning mode the
   archetype label appears under the name after **20 hands** — before that it shows
   "reading…" plus live stats, so the reveal feels earned, and the player practices
   forming the read first.
2. **Dossier panel** (`i` on a seat): live VPIP / PFR / AF computed from the
   session's hand history, the archetype blurb, and one exploit line ("stops
   bluffing when raised"). VPIP/PFR/AF are *taught in lesson 11* and then shown
   here — the HUD is the homework.
3. **Coach integration:** `ArchetypeFact` flows into recommendations — "Bet bigger
   for value — Marge (calling station) pays off with worse pairs" — so the
   archetype is not trivia, it changes the advice.

---

## 5. `internal/coach` — the live coach

### 5.1 Recommendation

```go
type Coach struct {
    strat ai.Strategy   // rulebased.New("Coach", ai.Archetypes["tag"], seed) — the same code the bots run
    prof  *profile.Profile
}

type Advice struct {
    Decision  ai.Decision // action + candidates + rationale
    Headline  string      // "Raise to 6 BB" — imperative, ≤ 40 chars
    Body      string      // 2–4 sentences from explain.go
    Numbers   []NumberChip // small labeled figures the TUI renders as chips:
                           // {"Pot odds", "25%"}, {"Your equity", "~31%"}, {"Outs", "9"}
}

func (c *Coach) Advise(v *engine.PlayerView) Advice
```

The coach panel (euchre's `renderCoachBox` pattern: fixed-height gold box, so the
layout never jumps) shows `Headline` + `Body` + chips whenever it's the hero's
turn. Advice is captured **at the moment the turn begins** and frozen — the
recommendation the hero saw is the recommendation grading uses, even if a
recompute would differ (it won't, seeds are deterministic, but the invariant makes
the audit trail airtight).

Coach visibility modes (profile setting): `full` (advice before acting),
`grade-only` (numbers chips shown, recommendation hidden until after acting — the
training-wheels-off mode), `off`. Post-hand review works in all modes.

### 5.2 Grading — the heart of the app

```go
type Grade int
const (
    GradeBest      Grade = iota // matched coach action (sizing within 25%)
    GradeGood                   // different but ScoreBB within 0.25bb of best
    GradeInaccuracy             // EV loss 0.25–1bb
    GradeMistake                // EV loss 1–3bb
    GradeBlunder                // EV loss ≥ 3bb
)

type GradedDecision struct {
    HandID     string
    Street     engine.Street
    Taken      engine.Action
    Best       ai.ScoredAction
    EVLossBB   float64      // Best.ScoreBB − Score(Taken); ≥ 0
    Grade      Grade
    Body       string       // explanation, includes the counterfactual
    ViewDigest ViewDigest   // compact snapshot of what hero knew (for review)
}

func (c *Coach) GradeAction(adv Advice, taken engine.Action) GradedDecision
```

Mechanics: `Advise` already scored every candidate (`Decision.Candidates`). The
hero's action is matched to its candidate class (a raise is matched to the nearest
scored size; a wildly nonstandard size is scored as its own candidate on demand).
Grade = EV-loss band. This gives the graded scale the brief asks for and makes
"two acceptable actions" natural: when calling and raising score within 0.25bb,
either earns `GradeGood`+ — the coach explicitly says *"Raise was my pick, but
calling is fine here too."* Euchre's binary matched/didn't is exactly what we're
replacing; poker has too many close spots for it.

**Outcome-independence, designed in three places:**

1. `GradeAction` is called by the game screen synchronously when the hero commits
   an action — before the engine deals another card. The `GradedDecision` is
   appended to the hand's annotation log then and never mutated.
2. The grade feedback line in the coach box after acting shows grade only —
   *never* "and you won/lost". During the hand, results and grades are kept in
   separate UI regions.
3. The review screen renders a **Decision vs. Outcome** line per graded decision:
   "✔ Good call (needed 25%, had 31%) — you missed and lost the pot. Right play,
   bad result. Over time this call prints money." and the mirror case: "✘ Mistake
   — calling needed 33%, you had 18%. You hit your gutshot and won 40bb *this
   time*; the play still loses money." `explain.go` has dedicated templates for
   all four (good/bad × won/lost) quadrants, and the session summary's headline
   stat is **decision accuracy, not chips won** — the profile tracks both so the
   stats screen can plot them diverging, which *is* the variance lesson.

What "wrong" is **not**: the grader never uses opponents' actual hole cards, never
uses future streets, and `review/annotate.go` (which does know the hole cards) is
a separate layer that renders *hindsight* info visually distinct (dimmed,
"hindsight" tag) from the frozen grades.

### 5.3 Explanation generator (`explain.go`)

```go
// Explain renders a Rationale into 2–4 sentences. Deterministic template
// selection keyed by (street, decision context, top fact kinds); small
// phrasing pools rotated by hand seed so text doesn't feel canned.
func Explain(d ai.Decision, v *engine.PlayerView) (headline, body string, chips []NumberChip)
```

Template rules (the euchre `tipPlay` discipline, upgraded):

- Every template is bound to the fact types it cites; `Explain` panics in tests if
  a template references a fact the rationale doesn't contain (truthfulness gate —
  the euchre discard-tip bug where the stated reason didn't match the real reason
  is the failure mode this prevents).
- Numbers are always paired with their derivation: "you need **25%** (call 50 into
  150) and have **~31%** — 8 clean outs, rule of 4 says 8×4 ≈ 32%."
- One idea per sentence; body ≤ 4 sentences; hardest number goes in a chip, not
  prose.
- Archetype notes append as the final sentence when an `ArchetypeFact` is present.

### 5.4 Teachable moments (`moments.go`)

Euchre's `teachableConcept` pattern: a prioritized registry, fire-once semantics,
persisted to the profile (euchre only persisted per game; we persist forever —
"first time you saw a flush draw" is a once-per-player event).

```go
type Moment struct {
    ID       string
    Title    string
    Body     func(v *engine.PlayerView, adv Advice) string // may cite live cards/numbers
    Trigger  func(v *engine.PlayerView, adv Advice) bool
    Priority int
}

func (c *Coach) PendingMoment(v *engine.PlayerView, adv Advice) *Moment
```

Initial registry (IDs are the persistence keys):

`first_flush_draw`, `first_oesd`, `first_gutshot` (and why it's half an OESD),
`first_button_raise_hand` (on the button with a chart-raising hand),
`first_great_price` (getting 4:1 or better), `first_cbet_spot`,
`first_facing_3bet`, `first_kicker_showdown` (won/lost on kicker),
`first_dominated` (fires in *review* when hero's AJ ran into AK),
`first_station_bluff_warning` (hero bets air into `CallDown < 0.8` seat),
`first_pot_committed` (calling leaves < 1 pot behind), `first_counterfeit`
(two-pair counterfeited on a paired board), `first_split_pot`,
`first_semibluff` (coach recommends betting a draw — names the concept).

Rendered as euchre-style modal popups (`renderPopup`), max one per decision,
dismiss with Enter, never during opponents' turns.

---

## 6. Hand history: the engine's action log (contract for review & stats)

The engine appends events to a `HandRecord` as they happen; nothing downstream
mutates it.

```go
type HandRecord struct {
    ID      string        // ULID
    Time    time.Time
    Blinds  engine.Blinds
    Seats   []SeatInfo    // name, personality key ("" = human), starting stack, position
    Events  []Event
}

type Event struct {
    Kind   EventKind      // EvDeal, EvPostBlind, EvAction, EvBoard, EvShowdown, EvPotAward, EvRebuy
    Seat   int            // -1 when not seat-scoped
    Action *engine.Action // EvAction
    Cards  []Card         // EvDeal (hole, per seat), EvBoard (street cards), EvShowdown (revealed)
    Amount int            // EvPostBlind, EvPotAward (per pot, side pots = multiple events)
    Pot    int            // pot size after this event — denormalized for cheap rendering
}
```

Coach annotations ride alongside, keyed by event index:

```go
type HandAnnotations struct {
    HandID  string
    Grades  map[int]coach.GradedDecision // event index of the hero action -> frozen grade
    Advice  map[int]coach.Advice         // what the coach panel showed at that decision
}
```

Persistence: one JSONL line per `{record, annotations}` pair appended to
`hands/YYYY-MM-DD.jsonl` (see §9) at hand end. JSONL because sessions crash and
append-only never loses finished hands; one file per day keeps files small enough
to scan without an index.

---

## 7. `internal/review` — post-hand review

```go
// Replay builds a street-by-street model: for each street, the board, pot,
// each seat's (now-revealed) hole cards, and the ordered actions with the
// hero's frozen grades attached.
func Replay(rec engine.HandRecord, ann HandAnnotations) *ReplayModel

type ReplayModel struct {
    Streets   []StreetFrame
    Decisions []DecisionFrame // hero decisions, in order — the review's spine
    Summary   Summary
}

type DecisionFrame struct {
    EventIdx  int
    Frozen    coach.GradedDecision // grade as computed at the time — immutable
    Hindsight Hindsight            // computed now, WITH revealed cards
}

type Hindsight struct {
    ActualVillainHands map[int][2]Card
    TrueEquity         float64 // hero vs actual hands at this decision, exact
    PerceivedEquity    float64 // what the coach estimated vs range at the time
    Note               string  // "your range read was fair: he was at the top of it"
}

type Summary struct {
    NetChips     int
    GradeCounts  map[coach.Grade]int
    EVLossBB     float64  // sum of decision EV losses — "you leaked 2.5bb this hand"
    KeyDecision  int      // index of the largest-EV-loss (or best) decision
}
```

Review screen flow: opens automatically after any hand where the hero saw a flop
(skippable; always available from the pause menu for the last 50 hands). Renders
the table street by street with **all hole cards face up and dimmed "hindsight"
badges**, stepping with ←/→ between hero decisions. Each decision shows three
lines:

```
 YOU CALLED 50          Coach: call            Grade: ✔ Good
 Then: needed 25%, had ~31% vs his likely range (9 outs)
 Now:  he held K♠Q♠ — your true equity was 34%. Good read, good call.
```

The **EV gained/lost** ledger is decision-EV-loss based (frozen `EVLossBB`), not
result based — "where EV was lost" per the brief means "which decisions leaked",
and the summary bar chart plots EV loss per decision, with the outcome shown
separately and deliberately unaligned with it.

---

## 8. `internal/tutorial` — guided lessons

### 8.1 Types (euchre's registry pattern, extended)

```go
type Lesson struct {
    ID            string
    Title         string
    Goal          string   // one line, shown on the curriculum screen
    Order         int
    Prerequisites []string
    Sections      []Section
}

type Section struct {
    Kind    SectionKind // SectionText, SectionVisual, SectionDrill, SectionScripted
    Title   string
    Text    string          // markdown-ish, card glyphs colorized
    Visual  *Visual         // SectionVisual
    Drill   *Drill          // SectionDrill
    Script  *ScriptedHand   // SectionScripted
}
```

`Visual` reuses euchre's `VisualElement` philosophy with poker types:
`VisualBoard` (community cards + hole cards with best-5 highlighted),
`VisualRangeGrid` (13×13 grid with an `equity.Range` shaded — *the* preflop
teaching device), `VisualTableSeats` (6-max positions labeled), `VisualHandLadder`
(the 10 hand categories ranked), `VisualPotOdds` (pot/call/percentage diagram).

### 8.2 Scripted hands

```go
type ScriptedHand struct {
    Deal      engine.ScriptedDeal
    Seats     []ScriptSeat        // hero seat + scripted opponents
    Stops     []Stop              // where the lesson pauses and teaches
    Intro     string
    Debrief   string              // shown at hand end
}

type ScriptSeat struct {
    Name    string
    Actions []engine.Action // consumed in order; must exactly match the engine's turn sequence — a mismatch is a test failure, not a runtime fallback
}

type Stop struct {
    AtDecision int      // Nth hero decision in the hand (0-based)
    Teach      string   // text shown in the coach box instead of normal advice
    Expect     *engine.Action // if set, only this action advances (drill-style); else any legal action, then graded normally
}
```

`ScriptSeat.Actions` implements `ai.Player` trivially (a `ScriptedPlayer`), so the
lesson runs in the *real* game screen with the real engine — same rendering, same
coach box, zero parallel simulation code. Scripted decks force the teaching spot:
lesson 7's scripted hand deals the hero 9♠8♠ on A♠K♠2♥ *every time*, the scripted
villain bets half pot, and the stop teaches counting the 9 flush outs before the
hero acts. Every scripted hand has a test asserting the script is consistent with
engine legality (this is cheap and catches every future engine change).

### 8.3 Drills

```go
type Drill struct {
    Prompt  string
    Visual  *Visual        // cards/board the question is about
    Answer  Answer
    Explain string         // shown after answering, right or wrong
}

type Answer interface{ Check(input string) bool }
type ChoiceAnswer  struct { Choices []string; Correct int }
type NumericAnswer struct { Value float64; Tolerance float64 } // outs: ±0; equity: ±5
type OrderAnswer   struct { Items []string; Correct []int }    // rank these hands
```

### 8.4 The curriculum (`content/`, one file each)

| # | ID | Title | Goal (one line) |
|---|----|-------|-----------------|
| 1 | `hand-rankings` | Hand Rankings | Know the ten hand categories cold, and that the best **five** of seven play. |
| 2 | `hand-flow` | How a Hand Works | Blinds, four streets, showdown — who acts when and why the button moves. |
| 3 | `kickers-ties` | Kickers & Ties | Read a board and say who wins — kickers, counterfeits, and split pots. |
| 4 | `position` | Position Is Power | Acting last is worth money; name the six seats and rank them. |
| 5 | `preflop-ranges` | Starting Hands by Seat | Play a tight, position-widening range — read an RFI chart, fold the rest without regret. |
| 6 | `facing-raises` | When Someone Raises First | 3-bet / call / fold logic, and why dominated hands (KJ vs a raiser) bleed money. |
| 7 | `pot-odds` | The Price of a Call | Compute required equity from pot and bet size in your head. |
| 8 | `outs-equity` | Outs & the Rule of 2 and 4 | Count clean outs and convert to equity fast. |
| 9 | `playing-draws` | Playing Draws | Combine 7+8: call, fold, or semi-bluff a draw based on price and fold equity. |
| 10 | `why-we-bet` | Why We Bet | Every bet is value or bluff; c-betting; checking is fine. |
| 11 | `bet-sizing` | Bet Sizing Speaks | Size as a fraction of the pot; what small/large sizes mean and offer. |
| 12 | `reading-players` | Reading Opponents | VPIP/PFR/AF, the five archetypes, and one exploit for each. |
| 13 | `bluffing` | Bluffing That Makes Money | Fold equity, good bluff candidates (blockers-lite), and who never folds. |

Each lesson = 3–6 sections, at least one drill; lessons 5–13 each end with one
scripted hand. Prerequisites form a chain with one branch (4→5→6 preflop track and
7→8→9 math track can interleave; 10+ require both).

### 8.5 Progress

Completion = viewed all sections + passed the drills (drills retry freely; passing
= correct, whether first try or fifth — this is a curriculum, not an exam). Stored
in the profile (§9). The Learning Journey screen (euchre's welcome/lessons/complete
flow) shows per-lesson checkmarks and resumes at the first incomplete lesson.

---

## 9. `internal/trainer` — standalone drill mode

```go
type QuizKind int // QuizRankings, QuizOuts, QuizEquity, QuizSpots

type Session struct {
    Kind     QuizKind
    Level    int
    Timed    bool
    Items    []Item
}

type Item struct {
    Drill    tutorial.Drill  // reuses the drill type — one renderer everywhere
    SkillTag string          // "outs.flushdraw", "rank.twopair-vs-trips", ...
}

func NewSession(kind QuizKind, prof *profile.Profile) *Session // level + weighting from profile
```

**Generation, not authoring:** trainer items are generated from random deals and
verified against `eval`/`equity` — the correct answer is computed, never hand
written, so the pool is infinite and always right.

- **Rankings speed quiz:** deal two 7-card boards-plus-holes, "left or right wins,
  or split?" — timed, streak-scored. Level gates: L1 obvious categories, L2 same
  category kicker fights, L3 counterfeits/split traps (generated by rejection
  sampling until the trap predicate holds).
- **Outs quiz:** hero hand + flop/turn vs stated villain holding; enter the number
  (`NumericAnswer`, exact). Levels: L1 pure flush/OESD, L2 combo draws, L3 tainted
  outs ("count clean outs only").
- **Equity estimation:** hero vs villain (shown) or vs a named range; slider guess,
  ±5% scores full, ±10% half. Teaches calibration; answer screen shows the rule-of-
  4 arithmetic next to the exact number.
- **Spot quiz ("what's your action?"):** a generated table state rendered exactly
  like the game screen; hero picks fold/call/raise; graded by `coach.GradeAction`
  — `GradeBest`/`GradeGood` both count as correct (close spots must not be marked
  wrong on coin flips). Levels: L1 preflop chart spots, L2 flop pot-odds spots,
  L3 turn/river with archetype reads shown.

**Difficulty progression:** per-`SkillTag` accuracy tracked as an exponential
moving average (α=0.3) in the profile. A level unlocks at ≥80% EMA over ≥20 items
at the previous level. Within a session, item selection weights tags by
`(1 − EMA)²` — practice bends toward weakness without becoming a punishment loop.

---

## 10. `internal/profile` — persistence

Location (XDG, no new deps):

```
$XDG_DATA_HOME/holdem/            # fallback ~/.local/share/holdem/
  profile.json                    # everything below except hand histories
  hands/2026-07-29.jsonl          # HandRecord+HandAnnotations, append-only
```

`os.UserConfigDir` is wrong for this (it's data, not config); resolve
`$XDG_DATA_HOME` manually with the `~/.local/share` fallback (and `%AppData%` on
Windows via one small switch).

```go
type Profile struct {
    Version        int                       // 1; migrations switch on this
    CreatedAt      time.Time
    Bankroll       int                       // session-persistent chips
    LessonsDone    map[string]time.Time      // lesson ID -> completion
    MomentsSeen    map[string]time.Time      // moment ID -> first shown
    DrillStats     map[string]SkillStat      // SkillTag -> EMA, attempts, level
    GradeTotals    map[string]int            // coach.Grade name -> count, lifetime
    SessionLog     []SessionSummary          // per session: hands, net bb, accuracy %, EV loss bb
    CoachMode      string                    // "full" | "grade-only" | "off"
    TableDefaults  TableConfig               // last lineup, blinds, stack
}

func Load() (*Profile, error)   // missing file -> fresh default, never an error prompt
func (p *Profile) Save() error  // write temp file + rename (atomic); called on state change, cheap
```

JSON, human-readable, versioned. Hand histories are separate (JSONL, §6) because
they grow unboundedly and the profile must stay a fast small read at startup.
Review's "last 50 hands" reads the newest JSONL files backwards; no index needed
at this scale.

---

## 11. Testing strategy (the euchre discipline, applied)

- **Chart tests:** every preflop chart asserts combo counts within a target band
  (BTN RFI between 38–46%) and spot-checks (AA always, 72o never, A5s BTN yes /
  UTG no).
- **Threshold tests:** postflop branches pinned like euchre's bidding tests —
  "flush draw facing half pot with 4:1 implied never folds", "station never
  bluffs river", "baseline never open-limps".
- **Grading tests** (the `coach_grade_test.go` analog, but semantic): folding a
  royal flush on the river is `GradeBlunder`; calling with exact pot odds is
  ≥ `GradeGood`; taking the non-recommended of two near-equal lines is
  `GradeGood`, not a mistake. And the anti-resulting test: grade of a losing
  correct call equals grade of a winning correct call, byte for byte.
- **Truthfulness test:** for N random spots, every number in `Explain` output is
  re-derivable from the rationale facts (templates can't invent figures).
- **Determinism test:** same `HandRecord` seed → identical bot actions, advice
  text, and equities on replay.
- **Equity oracle tests:** exact enumeration vs known values (AA vs KK preflop =
  81.9%, flush draw on flop vs top pair ≈ 34–38% band), and MC vs exact within
  1.5% on 100 random spots.
- **Script legality test:** every `ScriptedHand` in `content/` replays through the
  real engine without an illegal action.

---

## 12. Explicit non-goals (v1)

- No GTO solver, no CFR, no mixed-strategy equilibria. The baseline is exploitable
  by a strong player — that is fine; it teaches *fundamentals*, and its rules map
  1:1 to lessons. Revisit only if Brandon outgrows it.
- No hand-reading beyond §4.4's coarse filters. The coach says "his likely range"
  and shows it; refining perception street-by-street is a v2 with real payoff, but
  v1 honesty beats v1 sophistication.
- No blocker/combinatorics coaching beyond the "blockers-lite" sentence in lesson
  13.
- No multi-table, no tournaments (per the brief: cash only).
