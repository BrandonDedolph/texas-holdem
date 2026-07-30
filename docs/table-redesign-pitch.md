# Table Redesign Pitch — four directions for the in-game screen

Every current-state frame referenced below is a real render, captured by
driving the built table through Bubble Tea at 80×24, 104×30 and 60×20
(the same harness as `internal/app/golden_test.go`). The mockups are
hand-drawn but column-checked: every 80×24 mockup in this document is
exactly 24 rows and ≤80 columns, verified mechanically before commit.

**A note on numbers.** Chip counts, pot odds and equities in the mockups are
taken from real captured frames where possible (the 7♥7♣ hand from the golden
scenarios). Values marked `~` are illustrative stand-ins for what the coach's
`Rationale` would supply at runtime. Per `DESIGN.md` §4, no hand-written
number survives implementation — everything on a real screen is computed.

---

## 0. The brief this pitch answers

The owner's feedback on the current table screen, distilled:

1. **"Cluttered / hard to scan"** — the eye doesn't know where to land first.
2. **"Visually plain"** — the information is fine but the rendering is drab:
   weak use of color, structure, boxes.
3. **The standout: "It should be obvious what's what and understand the
   order, who's your neighbor on each side."** — He cannot read the seating
   order or adjacency off the current screen: who acts before him, who acts
   after him, who is immediately left and right. This is not cosmetic.
   Position in poker *is* the order of action; the app has a whole lesson
   called "Position Is Power"; and the screen he stares at for hours obscures
   the one thing that lesson teaches. **This is the primary requirement.**

He wants **all four** of these at a glance:

- his hand and what it can become (strength, draws, outs)
- the price and whether it's good (pot odds vs equity)
- what the coach thinks and why
- opponent reads (who's loose/tight, who not to bluff)

That is a lot for 80×24, and it collides with complaint #1 — so the win
condition is **hierarchy, not subtraction**. All four present, unmistakably
ranked, so the eye lands in the right order. Directions that drop information
answer a question he didn't ask.

He also supplied a visual reference: a polished graphical hold'em client with
a **literal drawn oval table** — seats as uniform bordered boxes sitting *on*
a closed ring (so adjacency is traceable by eye), hero's seat filled in an
accent color, chips-in-front on the inner edge between seat and pot, pot at
top-center of the felt, board cards as the high-contrast center, and
**color-coded action buttons** (red fold, blue call, green check, gold
all-in) with percentage presets and a slider. Direction A is a serious
attempt to land that reference in 80×24 monospace. He asked for conservative
*and* radical pitched side by side; Directions B–D abandon or restructure the
table metaphor in three different ways.

## 1. Why the current screen fails those requirements

From the captured frames (`internal/app/testdata/table_preflop_80x24.golden`
and live renders):

```
       Nia ᴮᴮ 990               Cole ᵁᵀᴳ 1,000           Ivy ᴴᴶ 970
     ▓▓ ▓▓  ● 10              · ·                      ▓▓ ▓▓  ● 30
                              folds                    raises to 30
```

- **Order is a puzzle.** Seats are floating text blocks in a 3/2/1 grid with
  nothing connecting them. The action order (hero → mid-left → top-left →
  top-center → top-right → mid-right) exists in the geometry but nothing
  *draws* it; superscript badges (ᴮᴮ ᵁᵀᴳ) are the only clue, and they demand
  the player already know that UTG acts first preflop but SB acts first
  postflop. "Who is my neighbor" is answerable only by mental rotation.
- **No hierarchy.** Every region renders at the same visual weight — same
  colorless text, same density. The most important object on the screen (the
  decision: price vs. equity) lives in the smallest type, split between a
  right-aligned info cell and the coach's prose.
- **Reads don't exist on screen.** The archetypes are fully implemented
  (`internal/ai/archetypes.go` — nit, TAG, LAG, station, maniac, each with a
  teaching blurb) and the screen never shows them. A beginner is told
  "never bluff the station" in a lesson and then plays against five
  identical-looking names.
- **Plainness.** After the chrome pass, the table is full-bleed text with
  thin rules. The action bar is undifferentiated keycaps: `f fold  c call 10
  r raise…`. Nothing is a box, a button, or a block of color; the four-color
  deck is the only saturated ink on the screen.
- **Dead rows.** Two blank rows around the action bar and the between-hands
  strip in every 80×24 frame, while the coach clips at `[e more]`.

## 2. Cross-cutting moves (apply to every direction)

These four upgrades are direction-independent; every mockup below assumes
them, and any of them could ship alone as a first increment.

**2.1 Semantic-color action buttons.** The action bar becomes labeled blocks
on colored backgrounds, not keycaps. One row, same fixed budget:

```
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)
```

`▓ … ▓` marks a reverse-video block: FOLD on the warn red, CHECK on felt
green, CALL on accent blue, RAISE/BET on a new violet (one new hex pair in
`palette.go` — currently there is no fifth semantic color), ALL-IN on gold.
The learner absorbs "red = give up, gold = everything" without being taught.
Sizing presets get the same treatment. ASCII/no-color fallback: the current
keycap text unchanged, so nothing is lost on dumb terminals. Cost: small —
styling in `action_bar.go` + one palette entry; the keybind-legend test
already pins the text content.

**2.2 Surface the reads.** Each opponent carries a one-word read derived from
its archetype key: `tight` (nit), `solid` (TAG), `loose` (LAG), `sticky`
(station), `wild` (maniac). The lineup is already in `TableConfig`; the
label is a lookup, not an inference. Cost: trivial, and it makes the
"opponent reads" glance item real in every direction below.

**2.3 One order API.** Every direction needs "who acts in what order this
street." The engine already knows (`Hand.CurrentSeat()`, `Live()`, blinds,
button); it needs one small pure helper, e.g. `h.StreetOrder() []Seat` —
the seats in action order for the current street, folded seats excluded or
flagged. Cost: small engine addition plus tests; every direction consumes it.

**2.4 Potential, not just strength.** The hand-strength label ("pair of
sevens") gains a second clause about what the hand can become ("flops a set
about 1 in 8"). Preflop this is a per-hand-class canned line (a ~10-entry
table); postflop it is outs/draws the equity package already computes. Cost:
small coach addition; principle #3 is preserved because the numbers come from
the same `Rationale` facts.

---

## Direction A — "The Drawn Table" (conservative)

**Thesis: the felt-table metaphor isn't broken — it was never actually
drawn. Draw the ring, put the seats on it, and order/adjacency become
something you can trace with your eye.**

This is the reference image translated to 80×24: a closed racetrack of
box-drawing characters, uniform bordered seat boxes sitting *on* the ring
(their bottom edges are ring segments), hero embedded in the bottom edge as
a filled accent-color block, chips-in-front on the inner edge between seat
and pot, pot at top-center of the felt, board as the high-contrast center.

### A at 80×24 — preflop, facing Ivy's raise to 30 (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
     ╭─ Nia ᴮᴮ · sticky ──╮  ╭─ Cole ᵁᵀᴳ · tight ─╮  ╭─ Ivy ᴴᴶ · wild ────╮
     │ 990                │  │ 1,000       folded │  │ 970         raised │
   ╭─┴────────────────────┴──┴────────────────────┴──┴────────────────────┴─╮
   │          ● 10                   POT 45                   ● 30          │
   ├─ Tara ˢᴮ · loose ──╮                              ╭─ Sam ᶜᴼ · solid ───┤
   │ 995                │ ● 5  ·    ·    ·    ·    ·   │ 1,000      folded  │
   ├────────────────────╯                              ╰────────────────────┤
   │                          ╭──╮ ╭──╮                                     │
   │                          │7♥│ │7♣│      pair of sevens                 │
   │                          ╰──╯ ╰──╯                                     │
   ╰─────────────────────────┤ ► YOU ᴮᵀᴺ Ⓓ 1,000 ├──────────────────────────╯
  hand   flops a set about 1 in 8 — unimproved, it is often second best
  order  Cole ✗ → Ivy 30 → Sam ✗ → ► YOU → Tara → Nia
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30
        77 is in the call-vs-open chart. The price is 1.5:1 — risk 30 to win
        45 → you need 40%; you have ~48% vs the ≈18% Ivy opens.      [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)


 your turn · you act 4th of 6 — Tara and Nia still behind you
                                                   esc menu · ? help
```

What the color pass adds (invisible in mono): the ring border in felt green
with the hero block reversed on the accent color; the `►` to-act marker and
its seat border pulse accent; folded boxes dim to muted; `POT 45` in felt
green; reads in muted italic; FOLD red / CALL blue / RAISE violet blocks.

### A at 80×24 — flop, bet-sizing open (checked to hero, pot 30)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · FLOP                        BTN · ? help
     ╭─ Nia ᴮᴮ · sticky ──╮  ╭─ Cole ᵁᵀᴳ · tight ─╮  ╭─ Ivy ᴴᴶ · wild ────╮
     │ 990        checked │  │ 1,000       folded │  │ 1,000       folded │
   ╭─┴────────────────────┴──┴────────────────────┴──┴────────────────────┴─╮
   │                                 POT 30                                 │
   ├─ Tara ˢᴮ · loose ──╮     ╭──╮ ╭──╮ ╭──╮           ╭─ Sam ᶜᴼ · solid ───┤
   │ 990        checked │     │8♥│ │J♣│ │T♠│  ·    ·   │ 1,000      folded  │
   ├────────────────────╯     ╰──╯ ╰──╯ ╰──╯           ╰────────────────────┤
   │                          ╭──╮ ╭──╮                                     │
   │                          │7♥│ │7♣│      pair of sevens                 │
   │                          ╰──╯ ╰──╯                                     │
   ╰──────────────────────────┤ ► YOU ᴮᵀᴺ Ⓓ 990 ├───────────────────────────╯
  hand   J and T are over your pair — two streets of danger cards to come
  order  Tara ✓ → Nia ✓ → ► YOU          (postflop the blinds act first)
────────────────────────────────────────────────────────────────────────────────
 COACH  Check
        Second pair on a wet board wants a cheap showdown — a bet folds out
        worse hands and gets called by better ones. Check behind.   [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓1 ⅓ 10▓  ▓2 ½ 15▓  ▓3 ⅔ 20▓  ▓4 POT 30▓  ▓5 ALL-IN▓             bet: 15
 min 10 ├──●────────────────────────────┤ 990     Tara calls 15 → needs 25%
 enter bet 15 · esc cancel

                                                   esc menu · ? help
```

**The cell arithmetic, honestly.** The ring (rows 2–12) costs 11 rows; the
current seat/board/hero region costs 13 (rows 3–15). The ring is *cheaper*
because the seat boxes are 2 visible rows (name+read in the border, stack +
status inside, bottom edge shared with the ring) instead of 3, and because
the last-action verb line is gone. The savings fund the `hand` and `order`
lines. What a terminal cannot do: curves. This is a rounded rectangle, not
an oval, and the six seats sit 3-top / 2-mid / hero-bottom exactly as today —
the *drawing* changes, the geometry doesn't. Box-drawing junctions (`┴ ├ ┤`)
all have same-width ASCII fallbacks (`+ | -`), so the ASCII render keeps the
identical grid.

**What it sacrifices, named.** The per-seat action verb ("raises to 30"
in full) is compressed to a status word in the box plus the amount on the
chip slot and the order rail. Two-line seat blocks can't show villain
face-down card glyphs (`▓▓ ▓▓`) — live-vs-folded is carried by dim/bright
and the chip slot instead; at showdown the revealed cards take over the
status slot (`shows T♣ 6♠`). The felt's interior blank space is spent on
being a table rather than on more information.

**How the four glance items rank.** (1) The eye lands center — board and
pot, the brightest objects. (2) Hero block + hole cards directly below,
with the `hand` line beneath. (3) The coach strip keeps its 3 rows. (4)
Reads live on every seat border, ambient rather than ranked. The *price*
is the weakest of the four here — it sits in the coach prose and the info
cell, as today. That is the cost of spending the pixels on the table.

**Order/adjacency answer.** Threefold: the closed ring makes neighbors
literally adjacent along a traceable line; the `order` rail states the
street's sequence with fold/bet marks and re-sorts each street (the
preflop→postflop change is *shown*, not assumed); the status line says
"you act 4th of 6 — Tara and Nia still behind you" in words.

**Breakpoints.** 104×30: the ring widens, and the reference's tabbed side
panel maps to the existing right panel — coach reasoning on top, the
action log and session stats below (the wide layout already renders both).
60×20: the ring cannot be drawn honestly in 20 rows; compact keeps the
current ledger, gains the read column and an `order` line, and sorts rows
in street order (see Direction B — the compact ledger *is* a queue).

**Build cost.** Medium. New: ring compositor in `table_layout.go` (the
hardest part — junction-correct borders at three widths of seat name),
2-row `SeatBox` replacing `SeatView`'s 3-row form, order rail, §2 items.
Reused: `MiniCard`, `BoardRow`, coach strip, action bar frame, pot math.
All 14 goldens and the stability-matrix anchors are re-recorded — that is
mechanical but it is the whole test surface of the screen. Estimate: the
largest of the four directions in pure rendering fiddliness, no new data.

**Wrong for.** Someone who decides, after a week, that he mostly wants the
numbers — the table spends ~11 rows being furniture, and the price/equity
comparison never gets bigger than one line. If complaint #1 ("where do I
look?") is really about the *decision* rather than the *room*, this
direction polishes the room.

---

## Direction B — "The Action Queue" (radical: order is the layout)

**Thesis: position is your place in a line, so draw the line. Abandon the
felt; render the players as the action queue itself, and give the decision
the left half of the screen, ranked top-to-bottom in the order a player
should think: hand → board → price → edge.**

The right column is the queue: every seat in this street's action order,
top to bottom, hero embedded — everyone above the hero row acts before
him, everyone below acts after. "Who is my neighbor" becomes literal
reading order. The queue re-sorts between streets, which teaches the
single most confusing fact about position (blinds act *last* preflop,
*first* postflop) every time it happens, for free.

### B at 80×24 — preflop, facing Ivy's raise to 30 (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
────────────────────────────────────────────────────────────────────────────────
 YOUR HAND                                │ ACTION ORDER · preflop
  ╭──╮ ╭──╮   pair of sevens              │  1  Cole · tight   1,000   ✗ fold
  │7♥│ │7♣│   flops a set about 1 in 8    │  2  Ivy · wild       970   RAISE 30
  ╰──╯ ╰──╯   unimproved, often 2nd best  │  3  Sam · solid    1,000   ✗ fold
                                          │ ►4  YOU · BTN Ⓓ    1,000   ?
 BOARD    ·    ·    ·    ·    ·           │  5  Tara · loose     995   ● 5
          nothing dealt yet               │  6  Nia · sticky     990   ● 10
                                          │
 PRICE    call 30 · pot 45 · 1.5:1        │ behind you: Tara, Nia — they see
          you need 40% to break even      │ your move before they act
                                          │
 EDGE     ~48% vs the ≈18% Ivy opens      │ POT 45 · Ivy has 970 behind
          ├───────────╫██─────────────┤   │
          0       need 40  have ~48  100  │ wild: raises a lot — call lighter
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30
        77 is in the call-vs-open chart. You close the action in position —
        set-mine cheap and stack him when a seven lands.             [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓

 your turn · fold, call, or raise                  esc menu · ? help
```

### B at 80×24 — flop, bet-sizing open (queue re-sorted postflop)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · FLOP                        BTN · ? help
────────────────────────────────────────────────────────────────────────────────
 YOUR HAND                                │ ACTION ORDER · flop
  ╭──╮ ╭──╮   pair of sevens              │  1  Tara · loose     990   ✓ check
  │7♥│ │7♣│   J and T beat your pair      │  2  Nia · sticky     990   ✓ check
  ╰──╯ ╰──╯   two overcards = danger      │ ►3  YOU · BTN Ⓓ      990   ?
                                          │
 BOARD    8♥   J♣   T♠   ·    ·           │ out: Cole · Ivy · Sam
          wet — straights everywhere      │
                                          │ the blinds now act BEFORE you —
 PRICE    check is free · pot 30          │ postflop order starts left of
          a bet must get through two      │ the button
                                          │
 EDGE     ~52% vs two limping ranges      │ POT 30 · both have 990 behind
          ├──────────╫█───────────────┤   │
          0      need 25  have ~52   100  │ sticky: calls a lot — don't bluff
────────────────────────────────────────────────────────────────────────────────
 COACH  Check
        Second pair on a wet board wants a cheap showdown — betting folds
        out worse and feeds better. Check behind.                    [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓1 ⅓ 10▓  ▓2 ½ 15▓  ▓3 ⅔ 20▓  ▓4 POT 30▓  ▓5 ALL-IN▓             bet: 15
 min 10 ├──●────────────────────────────┤ 990     Tara calls 15 → needs 25%
 your turn · sizing a bet                          esc menu · ? help
```

(The EDGE meter's `need` tick when facing no bet marks the villain's price
to continue if hero bets — the number the sizing preview already teaches.
When facing a bet it marks hero's own break-even; the block between `╫` and
`█` is the margin, green when have > need, red when it isn't. That margin
bar is the single most teachable pixel in this pitch: pot odds stop being
two abstract percentages and become a visible gap.)

**What it sacrifices, named.** The table. There is no felt, no spatial
room — the game looks like a briefing, not a card room. Chips do not sit
"in front of" anyone; they are a column. Showdown loses its theater: cards
reveal in the queue rows rather than flipping around a table. And the
layout is unapologetically asymmetric — hand left, people right — which
reads as an instrument, not a place.

**How the four glance items rank.** This direction is *built* as their
ranking: (1) hand + potential, top-left, biggest; (2) price and the edge
meter, mid-left; (3) coach, full width below; (4) queue with reads, right,
ambient — plus one rotating read hint (bottom-right) that surfaces the
most decision-relevant archetype note for whoever is live. Hierarchy is
solved by geometry: the left column *is* the thinking checklist, in order.

**Order/adjacency answer.** The strongest of the four directions: order is
the organizing principle. Before/after is above/below the hero row;
neighbors are the adjacent rows; street re-sorting demonstrates the
preflop/postflop switch every hand; the prose cell ("behind you: Tara,
Nia") says it in words. What it does *not* teach: the circular geometry of
a physical table (that Nia at "6" is also the person one seat to hero's
left). A learner headed for live play eventually needs the circle.

**Breakpoints.** 104×30: the queue keeps its column and the extra 24 cols
go to the coach side panel as today (full reasoning, no `[e more]`), with
the action log beneath it. 60×20: the queue *is* the compact ledger sorted
by street order with a read column — the compact layout barely changes,
which is a point in this direction's favor: one mental model at every size.
The left column compresses to the current compact rows (board line, coach
line with price, bar).

**Build cost.** Medium-small — the cheapest radical option. The queue is
`SeatView.RenderCompact` plus sorting by §2.3's order API and a read
column. New components: the edge meter (pure function, trivially golden-
tested) and the two-column composition in `table_layout.go`. The left
column's annotation lines are §2.4. Goldens/stability anchors re-recorded.
No new engine data beyond `StreetOrder()`.

**Wrong for.** Anyone who wants the game to feel like poker. If part of
the point of hours at this screen is the *romance* of the table — and
"make it look like the reference client" suggests it may be — this
direction will feel like doing homework. It is also the direction most
likely to still be the wrong shape when he outgrows the checklist: strong
players scan people first, numbers second.

---

## Direction C — "The Hand, Written Down" (radical: time is the layout)

**Thesis: a beginner learns from the hand's story, not its snapshot. Render
the hand as a script that writes itself — streets as chapters, actions in
order, hero's decisions grade-stamped inline — with the current decision
pinned at the bottom.**

The current screen shows only the present; the action so far (who raised,
who just called, what that means) is the context every decision depends on,
and today it evaporates as it happens (at 80 cols the action log exists
only in the wide layout). This direction makes the story the screen. Order
falls out for free: a story *is* the action order, written down.

### C at 80×24 — flop, hero to act, checked to him (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10                             BTN Ⓓ · ? help
────────────────────────────────────────────────────────────────────────────────
 PREFLOP   pot 15    Cole ✗ · Ivy ✗ · Sam ✗ — folded around
                     YOU call 10   ? Inaccuracy — raise to 25 was the play
                     Tara (loose) calls 5 · Nia (sticky) checks
 FLOP      8♥ J♣ T♠  pot 30
                     Tara checks · Nia checks
                     ► YOU — your turn
 TURN      ·

 RIVER     ·

────────────────────────────────────────────────────────────────────────────────
 YOU ᴮᵀᴺ Ⓓ 990    7♥ 7♣   pair of sevens — J and T on board beat your pair
 LIVE     Tara 990 (loose) · Nia 990 (sticky) — both act before you postflop
 PRICE    check is free · pot 30 · a bet must get through two players
 COACH    Check — second pair on a wet board wants a cheap showdown. A bet
          folds out worse hands and gets called by better ones, and both
          blinds get to react to whatever you do here.
────────────────────────────────────────────────────────────────────────────────
  ▓ x CHECK ▓   ▓ b BET… ▓                         pot 30 · Tara 990 behind

 your turn · check or bet
                                                   esc menu · ? help
```

### C at 80×24 — showdown / between hands (the story pays off)

```
 Hand #1 · 6-max NLHE · blinds 5/10                             BTN Ⓓ · ? help
────────────────────────────────────────────────────────────────────────────────
 PREFLOP   pot 15    Cole ✗ · Ivy ✗ · Sam ✗ — folded around
                     YOU call 10   ? Inaccuracy — raise to 25 was the play
                     Tara (loose) calls 5 · Nia (sticky) checks
 FLOP      8♥ J♣ T♠  pot 30
                     Tara ✓ · Nia ✓ · YOU ✓ check   ✔ Best
 TURN      T♦        board pairs — trips are possible now
                     Tara ✓ · Nia ✓ · YOU ✓ check   ✔ Best
 RIVER     4♦        no draw got there
                     Tara ✓ · Nia ✓ · YOU ✓ check   ✔ Best
 SHOWDOWN            Tara shows T♣ 6♠ — three tens · Nia shows 6♥ 2♥ — tens
                     YOU show 7♥ 7♣ — two pair, tens and sevens
────────────────────────────────────────────────────────────────────────────────
 POT 30 → Tara with three of a kind, tens
 GRADES   3 Best · 1 Inaccuracy (-0.7bb) — the preflop call · v replays it
 SESSION  hands 1 · decisions 75% good (3/4) · net -10
 NEXT     the button moves to Tara — you post the small blind next hand
────────────────────────────────────────────────────────────────────────────────
 ▓ space DEAL NEXT ▓   ▓ v REVIEW ▓


 hand complete
                                                   esc menu · ? help · v review
```

**Fixed budgets, honestly.** The story region is 10 rows with per-street
windows: preflop 3, flop 2, turn 2, river/showdown 3 (the two mockups above
allot them slightly differently per phase — the *region* is fixed, the
internal split shifts only at street boundaries, never mid-street). A
raising war can exceed a window, so each street line windows to its last
actions with fold-compression (`Cole ✗ · Ivy ✗ · Sam ✗ — folded around`)
and a `…` continuation marker; the full sequence is always one `v` away.
This windowing is the direction's engineering heart and its biggest risk —
it must never reflow, only rewrite in place.

**What it sacrifices, named.** The snapshot. Stacks are not a column you
scan — they appear where the story mentions them and in the LIVE line, so
"who has how much" takes reading, not glancing. Multiway pots with four
live seats make the LIVE line dense. The table metaphor is gone here too,
replaced by prose; showdown theater becomes text. And reading is slower
than seeing: at Fast speed this screen would churn text the player stops
reading, so it fights the grind style of play rather than supporting it.

**How the four glance items rank.** (1) The story is the headline — which
means *the villains' behavior* gets the most pixels of any direction (this
is where "Ivy raised again, third hand running" would someday live). (2)
Hand + potential on the first line below the rule. (3) Price. (4) Coach.
Reads appear inline where the players act — attached to behavior, which is
pedagogically the right place for them.

**Order/adjacency answer.** Order is the text itself: actions are listed in
the order they happened, every street, and the postflop re-order is visible
as a different name leading the flop line. Adjacency ("my left/right
neighbor") is the weakest here — the story tells you *sequence*, not
*seating*. The LIVE line's "both act before you postflop" carries it in
words only.

**Breakpoints.** 104×30: story left, and the right panel holds the frozen
decision desk (hand/price/edge/coach) — the two-column split this direction
actually wants; 80 is its compromise size. 60×20: street windows shrink to
one line each and the desk drops to the compact ledger's coach line; it
degrades to something close to the current compact screen with a 4-line
memory.

**Build cost.** Medium-large — the most new logic. The street-window
renderer with compression rules is new and needs its own stability tests
(the fixed-budget matrix gets a fourth dimension: action-count per street).
Grade stamps and showdown lines come from data that exists (`m.grades`,
engine events — the wide layout's ticker proves the plumbing). The decision
desk reuses §2 components. Everything else is re-recording.

**Wrong for.** The player he'll be in three months. Once betting lines are
familiar, the story is mostly redundant with what he remembers, and the
screen spends its best rows narrating things he saw happen two seconds ago.
Strongest as a *mode* (or as the between-hands/review surface — see the
recommendation) rather than as the only table.

---

## Direction D — "Two Rooms" (structural: the decision moment and the
between-hands moment are different screens)

**Thesis: one compromise layout serves two different moments badly. While a
hand runs, show the minimum needed to decide well — big, calm, ranked.
When the hand ends, swap to a study screen that spends all 24 rows on what
just happened. The dead between-hands rows of the current screen become
the app's best teaching surface.**

The current between-hands frame is the emptiest in the app (two reserved
strips and blank space) at exactly the moment the player is most receptive:
no clock, no decision pending, outcome fresh. Meanwhile the in-hand frame
carries review furniture (`v review` legends, session stats) it doesn't
need. Splitting the moments lets each be honest.

**The stability rule, addressed head-on.** "Fixed row budgets, no reflow on
state change" is load-bearing and this direction *bends* it: hand-end swaps
the whole screen. The defense: the rule exists so the eye always knows
where to look *while things are moving*; the swap happens at the calmest
instant of the loop, is total (a scene cut, not a shuffle — nothing
"moves"), and each room is internally fixed and separately golden/anchor
tested, exactly like navigating to HandReview today. But it is the owner's
rule, and this direction should not be built without his explicit yes.

### D at 80×24 — the Hand Room (preflop, facing Ivy's raise to 30)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
────────────────────────────────────────────────────────────────────────────────
   1  Cole   ᵁᵀᴳ · tight    1,000           ✗ folds
   2  Ivy    ᴴᴶ · wild        970    ● 30   raises to 30
   3  Sam    ᶜᴼ · solid     1,000           ✗ folds
  ►4  YOU    ᴮᵀᴺ Ⓓ          1,000           your turn
   5  Tara   ˢᴮ · loose       995    ● 5    behind you
   6  Nia    ᴮᴮ · sticky      990    ● 10   behind you
────────────────────────────────────────────────────────────────────────────────
 YOUR HAND    ╭──╮ ╭──╮   pair of sevens — flops a set about 1 in 8
              │7♥│ │7♣│
              ╰──╯ ╰──╯   BOARD   ·    ·    ·    ·    ·         POT 45
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30                                  price · call 30 · 1.5:1 · 40%
        77 is in the call-vs-open chart. The price is 1.5:1 — risk 30 to
        win 45, so you need 40%; against the ≈18% of hands Ivy opens from
        the HJ you have about 48%. You close the action in position —
        set-mine cheap and stack him when a seven lands.
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓


 your turn · fold, call, or raise
                                                   esc menu · ? help
```

Note what five uncontested rows buy: **the coach never truncates.** No
`[e more]`, no overlay, no clipped arithmetic — the full reasoning, every
decision, at 80×24. No other direction achieves that at this size.

### D at 80×24 — the Study Room (hand over, before the next deal)

```
 HAND #1 OVER · you lost 10 · Tara won 30 with three of a kind, tens
────────────────────────────────────────────────────────────────────────────────
 WHAT HAPPENED
   PREFLOP    you called 10 with 7♥ 7♣           ? Inaccuracy — raise to 25
   FLOP       8♥ J♣ T♠ — checked around          ✔ Best
   TURN       T♦ — checked around                ✔ Best
   RIVER      4♦ — checked around                ✔ Best
   SHOWDOWN   Tara T♣ 6♠ (three tens) · Nia 6♥ 2♥ (tens) · you two pair
────────────────────────────────────────────────────────────────────────────────
 THE LESSON FROM THIS HAND
   Small pairs want to raise or fold before the flop, not call. The call
   built a small pot you could only win with a set — and let Tara in cheap
   behind you with the hand that beat you.
────────────────────────────────────────────────────────────────────────────────
 SESSION   hands 1 · decisions 75% good (3/4) · net -10
           accuracy and chips can disagree — when they do, that is variance
────────────────────────────────────────────────────────────────────────────────
 NEXT      blinds 5/10 · the button moves to Tara · you post the small blind


  ▓ space DEAL ▓   ▓ v FULL REVIEW ▓   ▓ esc MENU ▓

 between hands
                                                   esc menu · ? help
```

**What it sacrifices, named.** Continuity — the scene cut is a real cost
and some people hate it (twice a minute at speed). Also the hand room's
table is the humblest of the pitch: a sorted ledger, no felt, no drawn
ring; players are rows. (The hand room could equally be Direction A's ring
or B's two-column — Two Rooms is orthogonal to the in-hand choice; it is
shown with the ledger to make the point that even the plainest in-hand
view works when the coach gets real space.) The Study Room adds one new
content demand: "THE LESSON" — cheaply seeded by the worst-graded
decision's existing explanation, but it will want curation over time.

**How the four glance items rank.** In the hand room: coach (biggest block
on screen), hand + potential, price (pinned top-right of the coach block),
reads in the queue. Between hands the ranking flips to grades → lesson →
session → next — which is the honest ranking of that moment.

**Order/adjacency answer.** The hand room's roster is street-ordered like
Direction B's queue (before-you above, behind-you below, in words in the
status column). Same strength, same weakness (no circle). The Study Room's
NEXT line teaches the button's movement — "the button moves to Tara · you
post the small blind" — which no current screen says anywhere.

**Breakpoints.** Hand room: 104×30 moves the coach to the side panel and
restores an action ticker; 60×20 is almost exactly the current compact
ledger (sorted, with reads). Study room: 104×30 adds the full action
transcript beside WHAT HAPPENED; 60×20 drops THE LESSON to one line and
SHOWDOWN to winners only.

**Build cost.** Medium. The hand room is a cheap variant of the compact
ledger (sorting + reads + a taller coach box). The study room is a new
view but every fact on it exists today (`m.grades`, session counters,
engine events, `PotAward`); "THE LESSON" v1 is a selection heuristic, not
new prose generation. The real work is the mode machine in `TableScreen`
(deal ↔ study), doubled goldens/anchors, and the pacing interaction
(SpeedInstant must collapse the study room for tests, like every other
pause).

**Wrong for.** A player who wants flow — deal me the next hand, stop
teaching me between every pot. (Mitigation: at Fast/Instant the study room
auto-skips unless there was a Mistake, which harmonizes with the decided
CoachMistakes philosophy: silence means you're fine.) Also wrong as a
*first* step: it multiplies whatever in-hand layout exists, so it wants to
land after A or B settles.

---

## Side-by-side

Scored for a beginner at this screen, ✦✦✦ best.

| Dimension                          | A Drawn Table | B Queue | C Story | D Two Rooms |
|------------------------------------|:---------:|:-------:|:-------:|:-----------:|
| Order & adjacency legible          | ✦✦✦ (circle + rail) | ✦✦✦ (the layout *is* order) | ✦✦ (sequence, not seating) | ✦✦ |
| Eye knows where to land            | ✦✦ (center felt) | ✦✦✦ (ranked column) | ✦✦ (bottom desk) | ✦✦✦ (one big thing per moment) |
| All four glance items, ranked      | ✦✦ (price weakest) | ✦✦✦ | ✦✦ (stacks weakest) | ✦✦✦ (split across rooms) |
| Teaches the price (odds vs equity) | ✦ | ✦✦✦ (edge meter) | ✦✦ | ✦✦ (in prose) |
| Coach readable without `[e more]`  | ✦ (3 rows) | ✦ (3 rows) | ✦ (2–3 rows) | ✦✦✦ (5 rows) |
| Opponent reads surfaced            | ✦✦ (on seats) | ✦✦✦ (column + hint) | ✦✦✦ (attached to actions) | ✦✦ |
| Looks like poker / transfers to a real table | ✦✦✦ | ✦ | ✦ | ✦ |
| Anti-plain (color, boxes, structure) | ✦✦✦ | ✦✦ | ✦ | ✦✦ |
| Between-hands moment used          | ✦ | ✦ | ✦✦✦ | ✦✦✦ |
| Chrome harmony (header/rule/footer grammar) | ✦✦ (ring is new chrome) | ✦✦✦ | ✦✦✦ | ✦✦✦ |
| Build cost (✦✦✦ = cheapest)        | ✦ | ✦✦✦ | ✦ | ✦✦ |

## Recommendation

**Build Direction A (the Drawn Table) as the in-hand screen, but steal two
organs from B before drawing a single border: the order rail and the edge
meter.** Then adopt **D's Study Room** as a second step.

Reasoning, ranked by how a beginner learns at this screen:

1. The owner's #1 complaint is order/adjacency, and his chosen reference
   solves it with a drawn ring. A and B solve it equally well on paper, but
   A solves it in the vocabulary he already responded to — and A is the only
   direction that keeps rehearsing the *circular* geometry he will face at
   any real table, which is the form position knowledge ultimately has to
   take. B's queue teaches sequence brilliantly and circle not at all.
2. A's genuine weakness — the price never gets big — is exactly B's
   strength, and B's edge meter is one 2-row pure component. It replaces
   the third coach row in A's strip (`need 40 ╫██ have ~48`), giving A the
   one quantitative visual it lacks without costing a row. The order rail
   is likewise direction-independent. Neither requires B's layout.
3. D's Study Room attacks the moment A leaves poorest (between hands) and
   is orthogonal to the in-hand layout; C's best idea — the grade-stamped
   narrative of the hand — is precisely what the Study Room's WHAT
   HAPPENED block is. Build the Study Room and C's soul ships without C's
   windowing engine. Gate it on the owner accepting the scene-cut; if he
   refuses, its content collapses back into the current between-hands
   strips, richer than today.
4. Cross-cutting §2 (colored action buttons, reads, order API, potential
   line) should ship first regardless of direction — every one of them
   improves the current screen even if no direction is ever built, and
   they de-risk the pitch: if colored buttons + reads + order rail on the
   *existing* layout already make him happy, the ring can wait.

Sequenced: §2 first (days, reversible) → A's ring (the big rendering job)
→ D's study room (needs his sign-off on the swap) → revisit whether B's
full queue is wanted as the 60×20 compact form (it nearly is already).

## What I'd want to know from the owner

1. **The transfer question (picks A vs B):** when you imagine yourself at a
   real table in a year, do you want this screen to have been rehearsing
   that geometry all along — or is this app purely a decision gym, and the
   screen should look like whatever teaches fastest? Your reference image
   suggests the former; your four glance items suggest the latter. This is
   the fork.
2. **The scene-cut question (gates D):** may the screen change wholesale at
   hand end — a hard cut to a study view, back on the next deal — or does
   "no reflow" mean *nothing* ever swaps, full stop?
3. **Grind or study (weights C and D):** in a typical session, are you
   playing 100 fast hands or 20 slow ones? The story and the study room
   earn their rows at 20 hands; at 100 they are friction.
4. **How much felt is enough:** if §2 alone (colored buttons, reads on
   seats, the order rail, the potential line) landed on the current layout
   next week, would that already be 80% of what you're missing — or is the
   drawn ring the point? Honest answer changes the budget by a factor of
   five.
5. **The meter check:** does `need 40 ╫██ have ~48` read instantly to you,
   or does it need words? It's the one novel glyph grammar in this pitch,
   and it should be A/B-tested on its only user before it earns a
   permanent row.
