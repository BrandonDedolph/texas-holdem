# Engine Design — `internal/engine/` and `internal/eval/`

Design for the game-engine layer of the terminal Texas Hold'em learning tool.
Scope: 6-max No-Limit Hold'em cash game. This document is the implementation
contract for two packages:

- `internal/engine/` — cards, deck, betting state machine, pots, hand and table
  lifecycle. Mirrors the euchre engine's *action-driven* pattern
  (`../../euchre/internal/engine/`): a mutable state object, an `Action`
  interface, `Apply(action) error`, and `LegalActions()` as the single source
  of truth for what is legal.
- `internal/eval/` — 7-card hand evaluation and human-readable hand
  description. Pure functions, zero allocation, no dependency on `engine`
  beyond the `Card` type.

Terminology note up front, because Hold'em overloads "hand": in this codebase
a **`Hand`** is *one deal* (the analogue of euchre's `Round`), a player's two
cards are **`HoleCards`**, and the evaluator's output is a **`HandRank`**.
Euchre's `Hand` (cards held) does not carry over; hole cards are a fixed
`[2]Card`, so no container type is needed.

---

## 1. Card & deck primitives

### 1.1 Card encoding: a single byte

```go
// Card is a compact card encoding: Card = Rank<<2 | Suit, values 0..51.
type Card uint8

type Rank uint8 // Two=0, Three=1, ... Ten=8, Jack=9, Queen=10, King=11, Ace=12
type Suit uint8 // Clubs=0, Diamonds=1, Hearts=2, Spades=3

func MakeCard(r Rank, s Suit) Card  // r<<2 | s
func (c Card) Rank() Rank           // c >> 2
func (c Card) Suit() Suit           // c & 3
```

**Decision: `uint8` integer encoding, rank-major.** Euchre used
`struct { Suit; Rank }`, which is perfectly readable and was the right call
for a 24-card trick game that evaluates a handful of comparisons per trick.
Hold'em is different: the equity trainer and post-hand EV review will run the
evaluator across millions of card combinations (§2.4 has the arithmetic), and
the encoding choice ripples into everything downstream:

- A `Card` is one byte; `[7]Card` fits in a single cache line and passes by
  value with no pointer chasing.
- A **set of cards is a `uint64` bitmask** (bit *n* set ⇔ card *n* present).
  Dead-card removal, duplicate detection, and "enumerate remaining deck" in
  the equity code become single AND/OR instructions instead of slice scans.
- Rank-major (`rank<<2 | suit`) means `card >> 2` is the rank — the operation
  the evaluator does constantly — and sorting cards numerically sorts by rank.

Readability is recovered at the API surface, not in the representation:

```go
func (c Card) String() string          // "A♠", "T♦" — for the TUI
func (c Card) Code() string            // "As", "Td" — canonical 2-char, for logs/tests
func ParseCard(s string) (Card, error) // accepts "As", "as", "A♠", "10h"
func MustCard(s string) Card           // panics on error; for tests and lesson content
func ParseCards(s string) ([]Card, error) // "As Kd 7h" or "AsKd7h"
```

Lesson content and tests will be written in `"As Kd"` notation and never touch
raw byte values. **Rejected alternative:** the euchre-style struct. It costs
2 bytes + no bitset story + slower map keys, and buys nothing once
`String()`/`ParseCard` exist. Also rejected: Cactus-Kev's prime-encoded 32-bit
card. Its prime trick only pays off for one specific evaluator design (§2) we
are not using, and it makes every card 4× larger.

```go
// CardSet is a bitmask of cards; bit c is set iff Card(c) is in the set.
type CardSet uint64

func (s CardSet) Has(c Card) bool
func (s CardSet) Add(c Card) CardSet
func (s CardSet) Remove(c Card) CardSet
func (s CardSet) Count() int          // bits.OnesCount64
func NewCardSet(cards ...Card) CardSet
```

### 1.2 Deck and deterministic dealing

The deck's only job is to be a *card source*. The engine deals from an
interface so that real play, seeded replays, and scripted lessons are the same
code path:

```go
// CardSource supplies cards in deal order. Implementations must panic only
// on programmer error (drawing more than 52).
type CardSource interface {
    Draw() Card
}

// Deck is a shuffled 52-card source.
func NewDeck(seed uint64) *Deck // shuffles with math/rand/v2 PCG seeded from seed
func NewDeckRandom() *Deck      // crypto-seeded, for normal play
```

**Determinism decision:** `math/rand/v2`'s PCG generator, seeded explicitly.
PCG in rand/v2 is a *specified* algorithm — the same seed produces the same
shuffle on every platform and Go release, which is what "lesson 12 always
deals you 9♣9♦ facing a raise" requires. The global rand source (which euchre
falls back to) is deliberately not used anywhere in the engine: every hand
records the seed it was dealt from, so any hand can be replayed exactly.

For lessons and tests that need *specific* cards rather than a reproducible
random shuffle:

```go
// ScriptedDeck deals a fixed prefix of cards, then falls back to a seeded
// shuffle of the remaining 48-odd cards. Lessons usually script only the
// hero's cards and the flop; the fallback fills in the rest without the
// lesson author enumerating 52 cards.
func NewScriptedDeck(prefix []Card, fallbackSeed uint64) *ScriptedDeck
```

**Deal order is part of the engine contract** (otherwise scripting is
guesswork): hole cards are dealt **two at a time per seat**, starting with the
first live seat left of the button, proceeding clockwise, button last; then
flop (3 cards), turn, river. **No burn cards** — burns exist to defeat marked
cards and second-dealing, which don't exist digitally, and they'd force lesson
scripts to insert junk cards. Rejected alternative: authentic one-card-at-a-time
round-robin dealing; it changes nothing observable and makes scripted prefixes
brain-hurting to write.

### 1.3 Chips

```go
// Chips is a chip count. Whole chips only — blinds are small integers (1/2),
// so there is no fractional-cent bookkeeping anywhere.
type Chips int64
```

Signed, so that per-hand net results (`won - invested`) are representable
directly. All engine invariants are stated in `Chips`; the invariant
`sum(stacks) + sum(pot contributions) == constant` holds after every applied
action and is asserted in tests (§6).

---

## 2. Hand evaluation — `internal/eval/`

### 2.1 The options, honestly weighed

| Approach | Speed (per 7-card eval, Go, rough) | Memory | Implementation cost |
|---|---|---|---|
| Naive: 21 × 5-card combos, keep best | ~1–3 µs | none | low, but still needs a correct 5-card evaluator |
| Two Plus Two 7-card LUT | ~5–10 ns | ~124 MB table | must generate or ship the table; table generator is its own project |
| Bitmask/histogram direct 7-card | ~50–150 ns | ~8 KB | moderate; each category is explicit code |
| Cactus Kev perfect-hash 5-card (+21 combos for 7) | ~400 ns–1 µs | ~130 KB | magic constants, opaque |

**What speed do we actually need?** The most demanding consumer is the equity
trainer / EV review doing exact enumeration:

- Hand-vs-hand preflop, exact: C(48,5) = 1,712,304 boards × 2 evals ≈ **3.4M
  evals** → ~0.3 s at 100 ns/eval. Done once per drill question; fine.
- Hand-vs-range on the flop: ~100 combos × C(45,2)=990 runouts × 2 ≈ **200K
  evals** → ~20 ms. Interactive-instant.
- Monte-Carlo (multiway, or when exact is too big): 100K samples is ~10 ms.

So the honest requirement is "~10M evals/sec", not "~200M evals/sec".

**Decision: direct bitmask + rank-histogram 7-card evaluator.**
The Two Plus Two LUT is the classic answer when you need hundreds of millions
of evals/sec (real solvers). We don't, and its costs are real: a 124 MB table
either shipped in the repo (absurd for a learning-tool binary) or generated on
first run (minutes, plus a second evaluator to generate it *from*). The naive
21-combo approach is rejected because it's 20× slower *and* you still have to
write the hard part (a correct 5-card ranker) — it saves nothing. The direct
evaluator is also the most *teachable* implementation, which matters in a
project whose entire point is learning: the code literally reads "count ranks;
if a suit has 5+ cards check straight-flush; else quads → boat → flush → …".

Sketch of the algorithm (design, not implementation):

1. Build `suitCount[4]` and per-suit 13-bit rank masks, plus a rank histogram
   `rankCount[13]` and an overall 13-bit `rankMask` — one pass over 7 cards.
2. If any suit has ≥ 5 cards (necessarily a unique suit), the hand is a
   flush or straight flush and the flush path is *self-contained*: check the
   suit's rank mask for a straight (straight flush), else take its top five
   ranks (flush). This shortcut is sound because 5+ suited cards leave at
   most 2 off-suit cards, which makes quads (needs three off-suit cards of
   one rank) and a full house (the suited cards are all distinct ranks, so
   no trips+pair can be assembled) impossible — nothing that outranks a
   flush can coexist with one in 7 cards except the same-suit straight
   flush. A unit test asserts this exhaustively.
3. Otherwise fall through the remaining categories in rank order: quads →
   full house → straight → trips → two pair → pair → high card, computed
   from the rank histogram and rank mask, returning at the first hit.
4. Straight detection on a 13-bit mask via an 8,192-entry `[uint8]` table
   (index = rank mask, value = top rank of best straight or 0xFF), built in
   `init()` in microseconds; handles the wheel (A-5). 8 KB total.

Zero allocations; inputs by value; results are a `uint32`.

### 2.2 `HandRank`: comparable *and* self-describing

```go
package eval

type Category uint8

const (
    HighCard Category = iota
    OnePair
    TwoPair
    ThreeOfAKind
    Straight
    Flush
    FullHouse
    FourOfAKind
    StraightFlush // Ace-high straight flush describes itself as "Royal Flush"
)

// HandRank is a totally-ordered hand strength. Higher is better. Layout:
//
//   bits 23..20  Category
//   bits 19..0   five 4-bit rank fields k1..k5, category-specific:
//     TwoPair:   k1=high pair, k2=low pair, k3=kicker,        k4=k5=0
//     FullHouse: k1=trips rank, k2=pair rank,                 k3..k5=0
//     Straight:  k1=top card of straight (5 for the wheel),   k2..k5=0
//     Flush/HighCard: k1..k5 = the five ranks, descending
//     ... etc.
//
// Because the layout is category-then-lexicographic-kickers, integer
// comparison IS hand comparison, and the fields decode back into English.
type HandRank uint32

func (r HandRank) Category() Category
func (r HandRank) Less(o HandRank) bool // just r < o; method exists for readability

// String is the short form the table UI shows:
//   "Flush, King high" / "Two Pair, Aces and Nines" / "Royal Flush"
func (r HandRank) String() string

// Describe is the full form the coach and post-hand review use:
//   "Two Pair, Aces and Nines with a Queen kicker"
//   "Straight, Nine to King"
//   "Full House, Kings full of Fours"
func (r HandRank) Describe() string
```

The self-description requirement is why `HandRank` packs *semantic fields*
rather than being an opaque dense index (the 2+2 LUT yields 0..7461 indices
that need a reverse table to describe). Packing category + kickers costs
nothing in comparability and makes `Describe()` a pure decode.

### 2.3 Evaluator API

```go
// Eval7 ranks the best 5-card hand within exactly 7 cards.
func Eval7(cards [7]Card) HandRank

// Eval5 ranks exactly 5 cards. Used by lessons ("rank these five") and as the
// cross-check oracle in tests.
func Eval5(cards [5]Card) HandRank

// EvalHoldem ranks hole cards against a 3-, 4-, or 5-card board.
// (5- and 6-card evaluation reuses the same core; the coach calls this on
// every street.)
func EvalHoldem(hole [2]Card, board []Card) HandRank

// Best5 additionally reports WHICH five cards form the best hand, so the TUI
// and review screen can highlight them. Slower path (falls back to 21-combo
// using Eval5); called once per showdown, not in equity loops.
func Best5(hole [2]Card, board []Card) (HandRank, [5]Card)
```

`Card` here is `engine.Card` — `eval` imports `engine` for the card type only.
(Alternative rejected: a shared `internal/cards` package. Two packages for
this project is enough; `engine` has no reason to import `eval`, so the edge
is acyclic: `eval → engine`.) The future equity package
(`internal/equity`, out of scope here) will consume exactly `Eval7` +
`CardSet` and nothing else from these packages.

### 2.4 Correctness strategy

The evaluator is the one component where a subtle bug silently teaches wrong
poker. Tests (in `internal/eval/`):

- **Exhaustive 5-card:** enumerate all C(52,5) = 2,598,960 hands, bucket by
  category, assert the known census (1,302,540 high cards … 40 straight
  flushes, 4 royals). Runs in seconds.
- **Oracle cross-check for 7-card:** a deliberately dumb, obviously-correct
  reference (21 combos × `Eval5`) compared against `Eval7` over 1M random
  7-card draws plus a curated adversarial set (wheel straights,
  flush-vs-boat boards, A-2-3-4-5-6-7 in one suit, quads on board, etc.).
- **Description golden tests:** table of `("AsKsQsJsTs", "Royal Flush")` pairs.

---

## 3. Betting engine

This is the correctness-critical core. It lives in `betting.go` + `pot.go`,
orchestrated by `hand.go`.

### 3.1 Street state machine

```go
type Street uint8
const (
    Preflop Street = iota
    Flop
    Turn
    River
)

type Phase uint8
const (
    PhaseBetting  Phase = iota // a betting round on h.street is open
    PhaseShowdown              // river betting closed with 2+ live players
    PhaseComplete              // pots awarded; result available
)
```

Transitions (all driven from inside `Hand.Apply`; the caller never advances
streets manually):

```
 start hand
     │  post SB, post BB (automatic, logged), deal hole cards
     ▼
 Betting(Preflop) ──betting closes──► deal flop ──► Betting(Flop)
     │                                                  │
     │                              betting closes ──► deal turn ─► Betting(Turn)
     │                                                                 │
     │                                             betting closes ─► deal river ─► Betting(River)
     │                                                                                │
     │                                                                betting closes─► Showdown ─► Complete
     │
     ├── at ANY street: fold leaves exactly 1 live player
     │        └──────────────► Complete  (uncontested; pot awarded, NO further
     │                                    cards dealt, winner needn't show)
     │
     └── at ANY street: betting closes AND ≤1 live player still has chips
              └── "all-in run-out": deal all remaining board cards with no
                  betting rounds ─────► Showdown ─► Complete
```

Two skip paths, spelled out:

1. **Everyone folds.** The hand ends *immediately* — remaining streets are
   never dealt (dealing them would leak information the real game doesn't
   reveal; the post-hand review can deal "what would have come" from the
   recorded deck separately, which is a review feature, not engine behavior).
2. **All-in run-out.** When a betting round closes and fewer than two live
   players can still act (everyone live is all-in, or exactly one player has
   chips behind and every bet is matched), the engine deals the remaining
   board cards in one step, emitting board events per street for the TUI to
   animate, and proceeds to showdown. No run-it-twice — a learning tool wants
   one outcome per decision.

### 3.2 Betting-round state and legality rules

Per-street betting state (internal, but its fields define the rules):

```go
type bettingState struct {
    CurrentBet      Chips            // highest total committed this street
    LastFullRaiseSz Chips            // increment of the last FULL bet/raise
    LastFullRaiseTo Chips            // bet level established by that raise
    Committed       [MaxSeats]Chips  // per-seat chips in this street
    ActedAtBet      [MaxSeats]Chips  // CurrentBet when seat last acted; -1 = hasn't acted
    ToAct           SeatSet          // seats that still owe a decision
}
```

**The rules, precisely:**

- **Blind posting.** SB and BB are posted automatically when the hand starts —
  they are compulsory in a cash game, so they are *not* player `Action`s and
  never appear in `LegalActions`; they are recorded as `EvPostBlind` events
  for replay. A short stack posts all-in for less. Preflop initializes
  `CurrentBet = BB`, `LastFullRaiseSz = BB`, `LastFullRaiseTo = BB`, and
  `ActedAtBet = -1` for everyone — posting a blind is not acting, which is
  exactly what gives the BB its option (§ closing rules).
- **Heads-up blinds.** With 2 players the **button posts the SB** and acts
  first preflop; the other seat posts the BB and acts first on every postflop
  street. With 3+ players: SB is left of the button, BB next; UTG (left of
  BB) opens preflop; first live seat left of the button opens postflop.
- **Min bet** (opening a postflop street): 1 BB, or all-in for less.
- **Min raise:** raise-to must be ≥ `CurrentBet + LastFullRaiseSz`, or all-in
  for less. A raise that meets this updates
  `LastFullRaiseSz = raiseTo − CurrentBet`, `LastFullRaiseTo = raiseTo`.
- **Incomplete raise (all-in) does not reopen action.** If a player goes
  all-in for more than `CurrentBet` but less than a full raise:
  `CurrentBet` rises to the all-in amount, but `LastFullRaiseSz` and
  `LastFullRaiseTo` are **unchanged**. Every live seat facing the new price
  re-enters `ToAct`, but a seat may **raise** only if
  `ActedAtBet[seat] < LastFullRaiseTo` — i.e. it has not yet acted, or a full
  raise has occurred since it last acted. Otherwise its legal actions are
  fold/call only. This single inequality encodes the whole
  reopening rule; it is worth a table of unit tests on its own.
- **No cap.** NL has no bet ceiling and no raise-count cap; max is always the
  acting player's stack (table stakes). There is deliberately no
  "max raises per street" field anywhere — its absence is the rule.
- **Raise semantics are raise-TO,** not raise-by (`Raise{To: 300}` means the
  total street commitment becomes 300). This matches how players and the TDA
  express raises and avoids the classic ambiguity bug.

**Action-closing conditions.** A betting round ends when `ToAct` is empty.
`ToAct` bookkeeping:

- Round start: every live, non-all-in seat is in `ToAct` (preflop this
  *includes* SB and BB — posting was not acting).
- A seat that folds, checks, calls, bets, or raises is removed.
- A full bet/raise puts every *other* live non-all-in seat (back) into `ToAct`.
- An incomplete all-in raise puts every live non-all-in seat facing the new
  price back into `ToAct` (with raising rights per the inequality above).

This formulation gets the **BB option** for free: when action limps around,
the BB is still in `ToAct` (never acted) and may check or raise. It also
handles the walk (everyone folds to BB → only one live player → hand over).

- **Dead blinds / missed blinds.** Two distinct things:
  - **Rebuy** (adding chips in your seat) happens only *between* hands, owes
    no blind, and is capped so `stack ≤ MaxBuyIn` (§5). Nothing "dead" here.
  - **Returning from sit-out / newly seated:** the player **waits for the
    BB** — they are dealt in only when their seat is due to post the big
    blind. Rejected alternative: casino-style "post dead SB + live BB to play
    immediately". It is more authentic but injects dead money that corrupts
    the pot-odds arithmetic the coach is trying to teach, for a situation
    (human sitting out mid-session of a solo learning game) that is rare.
    The `Table` tracks a per-seat `waitingForBB` flag; the engine's hand
    logic never sees dead blinds at all.

### 3.3 Side pots

**Design decision: pots are *derived*, not maintained.** The engine does not
mutate a pot-object tree as actions happen. It records, per seat, the total
chips contributed to the hand (`TotalCommitted[seat]`) and who has folded.
The pot structure is a **pure function** of those two facts, computed on
demand (for the TUI's pot display) and at showdown (for awarding). This
eliminates the entire class of "pot got out of sync with contributions" bugs
that plague incremental implementations, at the cost of an O(n log n)
recompute over at most 6 seats — free.

```go
// Pot is one layer of the (possibly split) pot.
type Pot struct {
    Amount   Chips
    Eligible SeatSet // live seats that can win this pot
}

// BuildPots derives the pot layers from total contributions.
// Also returns the uncalled excess to refund (see algorithm step 1).
func BuildPots(committed [MaxSeats]Chips, folded, live SeatSet) (pots []Pot, refund [MaxSeats]Chips)
```

**Algorithm** (N players all-in at arbitrary depths):

1. **Refund the uncalled excess.** If the highest contribution among *live*
   players exceeds the second-highest, the difference was never called;
   return it to that player before building pots. (Folded players'
   contributions are never refunded.)
2. Collect the **distinct contribution levels** of live players, ascending:
   `L1 < L2 < … < Lk`.
3. For each level `Li` (with `L0 = 0`), build one pot layer:
   - `Amount = Σ over ALL seats (including folded) of
      clamp(committed[seat], Li) − clamp(committed[seat], Li−1)`
     — folded players' money falls into the lowest layers it reaches, where
     it belongs.
   - `Eligible = { live seats with committed ≥ Li }`.
4. Adjacent layers with identical `Eligible` sets merge (purely cosmetic; it
   keeps the TUI from displaying phantom side pots).

**Awarding at showdown** (`pot.go`):

For each pot, from the last side pot down to the main pot: evaluate
`EvalHoldem` for each eligible seat, find the max `HandRank`, split
`Amount` equally among the tied winners. **Odd chips:** the remainder
(at most `winners−1` chips) is distributed one chip each to the tied winners
in clockwise order **starting from the first seat left of the button**.
(Rejected alternative: stud's "odd chip to highest card by suit" — that rule
exists for stud's suit-ordered bring-in culture; hold'em rooms use
position.)

**Worked example.** Blinds 1/2, four players. Seat D posts BB 2 and folds to
a shove; final total contributions: A = 25 (all-in), B = 80, C = 60 (all-in),
D = 2 (folded). Live = {A, B, C}.

1. Refund: highest live contribution 80 (B), second-highest 60 → refund
   **20 to B**. Effective: A 25, B 60, C 60, D 2.
2. Live levels: 25, 60.
3. Layers:
   - **Main pot** (0→25]: A 25 + B 25 + C 25 + D 2 = **77**, eligible {A,B,C}.
   - **Side pot** (25→60]: B 35 + C 35 + D 0 = **70**, eligible {B,C}.
   - Check: 77 + 70 + 20 refund = 167 = 25+80+60+2. Chips conserve.
4. Showdown, case 1 — C has the best hand overall: C wins side pot 70
   (best of {B,C}) and main pot 77 → C collects 147.
5. Showdown, case 2 — A and C tie for best, B worst: side pot 70 → C alone
   (A isn't eligible). Main pot 77 splits A/C: 38 each, 1 odd chip to
   whichever of A, C sits first clockwise from the button.

Note what the layering guarantees automatically: A can never win more than
25 × (number of callers) + dead money, and B's uncalled 20 never enters any
pot. These two properties are the acceptance tests for `BuildPots`.

**Showdown reveal order** (recorded in events for the TUI; the engine always
knows all cards): last aggressor on the river shows first, then clockwise;
if the river checked through, first live seat left of the button shows first.
Losing hands are recorded as mucked-but-known — the post-hand review reveals
everything anyway, per the product brief.

---

## 4. Action model

Mirrors euchre's `Action` interface exactly in spirit:

```go
type Seat int8            // 0..MaxSeats-1; -1 = no seat
const MaxSeats = 6
type SeatSet uint8        // bitmask of seats

type ActionType uint8
const (
    ActionFold ActionType = iota
    ActionCheck
    ActionCall
    ActionBet   // opening wager on a street where CurrentBet == 0
    ActionRaise // increasing a nonzero CurrentBet (preflop opens are raises over the BB)
)

// Action is a player decision applied to a Hand.
type Action interface {
    Type() ActionType
    Seat() Seat
}

type Fold  struct{ S Seat }
type Check struct{ S Seat }
type Call  struct{ S Seat }              // amount is computed by the engine —
                                         // callers can't submit a stale amount
type Bet   struct{ S Seat; Amount Chips }
type Raise struct{ S Seat; To Chips }    // raise-TO total, not raise-by
```

There are **no posting or dealing actions**: blinds and cards are compulsory
and deterministic, so they are engine-applied side effects of
`Table.StartHand`, visible as events (§5.2), never as decisions. This is a
deliberate divergence from euchre (whose `DiscardAction` etc. are genuine
decisions): an `Action` in this engine is *always* a choice, which keeps
`LegalActions` meaning exactly "the choices".

### 4.1 `LegalActions` — ranges, not enumerations

NL sizing is continuous, so "return `[]Action`" (euchre's shape) can't
enumerate every legal bet. The engine returns *option descriptors* with
min/max, and concrete `Action` values are constructed from them:

```go
// ActionOption describes one legal action type with its sizing bounds.
// For Fold/Check: Min = Max = 0.
// For Call:       Min = Max = chips required to call (capped at stack ⇒ all-in call).
// For Bet:        Min = min bet, Max = stack (all-in). Amount is street-total.
// For Raise:      Min = min legal raise-to, Max = raise-to all-in.
//                 If the seat's raising rights are closed (incomplete-raise
//                 rule) Raise is simply absent from the slice.
type ActionOption struct {
    Type     ActionType
    Min, Max Chips
}

// LegalActions returns the acting seat's options. Empty slice iff the hand
// is not awaiting a decision (PhaseShowdown/Complete or run-out).
func (h *Hand) LegalActions() []ActionOption

// Validate reports whether a would be legal right now, without applying it.
// The coach uses this to grade hypotheticals cheaply.
func (h *Hand) Validate(a Action) error

// Apply validates and applies an action, advancing streets/phases as needed
// (including auto run-out). It is the ONLY mutator during a hand.
func (h *Hand) Apply(a Action) error
```

The TUI renders its buttons/slider directly from `[]ActionOption`; the AI and
coach choose within the same bounds. Illegal states are unrepresentable in
the sense that both consumers *construct* actions from the option list, and
`Apply` re-validates anyway (defense in depth — same layering as euchre's
`LegalActions` + `ApplyAction` error return).

Special cases encoded in the options, so no consumer re-derives rules:

- All-in call for less than the price: `Call{Min=Max=stack}`.
- Short all-in "raise": `Raise{Min=Max=stack-total}` when
  `stack-total < CurrentBet + LastFullRaiseSz` but `> CurrentBet`.
- Check only when `Committed[seat] == CurrentBet` (BB option preflop, or an
  unopened postflop street).

---

## 5. Game / table lifecycle

`Table` is the analogue of euchre's `Game`: it owns configuration and
cross-hand state (stacks, button, seat status) and manufactures `Hand`s.

```go
type TableConfig struct {
    SmallBlind, BigBlind Chips
    MinBuyIn, MaxBuyIn   Chips // e.g. 40BB / 100BB
    Seats                int   // ≤ MaxSeats; 6 for this product
}

type SeatStatus uint8
const (
    SeatEmpty SeatStatus = iota
    SeatActive
    SeatSittingOut
    SeatWaitingForBB // seated (or returned) but not dealt in until BB reaches them
)

type Table struct { /* config, seat states, stacks, button, hand counter, rng seed */ }

func NewTable(cfg TableConfig) *Table

// Seating & bankroll — all only effective between hands; calls during a live
// hand return ErrHandInProgress.
func (t *Table) Sit(seat Seat, name string, buyIn Chips) error
func (t *Table) Leave(seat Seat) error
func (t *Table) SitOut(seat Seat) error
func (t *Table) SitIn(seat Seat) error            // → SeatWaitingForBB
func (t *Table) Rebuy(seat Seat, amount Chips) error // stack+amount ≤ MaxBuyIn

// Hand lifecycle.
func (t *Table) StartHand(src CardSource) (*Hand, error) // posts blinds, deals
func (t *Table) FinishHand(h *Hand) error // applies HandResult to stacks, moves button
func (t *Table) Button() Seat
func (t *Table) Stack(seat Seat) Chips
```

**Button rotation — decision: simple moving button.** After each hand the
button advances to the next seat that will be dealt in. Rejected alternative:
the casino **dead-button** rule (button can point at an empty seat so nobody
skips blinds). Dead button exists for fairness between humans in a game with
money; here 5 of 6 seats are AIs that auto-rebuy and never sit out, so the
situations where the rules differ are rare, and the dead-button state machine
(dead SB, dead BB, button on empty seat) is a notorious bug farm. The
simplification is documented behavior, not an accident. Heads-up (everyone
else sat out): button = SB per §3.2, and the button alternates.

**Busting:** a seat that finishes a hand with stack 0 is flagged; the app
layer decides (human: rebuy prompt or sit out; AI: auto-rebuy to `MaxBuyIn`
— policy lives in the app/ai layer, the engine only exposes `Rebuy`).

**Rake: none, on purpose.** Three reasons: (1) the coach teaches pot odds,
and rake makes every pot-odds example off by a table-specific fudge factor —
actively harmful for learning; (2) there is no economy to protect in a
single-player learning tool; (3) YAGNI — if ever wanted, it is one deduction
hook at pot-award time in `FinishHand`, and retrofitting it there touches
nothing else. No rake field exists in `TableConfig`; its absence is the
decision.

### 5.2 The hand object and its event log

```go
type Hand struct { /* unexported: cfg snapshot, deck, phase, street, board,
                      holes, per-seat state, bettingState, event log */ }

// Read-only views (the TUI, coach, and AI consume these; parity with
// euchre's GameState accessor style):
func (h *Hand) Phase() Phase
func (h *Hand) Street() Street
func (h *Hand) Board() []Card
func (h *Hand) HoleCards(seat Seat) ([2]Card, bool)
func (h *Hand) CurrentSeat() Seat            // -1 when no decision pending
func (h *Hand) Live() SeatSet                // not folded
func (h *Hand) Stack(seat Seat) Chips        // behind, mid-hand
func (h *Hand) Committed(seat Seat) Chips    // this street
func (h *Hand) PotTotal() Chips              // everything committed so far
func (h *Hand) Pots() []Pot                  // derived view, incl. side pots
func (h *Hand) ToCall(seat Seat) Chips
func (h *Hand) Button() Seat
func (h *Hand) Events() []Event
func (h *Hand) Result() (*HandResult, bool)  // ok only in PhaseComplete

// Clone deep-copies the hand. Cheap: all state is value types / small slices.
// The coach clones to evaluate hypothetical lines without touching the real
// hand; the AI may clone for lookahead.
func (h *Hand) Clone() *Hand
```

Every state change appends an `Event` — this is the raw material for the
post-hand review and the coach's grading, so it is engine-owned, not
reconstructed by the UI:

```go
type EventKind uint8
const (
    EvPostBlind EventKind = iota // Seat, Amount
    EvDealHole                   // Seat (cards in Cards; hidden by UI as needed)
    EvDealBoard                  // Street, Cards
    EvAction                     // Seat, ActionType, Amount (street-total), PotAfter
    EvRefundUncalled             // Seat, Amount
    EvShowdown                   // Seat, Cards, Rank (eval.HandRank)
    EvAwardPot                   // Seat, Amount, pot index
)

type Event struct {
    Kind     EventKind
    Street   Street
    Seat     Seat
    Action   ActionType
    Amount   Chips
    PotAfter Chips
    Cards    []Card
    Rank     eval.HandRank
}

type HandResult struct {
    Awards   []PotAward           // per pot: winners, amounts, ranks
    Net      [MaxSeats]Chips      // won − invested, per seat (sums to 0)
    WentTo   Street               // how far the hand got
    Showdown bool
}
```

---

## 6. Testability & determinism

The lessons/drills requirement — "deal exactly this teaching situation" — is
served by one constructor that composes everything above:

```go
// HandSetup describes an exact scenario. Anything omitted is filled
// deterministically from Seed.
type HandSetup struct {
    Config TableConfig
    Button Seat
    Stacks map[Seat]Chips     // seats present in the hand
    Holes  map[Seat][2]Card   // optional per seat; unspecified seats get seeded-random cards
    Board  []Card             // 0..5 cards; the runout prefix
    Seed   uint64             // fills every unspecified card; also recorded for replay
}

// NewHandFromSetup builds a Hand in PhaseBetting(Preflop) with blinds posted.
// It validates: no duplicate cards, stacks within table rules, ≥2 seats.
// Internally it lays the scripted cards into deal order (§1.2) and backs the
// hand with a ScriptedDeck.
func NewHandFromSetup(s HandSetup) (*engine.Hand, error)
```

A lesson step then reads like a sentence:

```go
h, _ := engine.NewHandFromSetup(engine.HandSetup{
    Config: engine.TableConfig{SmallBlind: 1, BigBlind: 2},
    Button: 0,
    Stacks: map[engine.Seat]engine.Chips{0: 200, 1: 200, 2: 200},
    Holes:  map[engine.Seat][2]engine.Card{2: engine.Holes("9c 9d")},
    Board:  engine.MustCards("Ah 9s 4s"),
    Seed:   42,
})
```

Test-suite pillars (beyond the eval tests in §2.4):

- **Chip conservation invariant:** after *every* `Apply`,
  `Σ stacks + Σ committed == Σ starting stacks`; `HandResult.Net` sums to 0.
  Asserted by a helper wrapped around every scenario test.
- **Fuzz via legality:** drive random hands by repeatedly picking a uniformly
  random element of `LegalActions()` (and a random size within `[Min,Max]`)
  until `PhaseComplete`; assert invariants and termination. This is the
  single highest-value test for the betting engine — it finds `ToAct`
  bookkeeping bugs no hand-written scenario anticipates.
- **Scenario tables** for the named rules: min-raise ladder, incomplete-raise
  reopening (a table of "who may raise now" cases against
  `ActedAtBet < LastFullRaiseTo`), BB option, heads-up order, walk,
  three-way all-in side pots (the §3.3 worked example is a literal test),
  odd-chip placement.
- **Replay determinism:** record a hand's seed + action list; re-running
  produces byte-identical event logs.

Same philosophy as euchre's `deck.Seed(1)` tests, upgraded from "stable
shuffle" to "fully scripted deal", because lessons need exact cards, not just
reproducible ones.

---

## 7. Package & file layout

```
internal/engine/
  card.go          Card, Rank, Suit, CardSet; String/Code/ParseCard/MustCard/
                   ParseCards/MustCards, Holes helper
  card_test.go
  deck.go          CardSource, Deck (seeded PCG), ScriptedDeck
  deck_test.go
  chips.go         Chips + formatting helpers (e.g. "1,240 (620 BB)")
  action.go        ActionType, Action interface, Fold/Check/Call/Bet/Raise,
                   ActionOption
  betting.go       bettingState; legality (LegalActions/Validate internals);
                   min-raise & reopen logic; ToAct bookkeeping
  betting_test.go  min-raise ladder, reopen table, BB option, HU order, fuzz
  pot.go           Pot, BuildPots, awarding, odd-chip rule, PotAward
  pot_test.go      worked example, N-way all-in property tests
  hand.go          Hand: street state machine, Apply, run-out, showdown,
                   Clone, HandSetup + NewHandFromSetup, HandResult
  hand_test.go     full-hand scenarios, chip conservation, replay determinism
  table.go         Table, TableConfig, SeatStatus; seating, rebuy, sit-out,
                   waiting-for-BB, button rotation, StartHand/FinishHand
  table_test.go
  events.go        EventKind, Event
  errors.go        ErrNotYourTurn, ErrIllegalAction, ErrBetSizing,
                   ErrHandInProgress, ... (typed sentinels, matches euchre's
                   error style)

internal/eval/
  rank.go          Category, HandRank packing, String, Describe
  eval.go          Eval5, Eval7, EvalHoldem, Best5
  tables.go        init()-built straight-lookup table (8 KB) and rank-name data
  eval_test.go     exhaustive 5-card census, oracle cross-check, adversarial set
  rank_test.go     Describe/String golden tests
```

Dependency edges: `eval → engine` (for `Card` only); `engine` imports nothing
project-local. The future `internal/ai`, `internal/equity`, and coach code sit
above both, exactly as euchre's `ai` sits above its `engine`.

### Key deliberate divergences from the euchre engine, summarized

| Euchre | Hold'em | Why |
|---|---|---|
| `Card` struct (2 fields) | `Card uint8` + `CardSet uint64` | evaluator/equity throughput; bitset ops |
| `Hand` container type | `[2]Card` hole cards | fixed size; nothing to manage |
| `LegalActions() []Action` | `LegalActions() []ActionOption` | NL sizing is a range, not an enumeration |
| Posting/discard are Actions | blinds are engine events | compulsory ≠ a decision |
| Incremental scoring state | pots derived from contributions | kills a whole bug class |
| `math/rand` global fallback | rand/v2 PCG, explicit seed always | cross-version reproducible lessons |
| `Game`/`Round` | `Table`/`Hand` | poker vocabulary; same responsibilities |
