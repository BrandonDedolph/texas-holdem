# Table Redesign Pitch 2 — four more drawn tables

The owner's verdict on the first pitch (`docs/table-redesign-pitch.md`):

> "I like the idea of A more, B is too transactional. Can you pitch 4 more
> designs like A."

So the drawn-felt table wins the family argument, and **"transactional" is
the rejection word**. B's sin was not its information — it was that the
screen read as a *form*: labelled fields, a metered comparison with axis
ticks, players as rows in a register. Efficient, legible, and lifeless.
Whatever ships must feel like sitting at a table, not filling in a ticket.

This pitch answers with **four genuinely different drawn tables**. Each
varies a different axis of the family — ring geometry, point of view, where
the coach lives, how the price is made physical, how much screen the table
deserves — and each is a competitor to A, not a restyle of it.

Every 80×24 mockup below is exactly 24 rows and ≤80 columns, verified
mechanically (row count, per-row display width, and box-junction column
alignment checked by script before commit). Numbers reuse the captured
golden scenarios (the 7♥7♣ hand; the all-in side-pot hand); values marked
`~` are stand-ins for what `Rationale` supplies at runtime — per
`DESIGN.md` §4 no hand-written number survives implementation. All four
directions assume the first pitch's §2 cross-cutting moves (colored action
blocks, surfaced reads, the `StreetOrder()` API, the potential line).

Glyph fallbacks, once for the whole pitch: every new glyph has a same-width
ASCII substitute in the existing `glyphs.go` pattern — `╱ ╲` → `/ \`,
`╴ ╶` → `-`, `◇` → `*`, `◂ ▸` → `<` `>`, `▲` → `^`, `●` → `o`, `▓` → `#`.
The ASCII frame keeps identical geometry, as the current ASCII golden does.

---

## Direction E — "The Hexagon"

**Thesis: a 6-max table already has six sides — give every player one edge
of the ring, and stamp this street's action order onto the rim, so both
adjacency ("who touches me") and order ("who acts when") are properties of
the shape itself.**

A's racetrack kept the current 3-top/2-mid/hero-bottom grouping and drew a
rounded rectangle around it. The Hexagon goes further: the ring is a
six-sided table, one seat per side, arranged in true clockwise seating
order — hero on the bottom edge, Tara (SB) and Nia (BB) stacked on his
left rim, Cole across the top, Ivy and Sam down the right rim. Your left
neighbor is literally the plate above-left of you; the player on your
right is above-right. Each plate carries a small engraved digit — its
action order *this street* — and the digits march clockwise around the
rim. When the street changes, the digits re-stamp: the preflop→postflop
switch (blinds act last, then first) becomes a visible renumbering of the
same six fixed plates, taught by the furniture every single hand.

### E at 80×24 — preflop, facing Ivy's raise to 30 (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
      ╭──────────────────────┤ 1 Cole ᵁᵀᴳ · tight ├──────────────────────╮
     ╱                           1,000 ✗ folded                           ╲
    ╱                                 POT 45                               ╲
   ├ 6 Nia ᴮᴮ · sticky ─╮                               ╭─ 2 Ivy ᴴᴶ · wild ─┤
   │ 990          ● 10  │                               │ ▲ ● 30       970  │
   ├────────────────────╯                               ╰───────────────────┤
   │                          ·    ·    ·    ·    ·                         │
   ├ 5 Tara ˢᴮ · loose ─╮                               ╭─ 3 Sam ᶜᴼ · solid ┤
   │ 995           ● 5  │                               │ ✗ folded    1,000 │
   ├────────────────────╯       ╭──╮ ╭──╮               ╰───────────────────┤
    ╲        pair of sevens ─   │7♥│ │7♣│   ─ flops a set about 1 in 8     ╱
     ╲                          ╰──╯ ╰──╯   ● 30 to call                  ╱
      ╰─────────────────────┤ 4 ► YOU ᴮᵀᴺ Ⓓ 1,000 ├──────────────────────╯
 order   clockwise 1 → 6 · Tara and Nia — your left — still act behind you
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30
        77 is in the call-vs-open chart. Risk 30 to win 45 → you need 40%;
        you have ~48% against the ≈18% of hands Ivy opens.           [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)

 your turn · fold, call, or raise
                                                   esc menu · ? help
```

### E at 80×24 — showdown (Tara's trips take it)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · SHOWDOWN                    BTN · ? help
      ╭──────────────────────┤ ✗ Cole ᵁᵀᴳ · tight ├──────────────────────╮
     ╱                           1,000 · folded                           ╲
    ╱                    POT 30 → Tara · three of a kind, tens             ╲
   ├ 2 Nia ᴮᴮ · sticky ─╮                               ╭─ ✗ Ivy ᴴᴶ · wild ─┤
   │ 990   6♥ 2♥ tens   │                               │ folded     1,000  │
   ├────────────────────╯                               ╰───────────────────┤
   │                      8♥   J♣   T♠   T♦   4♦                            │
   ├ 1 Tara ˢᴮ · loose ─╮                               ╭─ ✗ Sam ᶜᴼ · solid ┤
   │ 1,020  T♣ 6♠ trips │                               │ folded      1,000 │
   ├────────────────────╯       ╭──╮ ╭──╮               ╰───────────────────┤
    ╲          you show ──▸     │7♥│ │7♣│   ─ two pair, tens and sevens    ╱
     ╲                          ╰──╯ ╰──╯   second best — it happens      ╱
      ╰──────────────────────┤ 3 YOU ᴮᵀᴺ Ⓓ 990 ├─────────────────────────╯
 grades  3 Best · 1 Inaccuracy (-0.7bb) — the preflop call · v replays it
────────────────────────────────────────────────────────────────────────────────
 COACH  Hand over — grade the read, not the result
        Your two pair lost to trips, but 3 of 4 decisions matched the chart.
        The inaccuracy: calling 10 preflop instead of raising to 25.
────────────────────────────────────────────────────────────────────────────────
  ▓ space DEAL NEXT ▓   ▓ v REVIEW ▓        session · 75% good (3/4) · net -10

 hand complete · the button moves to Tara
                                                   esc menu · ? help · v review
```

What the color pass adds: the ring in felt green; the hero plate reversed
on the accent color; the to-act plate's digit and `►` pulsing accent;
folded plates dimmed to muted with their digit replaced by `✗`; the order
digits in gold — the one saturated ink on the rim, so the eye can trace
1→6 around the shape; `● 30 to call` in the warn color when the price is
bad, felt green when the coach says it's good; reads in muted italic.

**What it optimizes for.** The owner's #1 complaint, at full strength: the
circle is real (six seats, six sides, true seating order — not a grouped
grid), and order is engraved on it rather than annotated beside it. It
also keeps every seat treatment identical — no seat is a special case, so
scanning cost is uniform.

**What it sacrifices.** Felt interior. The corner cuts spend rows 2–4 and
12–13 on shape, so the board and pot share a tighter center than A's, and
the per-seat action verb is compressed to a status word plus chips (as in
A). Two-row plates cannot show face-down card glyphs; live-vs-folded is
carried by dim/bright, the digit/✗ swap, and the chip slots. At showdown
the revealed cards take the stack row's spare width (`6♥ 2♥ tens`), which
is terse.

**Why it is not transactional.** Nothing on the table is a field. Chips
sit on the felt in front of whoever committed them; the price is a chip
stack lying in front of your cards (`● 30 to call` *on the felt*, not in a
labelled cell); the order is engraved on the rail like a casino plaque,
not listed in a legend. The only prose-with-numbers is the coach strip A
already had. Where B put a meter with axis labels, the Hexagon puts
furniture in seating order — the information is the same, the *material*
is a table.

**Build cost, honestly.** Medium — A's bill plus one line item. Everything
A needs (ring compositor in `table_layout.go`, 2-row `SeatBox`, §2 items),
plus fixed slope-1 diagonal corners (static strings, cheap) and digit
stamping fed by `StreetOrder()`. The corner rows are the only genuinely
new rendering. All table goldens and stability anchors re-record. The
one unknown that A doesn't have: whether `╱ ╲` render as clean strokes in
the owner's terminal font — a ten-minute test that should happen before a
line of the compositor is written.

**Failure mode.** The diagonals. In fonts where `╱ ╲` are thin, gappy, or
misaligned with the box-drawing weights, the shape reads as broken lines
and the whole "one drawn object" premise collapses; the ASCII `/ \`
fallback is strictly uglier than A's all-orthogonal fallback. Second
failure: if the owner's eye wants the villains' last action as a verb
("raises to 30"), the compressed status vocabulary (`▲ ● 30`) will feel
cryptic for the first sessions.

**Breakpoints.** At 104×30 the hexagon deepens (longer corner cuts, one
more felt row) and the coach moves to the existing right panel with the
action log beneath it. At 60×20 the hexagon is not drawable honestly;
compact keeps the current ledger but gains the order digits as its first
column — the one Hexagon organ that survives every size.

---

## Direction F — "The Rail Seat"

**Thesis: you don't look at a poker table from a helicopter — you look at
it from a chair. Draw the table in shallow perspective from the hero's
seat: far opponents small across the far rail, your two neighbors large at
your elbows, and your own cards not in a seat box but in your hands, below
the rail.**

This is the only direction where "who is beside me" is answered by
*physics* instead of diagram: Tara is the big block at your lower left
because she is *sitting* at your lower left; Nia is behind her on the same
side; Sam is at your right elbow. Near things are bigger and lower, far
things smaller and higher — the size gradient is the seating chart. The
hero has no seat box at all: his name is set into the near rail, and his
cards, stack, and the chips he is about to push live below it, in first
person. The price is not a number to read but an act to perform: *slide
● 30 forward*.

### F at 80×24 — preflop, facing Ivy's raise to 30 (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
                   ╭─────────┤ Cole ᵁᵀᴳ · tight ✗ ├─────────╮
 Nia ᴮᴮ · sticky  ╱                1,000                     ╲  Ivy ᴴᴶ · wild
 990 ● 10        ╱                POT 45                      ╲ 970 ▲ ● 30
                ╱                                              ╲
 Tara ˢᴮ loose ╱      ·      ·      ·      ·      ·             ╲ Sam ᶜᴼ solid
 ▓▓ ▓▓  995   ╱                                                  ╲ · ·  1,000
 ● 5 in      ╱                  to call ● 30                      ╲ ✗ folded
            ╱                                                      ╲
           ╰────────────────┤ ► YOU ᴮᵀᴺ Ⓓ · 1,000 ├─────────────────╯
      ╭──╮ ╭──╮    pair of sevens — flops a set about 1 in 8
      │7♥│ │7♣│    your stack ▓▓▓▓ 1,000 · slide ● 30 forward to call ──▸
      ╰──╯ ╰──╯    Tara sits at your left · Sam at your right
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30
        You close the action — no one left behind you. Risk 30 to win 45 is
        1.5-to-1: you need 40%, and 77 runs ~48% against Ivy's opens.[e more]
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)



 your turn · fold, call, or raise
                                                   esc menu · ? help
```

### F at 80×24 — flop, bet-sizing open (checked to hero, pot 30)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · FLOP                        BTN · ? help
                   ╭─────────┤ Cole ᵁᵀᴳ · tight ✗ ├─────────╮
 Nia ᴮᴮ · sticky  ╱                1,000                     ╲  Ivy ᴴᴶ · wild
 990 ✓ checked   ╱                POT 30                      ╲ 1,000 ✗ fold
                ╱                                              ╲
 Tara ˢᴮ loose ╱      8♥     J♣     T♠      ·      ·            ╲ Sam ᶜᴼ solid
 ▓▓ ▓▓  990   ╱                                                  ╲ · ·  1,000
 ✓ checked   ╱           sliding ● 15 toward the pot ──▸          ╲ ✗ folded
            ╱                                                      ╲
           ╰─────────────────┤ ► YOU ᴮᵀᴺ Ⓓ · 990 ├──────────────────╯
      ╭──╮ ╭──╮    pair of sevens — J and T are over your pair
      │7♥│ │7♣│    your stack ▓▓▓▓ 990 · both blinds now act before you
      ╰──╯ ╰──╯    last decision · called 10 ? Inaccuracy — raise was the play
────────────────────────────────────────────────────────────────────────────────
 COACH  Check
        Second pair under two overcards wants a cheap showdown — a bet folds
        out worse hands and gets called by better ones.              [e more]
────────────────────────────────────────────────────────────────────────────────
  ▓1 ⅓ 10▓  ▓2 ½ 15▓  ▓3 ⅔ 20▓  ▓4 POT 30▓  ▓5 ALL-IN▓             bet: 15
 min 10 ├──●────────────────────────────┤ 990     Tara calls 15 → needs 25%
 enter bet 15 · esc cancel

 your turn · sizing a bet
                                                   esc menu · ? help
```

What the color pass adds: felt green fills the trapezoid interior (the
one screen in the app with a colored surface); the near rail and `YOU`
plate in accent; elbow seats rendered a shade brighter than far seats to
reinforce near/far; the sliding chip line in gold while sizing; folded
far seats dimmed almost out.

**What it optimizes for.** Presence and adjacency. It is the most
poker-feeling frame in either pitch — the screen looks like the thing the
owner will eventually sit at, and left/right neighbors stop being an
inference entirely. The first-person hand block also gives the hero's
cards more visual mass than any seat-box treatment can.

**What it sacrifices.** The far rail. Perspective compresses exactly the
players whose actions open most pots — the UTG raiser is a small line of
text at the top while your (often folded) neighbors get two fat rows. Four
to five rows are spent on sloped empty felt. Order *sequence* is weaker
than the Hexagon's: the geometry gives you seating, and the preflop
action order must still be read from badges (the status line carries "who
is still behind you" in words).

**Why it is not transactional.** It is the anti-form. Nothing is a column;
nothing lines up to be scanned as a register. You look *across* a table at
people, and the decision is staged as a physical gesture — your stack
drawn as chips, the call as chips sliding out of it toward the pot
(`slide ● 30 forward to call ──▸`). B asked you to compare two numbers on
a meter; the Rail Seat asks you to push chips, and shows the numbers on
the coach's line where prose belongs.

**Build cost, honestly.** Medium-large — the most new rendering in this
pitch. An asymmetric trapezoid compositor (stair-stepped outside text
must track the diagonal per row); three seat scales (far text, elbow
block, first-person hero) where today there is one `SeatView`; the hero
region is a new component, not a variant. Nothing new below the TUI —
same data as A. All goldens re-record; the layout-stability matrix gains
no new dimension but its anchors all move.

**Failure mode.** If the owner plays mostly preflop decisions (he does —
that is where beginners live), the visually demoted far rail is where the
action is, and the design has spent its emphasis on the two players least
likely to matter in the average hand. It is also the direction most
sensitive to terminal font geometry: the trapezoid only reads as
perspective if the diagonals are clean; in the ASCII fallback it reads as
a funnel.

**Breakpoints.** At 104×30 the trapezoid widens and deepens, elbow seats
gain a card-back row, and the coach takes the right panel over the action
log. At 60×20 perspective is dropped entirely — compact is the current
ledger with Tara and Sam tagged `your left` / `your right`, which keeps
the design's one load-bearing idea at a size that cannot draw it.

---

## Direction G — "The Dealer's Voice"

**Thesis: the app already has a seventh character — `archetypes.go`
defines `"coach"` as a personality, the TAG baseline "advising from your
seat." Put that character *at the table*: a dealer plate on the top rail
between Cole and Ivy, exactly where a real dealer sits, and let all
coaching arrive as the dealer's table talk, spoken on the felt. The coach
strip below the table disappears, and the table takes the whole screen.**

Every other direction (and A) splits the screen into "the game" above a
rule and "the teaching" below it. The Dealer's Voice refuses the split:
the advice verb lives on the dealer's plate (`◇ COACH says CALL 30`), the
reasoning is two-to-three quoted lines written on the felt where a
dealer's voice would land, and the pot odds arrive as speech — "You're
getting 1.5-to-1" — instead of as an info cell. The felt, freed from
sharing the frame with a strip, grows to seventeen rows: the biggest,
calmest table in either pitch.

### G at 80×24 — preflop, facing Ivy's raise to 30

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
   ╭───┤ Cole ᵁᵀᴳ · tight ├───┤ ◇ COACH says CALL 30 ├───┤ Ivy ᴴᴶ · wild ├───╮
   │  1,000 ✗ folded                                       970 ▲ raised ● 30 │
   ├ Nia ᴮᴮ · sticky ──╮                                                     │
   │ 990          ● 10 │                POT 45                               │
   ├───────────────────╯                                                     │
   │                        ·    ·    ·    ·    ·         ╭─ Sam ᶜᴼ · solid ─┤
   │                                                      │ 1,000   ✗ folded │
   ├ Tara ˢᴮ · loose ──╮                                  ╰──────────────────┤
   │ 995           ● 5 │  ◇ "Thirty to you. You're getting 1.5-to-1 —        │
   ├───────────────────╯     you need 40%, and sevens run about 48%          │
   │                         against what Ivy opens. I'd call."      [e more]│
   │                       pair of sevens — flops a set about 1 in 8         │
   │                                ╭──╮ ╭──╮                                │
   │                                │7♥│ │7♣│  ◂ ● 30 to call                │
   │                                ╰──╯ ╰──╯                                │
   ╰────────────────────────┤ ► YOU ᴮᵀᴺ Ⓓ · 1,000 ├──────────────────────────╯
 order  Cole ✗ → Ivy ▲ 30 → Sam ✗ → ► YOU → Tara → Nia        (clockwise 1→6)
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)

 your turn · fold, call, or raise

                                                   esc menu · ? help
```

### G at 80×24 — the side-pot all-in hand (two all-ins in front)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
   ╭────┤ Cole ᵁᵀᴳ · tight ├────┤ ◇ COACH says FOLD ├────┤ Ivy ᴴᴶ · wild ├───╮
   │  1,000 ✗ folded                                          1,000 ✗ folded │
   ├ Nia ᴮᴮ · sticky ──╮                                                     │
   │ ALL-IN      ● 700 │   MAIN 180 · SIDE 480 Tara+Nia · SIDE 400 Nia       │
   ├───────────────────╯                                                     │
   │                        ·    ·    ·    ·    ·         ╭─ Sam ᶜᴼ · solid ─┤
   │                                                      │ 1,000   ✗ folded │
   ├ Tara ˢᴮ · loose ──╮                                  ╰──────────────────┤
   │ ALL-IN      ● 300 │  ◇ "Two all-ins. Calling risks 640 to win 1,060     │
   ├───────────────────╯     — 1.7-to-1, need 38% — but 77 is not in the     │
   │                         4-bet chart. Folding costs nothing."    [e more]│
   │                       pair of sevens — must flop a set, about 1 in 8    │
   │       your raise       ╭──╮ ╭──╮                                        │
   │       to 60 ✔ Good ─▸  │7♥│ │7♣│  ◂ ● 640 to call · your ● 60 in        │
   │                        ╰──╯ ╰──╯                                        │
   ╰────────────────────────┤ ► YOU ᴮᵀᴺ Ⓓ · 1,940 ├──────────────────────────╯
 order  Cole ✗ → Ivy ✗ → Sam ✗ → ► YOU → Tara ALL-IN → Nia ALL-IN
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 640 ▓                      to call 640 · 1.7:1 (38%)

 your turn · fold or call

                                                   esc menu · ? help
```

What the color pass adds: the dealer plate and `◇` in the coach's accent
color, distinct from all seat colors — the voice is visually a person;
speech in the normal text color (it is the primary read); the advice verb
on the dealer plate in the semantic action color (CALL blue, FOLD red);
pot trays in felt green; `✔ Good` grade stamp in the success color beside
the hero's last action.

**What it optimizes for.** Unity and warmth. One frame, one place, no
mode-switch between "playing" and "being taught" — the teaching is
diegetic, which is the most natural fit in either pitch for principle #2
(the coach *is* a table personality; here it is finally drawn as one).
The grade stamp sits inside the scene (`your raise to 60 ✔ Good ─▸`), so
even feedback is furniture. And the price gets center-stage pixels that A
never gives it — spoken, with the odds, at the eye's landing point.

**What it sacrifices.** The chrome grammar. This is the one direction
that departs from the shared header/rule/content/status grammar: there is
no coach strip, and content owns rows 2–18. The justification: the strip
exists to separate app-voice from game-state, and this direction's whole
thesis is that the coach is game-state; keeping both would say it twice.
It also caps reasoning at ~3 lines × 50 columns — less than the strip's
budget — so `[e more]` works harder here than anywhere.

**Why it is not transactional.** The numbers are dialogue. A meter tells
you `need 40 / have 48`; a dealer says "you're getting 1.5-to-1 — you
need 40%, and sevens run about 48%." Same facts (drawn from the same
`Rationale`, per principle #3), but delivered as a person speaking inside
a scene, which is the exact opposite register of a trade ticket. There is
no labelled field anywhere on the felt — pots are trays, prices are
speech, chips are chips.

**Build cost, honestly.** Medium. The ring is A's compositor (orthogonal
racetrack, no diagonals). New: the dealer plate (static), the speech
region — a fixed 3-row window on the felt with the same wrap+`[e more]`
logic `coach_panel.go` already implements, re-parented; and quote-style
rendering of the existing coach output (a formatting pass, not new
content — the truthfulness test still holds verbatim). The coach strip
code stays for other screens. Goldens re-record; the stability matrix
gains one case (speech at max window).

**Failure mode.** Chatter. At Fast speed a talking dealer risks becoming
the noise you learn to ignore — and an ignored coach is a dead learning
tool. Text lying on the felt also competes with the board for the same
central fixation point; if the speech window regularly clips, the player
is either opening the `e` overlay constantly (friction) or missing the
arithmetic (worse). This direction should not ship without the owner
reading three real hands of dealer talk and saying he'd want thirty more.

**Breakpoints.** At 104×30 the felt widens, speech gets four longer lines
and effectively never clips, and the action log returns on the right
panel. At 60×20 the dealer collapses into the compact ledger's coach line
prefixed `◇` — the voice survives as one spoken sentence even where the
table cannot be drawn.

---

## Direction H — "The Study Table"

**Thesis: a poker book draws the table small, complete, and surrounded by
its teaching — because a diagram you can take in at one glance, annotated
in the margins, is how positions and prices actually get learned. Draw a
small, faithful ring; spend the reclaimed columns on margin notes and the
reclaimed rows on a coach that never truncates.**

Every other direction makes the table bigger. This one makes it *smaller*
— a 54-column oval that fits in one foveal glance, every seat on the rim
in true clockwise order, hero embedded in the bottom edge — and puts the
teaching where a book puts it: your hand and its future in the left
margin, the price drawn as chip piles in the right margin, the street's
order under the diagram, and below the rule the only coach in either
pitch with a five-row budget: **no `[e more]`, ever, at 80×24.** The
margin chips carry the anti-transactional trick: the pot is `●●● 45` and
the call is `●● 30` — the 2:3 ratio is *drawn* (one ● per 15 chips), so
"call 2 to win 3, need 40%" is a picture before it is arithmetic.

### H at 80×24 — preflop, facing Ivy's raise to 30 (coach Full)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · PREFLOP                     BTN · ? help
             ╭───┤ Nia ᴮᴮ 990 ├──┤ Cole ᵁᵀᴳ ✗ ├──┤ Ivy ᴴᴶ 970 ├───╮
             │   sticky · ● 10      tight        wild · ▲ ● 30    │
 your 7♥ 7♣  ├╴Tara ˢᴮ 995                             Sam ᶜᴼ ✗  ╶┤  the pot
 a pair that │   loose · ● 5        POT 45           solid        │  ●●● 45
 flops a set │               ·    ·    ·    ·    ·                │  to call
 about 1 in  │                                                    │  ●● 30
 8 — else    │                     ╭──╮ ╭──╮                      │  call 2 to
 often ends  │                     │7♥│ │7♣│  ◂ ● 30              │  win 3 —
 second best ╰───────────────┤ ► YOU ᴮᵀᴺ Ⓓ 1,000 ├────────────────╯  need 40%
 order   Cole ✗ → Ivy ▲ 30 → Sam ✗ → ► YOU → Tara → Nia    you have ~48% — good
────────────────────────────────────────────────────────────────────────────────
 COACH  Call 30
        77 is in the call-vs-open chart. The price is 1.5-to-1 — risk 30 to
        win 45, so you need to win 40% of the time. Against the ≈18% of
        hands Ivy opens from the hijack, sevens win about 48%. You close
        the action in position — set-mine cheap, stack her when a seven hits.
────────────────────────────────────────────────────────────────────────────────
  ▓ f FOLD ▓   ▓ c CALL 30 ▓   ▓ r RAISE… ▓        to call 30 · 1.5:1 (40%)



 your turn · fold, call, or raise
                                                   esc menu · ? help
```

### H at 80×24 — flop, bet-sizing open (the margins re-aim at Tara)

```
 Hand #1 · 6-max NLHE · blinds 5/10 · FLOP                        BTN · ? help
             ╭───┤ Nia ᴮᴮ 990 ├───┤ Cole ᵁᵀᴳ ✗ ├───┤ Ivy ᴴᴶ ✗ ├───╮
             │   sticky ✓ chk       tight         wild            │
 your 7♥ 7♣  ├╴Tara ˢᴮ 990                             Sam ᶜᴼ ✗  ╶┤  pot ●● 30
 second pair │   loose ✓ chk        POT 30           solid        │  a ● 15 bet
 now — J and │            8♥    J♣    T♠     ·      ·             │  offers Tara
 T are over  │                                                    │  3-to-1 —
 you, with 2 │                     ╭──╮ ╭──╮                      │  she needs
 streets of  │                     │7♥│ │7♣│  ● 15 ▸              │  25% to see
 danger left ╰────────────────┤ ► YOU ᴮᵀᴺ Ⓓ 990 ├─────────────────╯  the turn
 order   Tara ✓ → Nia ✓ → ► YOU            (postflop the blinds act first)
────────────────────────────────────────────────────────────────────────────────
 COACH  Check
        Second pair on a wet board wants a cheap showdown. A bet folds out
        the worse hands and gets called by the better ones — and both
        blinds get to react to whatever you do here. Check behind, keep
        the pot small, and take a free look at the turn in position.
────────────────────────────────────────────────────────────────────────────────
  ▓1 ⅓ 10▓  ▓2 ½ 15▓  ▓3 ⅔ 20▓  ▓4 POT 30▓  ▓5 ALL-IN▓             bet: 15
 min 10 ├──●────────────────────────────┤ 990     Tara calls 15 → needs 25%
 enter bet 15 · esc cancel

 your turn · sizing a bet · your preflop call graded ? Inaccuracy (-0.7bb)
                                                   esc menu · ? help
```

Note what the sizing state does: the right margin flips from *your* price
to *the price you are offering* — "a ● 15 bet offers Tara 3-to-1 — she
needs 25%" — which is the single most advanced idea the app teaches,
placed where the eye already learned to look for "is the price good."

What the color pass adds: the small ring in felt green with the hero
plate reversed accent; margin notes in the muted teaching color so the
table stays the brightest object; the chip-pile glyphs in gold (pot) and
accent (your side) so the ratio comparison is also a color pairing;
grades in their existing success/warn colors.

**What it optimizes for.** Glance economy and the coach. The table is
small enough that order, stacks, chips, and board arrive in a single
fixation; the four glance items each own a stable home (hand top-left,
price top-right, coach below, reads on plates and pegs); and the full
reasoning is always present — the only direction in either pitch that
never clips the coach at the target size.

**What it sacrifices.** Grandeur. The table is a diagram, not a room —
seats are one-line plates and pegs, no card backs, no per-seat action
verbs (status folds into `✓`/`✗`/chips). Showdown theater is modest. And
it is the least "not visually plain" of the four: its beauty is
typographic, not architectural, and if the owner wanted spectacle this
will read as restraint.

**Why it is not transactional.** This is the direction closest to the
line, so the defense is explicit. B failed because it presented *fields*:
`PRICE`, `EDGE`, axis-labelled meters, players as rows. The Study Table
presents a *drawing with handwriting around it*: the margin notes are
prose fragments ("call 2 to win 3 — need 40%"), never `label: value`
pairs; the price is physical chip piles whose heights are the odds; the
people stay on a drawn ring in seating order, never in a register. The
test: cover the margins, and a complete drawn table remains; cover B's
panels and nothing remained but a header. The table is the subject and
the teaching annotates it — B inverted that.

**Build cost, honestly.** Medium-small — the cheapest direction in this
pitch. The small ring is a simpler compositor than A's (fewer junction
widths, one-row plates, two pegs); margin notes are two fixed 11-column
regions fed by existing `Rationale` facts plus §2.4's potential line; the
chip-pile renderer is a pure function (chips = ceil(amount/bb·k)) that is
trivially golden-tested; the coach panel is the existing component with a
bigger budget. Goldens re-record, as everywhere.

**Failure mode.** Miniaturization. If six plates, chips, and a board in a
54×9 oval feel cramped rather than neat — especially with 4-way action
and revealed cards at showdown — the diagram stops being legible exactly
when the hand gets interesting. And the margins must be policed hard:
the day a margin note becomes `EQUITY: 48%`, this design has quietly
become B with a table-shaped ornament.

**Breakpoints.** At 104×30 the table stays deliberately small and the
margins widen into real sidebars (left gains the outs breakdown, right
gains the villain dossier the archetypes deserve), with the coach still
full-width below. At 60×20 the margins are the casualty — their content
folds into the coach line and the table becomes the current compact
ledger with the order line kept.

---

## Side-by-side against A — the owner's five needs

✦✦✦ best. A's column restates the first pitch's honest scores.

| Owner's need                        | A Drawn Table | E Hexagon | F Rail Seat | G Dealer's Voice | H Study Table |
|-------------------------------------|:---------:|:---------:|:-----------:|:---------:|:-----------:|
| 1 Order & adjacency from geometry   | ✦✦✦ | ✦✦✦ (true circle, order engraved) | ✦✦✦ adjacency / ✦✦ order | ✦✦✦ (ring + order rail) | ✦✦ (small ring + order line) |
| 2 Eye knows where to land           | ✦✦ | ✦✦ | ✦✦✦ (your hands, then the felt) | ✦✦ (speech vs board compete) | ✦✦✦ (one small bright object) |
| 3 Not visually plain                | ✦✦✦ | ✦✦✦ | ✦✦✦ (the boldest drawing) | ✦✦✦ | ✦✦ |
| 4 Four glance items, ranked         | ✦✦ (price weakest) | ✦✦ (price now chips, still small) | ✦✦ (far reads weak) | ✦✦✦ (price center-stage) | ✦✦✦ (each item has a home) |
| 5 Chrome harmony                    | ✦✦ | ✦✦ | ✦ (asymmetric, text outside rim) | ✦ (deliberately breaks the strip) | ✦✦✦ (plus the only uncut coach) |
| Build cost (✦✦✦ = cheapest)         | ✦ | ✦ | ✦ | ✦✦ | ✦✦✦ |

## Recommendation

**None of the four sweeps A off the table — but the Hexagon beats A
narrowly on the need that started all this, and two organs from the others
should be grafted into whichever ring wins.**

1. **E vs A is the real decision, and it is a font test away.** They cost
   the same to build and share the same skeleton; the Hexagon's true
   circle with engraved, re-stamping order digits is a straight upgrade on
   need #1 (A groups seats 3-top/2-mid and explains order in a rail; E
   *is* the order). The only thing that should stop it is the terminal:
   spend ten minutes rendering `╱ ╲` corner cuts in the owner's actual
   font and theme before committing. If the diagonals disappoint, build A
   — and stamp E's order digits onto A's seat plates anyway, because the
   digits are geometry-independent and directly answer his #1 complaint.
2. **Graft H's chip-pile price into the winner.** `●●● 45 / ●● 30 — call
   2 to win 3, need 40%` replaces the info-cell price and fixes the
   acknowledged weakest glance item of A (and E) without a meter and
   without a new row: it can live on the felt or at the action bar's right
   end. It is a pure function and the cheapest single improvement in
   either pitch.
3. **Hold G as the voice, not the layout.** The dealer-as-coach is the
   most charming idea here and the truest to the codebase (the `"coach"`
   archetype already exists), but it bets the screen on speech-on-felt
   working at speed. Ship the winner first; then trial G's register
   cheaply by rewording the existing coach strip into first-person dealer
   voice ("You're getting 1.5-to-1…") for a week. If the owner loves the
   voice, the full felt-speech layout is a known quantity to build; if it
   grates, we learned it for free.
4. **F is the romance option — file it under "someday."** It is the best
   answer to "make it feel like poker" and the worst answer to "teach me
   preflop," which is where the owner currently lives. Its one portable
   idea — tagging Tara and Sam `your left` / `your right` in compact —
   should ship regardless.

Sequenced: font-test E's diagonals → build E (or A + E's digits) with H's
chip-pile price → re-voice the coach strip as a dealer for a week (gates
G) → revisit F when live-play transfer becomes a stated goal.

## The one question

**When the coach speaks, who do you want to be hearing — a person at the
table, or an instructor beside it?** Concretely: should advice arrive in
character, as table talk from a dealer who is part of the scene ("You're
getting 1.5-to-1 — I'd call"), or as the app's own voice in its own strip,
outside the game world? Everything else in this pitch is layout; this is
identity, it decides G outright, it shapes how E/A/H phrase every line the
coach will ever say, and it cannot be settled by mockups — only by which
sentence the owner wants to read two hundred times a session.
