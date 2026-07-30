# TUI / UX Design — texas-holdem

Status: design, ready to implement.
Companions: `docs/BRIEF.md` (product), engine/AI design docs (separate).
Reference implementation: `../euchre` — this project should feel like a sibling of it.
Terminal baseline: **80×24 full layout**, compact fallback down to **60×20**, hard floor below that.

---

## 1. Screen map & navigation

### 1.1 Screens

```
                          ┌──────────────┐
                          │   MainMenu   │
                          └──────┬───────┘
          ┌────────────┬─────────┼──────────┬─────────────┬──────────┐
          ▼            ▼         ▼          ▼             ▼          ▼
     ┌─────────┐ ┌─────────┐ ┌────────┐ ┌─────────┐ ┌───────────┐ ┌────────┐
     │GameSetup│ │ Lessons │ │Trainer │ │QuickRef │ │ Settings  │ │  Quit  │
     └────┬────┘ └─────────┘ └────────┘ └─────────┘ └───────────┘ └────────┘
          ▼
     ┌─────────┐   review last hand    ┌────────────┐
     │  Table  │ ◄───────────────────► │ HandReview │
     └─────────┘   back to table       └────────────┘
```

| Screen constant        | Model         | File                | Purpose |
|------------------------|---------------|---------------------|---------|
| `ScreenMainMenu`       | `MainMenu`    | `main_menu.go`      | Entry menu (Play / Lessons / Trainer / Quick Reference / Settings / Quit) |
| `ScreenGameSetup`      | `GameSetup`   | `game_setup.go`     | Blinds, starting stack (bb), opponent difficulty, coach verbosity, speed |
| `ScreenTable`          | `Table`       | `table.go`          | The cash-game session — the centerpiece |
| `ScreenHandReview`     | `HandReview`  | `hand_review.go`    | Post-hand replay with AI cards revealed + EV annotations |
| `ScreenLessons`        | `Lessons`     | `lessons.go`        | Guided curriculum (hand rankings → position → ranges → pot odds → betting) |
| `ScreenTrainer`        | `Trainer`     | `trainer.go`        | Standalone drills: rank the hand / count outs / estimate equity |
| `ScreenQuickReference` | `QuickReference` | `quick_reference.go` | Tabbed cheat sheet (rankings, positions, odds table, glossary) |
| `ScreenSettings`       | `Settings`    | `settings.go`       | Theme, deck coloring, speed, coach verbosity, ASCII fallback |

### 1.2 Routing — euchre's `NavigateMsg`, with two deliberate changes

We keep euchre's pattern verbatim: a root `App` model holding
`map[Screen]tea.Model`, a `NavigateMsg{Screen, Data}` routed in `App.Update`,
`Navigate(screen)` / `NavigateWithData(screen, data)` command constructors, a
`QuitMsg`, and window size forwarded to the newly-active screen before its
`Init()` runs. It works, it's tested, and a sibling project should not invent a
second navigation idiom.

Two changes, each with a concrete reason:

1. **Session-preserving navigation.** Euchre's `navigate()` *recreates* the
   target model on every navigation. That's fine there — you never leave a game
   in progress. Here the core loop is *play hand → jump to HandReview → come
   back to the same table with the same stacks*. So `App.navigate` recreates a
   model only when (a) it doesn't exist yet, or (b) the message carries fresh
   construction data (e.g. `TableConfig` from GameSetup). `NavigateMsg{Screen:
   ScreenTable, Data: nil}` returns to the live session. Leaving the table for
   the main menu sends `EndSessionMsg`, which drops the cached `Table` model.

2. **Typed navigation payloads.** `Data interface{}` stays (it's the euchre
   shape), but every payload is a named struct — `TableConfig`,
   `ReviewRequest{Hand *engine.HandHistory, ReturnTo Screen}`,
   `LessonRequest{LessonID string}` — never an anonymous map or primitive.
   `HandReview` needs `ReturnTo` because it is reachable from both `Table`
   (review last hand) and `Lessons` (worked examples).

No screen stack. The graph above is shallow (max depth 2 from the menu) and an
explicit `ReturnTo` on the one screen with two parents is simpler than a
general stack — same call euchre made.

```go
// app.go
type Screen int

const (
    ScreenMainMenu Screen = iota
    ScreenGameSetup
    ScreenTable
    ScreenHandReview
    ScreenLessons
    ScreenTrainer
    ScreenQuickReference
    ScreenSettings
)

type App struct {
    currentScreen Screen
    screenModels  map[Screen]tea.Model
    settings      *usersettings.Settings // persisted prefs (speed, coach, theme)
    width, height int
    quitting      bool
}

type NavigateMsg struct {
    Screen Screen
    Data   interface{} // always one of the named payload structs below, or nil
}

// Payloads
type TableConfig struct {
    SmallBlind, BigBlind int
    StartingStackBB      int
    Difficulty           ai.Difficulty
    CoachMode            CoachMode // see §5
    Speed                Speed     // see §6
}

type ReviewRequest struct {
    Hand     *engine.HandHistory
    ReturnTo Screen // ScreenTable or ScreenLessons
}

func Navigate(s Screen) tea.Cmd
func NavigateWithData(s Screen, data interface{}) tea.Cmd
func Quit() tea.Cmd

type EndSessionMsg struct{} // drop the cached Table model (leaving the game)
```

---

## 2. The poker table layout

### 2.1 Space budget — the hard constraint

Euchre's play screen was designed against 40-row test terminals and spends 4
cells on a decorative screen border. We must fit **80×24**. Decisions that
follow directly from that:

- **No outer screen border on the Table screen.** Menus/lessons keep the
  euchre-style rounded frame; the table cannot afford 2 rows + 2 cols of
  chrome. The header and action bar visually frame the screen instead.
- **Mini cards (4×3), not euchre's 7×5 cards.** A 5-card board at 7×5 is
  35×5; at 4×3 it's 21×3 (with 1-col gaps: 24×3). Hole cards likewise.
- **Coach is a 2-row strip at 80 cols**, a full side panel only at ≥104 cols
  (§5). A side panel at 80 cols would starve the seats of width.
- **Every region has a fixed row budget, occupied even when empty** — the
  euchre layout-stability invariant. Seat action lines, the coach strip, and
  the action bar never appear/disappear; they render blank.

Row budget at 80×24 (rows 1–24):

| Rows  | Region |
|-------|--------|
| 1     | Header: hand #, game, blinds, street, help hint |
| 2     | blank |
| 3–5   | Top seats ×3 (name/stack · cards+bet · last action) |
| 6     | blank |
| 7–9   | Mid-left seat │ board (mini cards) │ mid-right seat |
| 10    | Pot line (main + side pots) |
| 11    | blank |
| 12–14 | Hero row: name/stack/badges · hole cards · hand-strength label |
| 15    | Hero last-action / grade line (reserved) |
| 16    | rule ─────────── |
| 17–18 | Coach strip (2 lines, reserved even when coach is off → shows street summary) |
| 19    | rule ─────────── |
| 20–21 | Action bar (2 lines: actions/sizing · live math) |
| 22    | blank |
| 23    | Status line (whose turn / temp messages) |
| 24    | Footer: `esc menu · ? help · v review` (right-aligned) |

### 2.2 Seat geometry

Fixed screen positions; **hero is always bottom-center**. Position badges
(BTN/SB/BB/UTG/HJ/CO) rotate around the fixed seats each hand — this is itself
a lesson: *seats don't move, the button does*.

```
        seat 2 (TL)      seat 3 (TC)      seat 4 (TR)
 seat 1 (ML)            [ board + pot ]           seat 5 (MR)
                     seat 0 (BC) = HERO
```

Seat index = clockwise from hero, matching engine action order: after the hero
acts, action flows to seat 1 (mid-left), up the left side, across the top,
down the right — a consistent visual sweep the player can learn to read.

**Seat block** (3 rows, 20 cols max, left-aligned within its slot):

```
 Nia ᴴᴶ 830          ← name, position badge, stack (Ⓓ dealer disc after badge on BTN)
 ▓▓ ▓▓  ● 30         ← hole cards (▓▓ face-down, A♠ K♦ revealed, ·· mucked/folded)
 raises to 30        ← last action this street (reserved line, may be blank)
```

Glyph vocabulary (each has an ASCII fallback, §3.4):

| Glyph | Meaning |
|-------|---------|
| `Ⓓ`   | dealer button (also the BTN badge is gold) |
| `● 30`| chips in front — current-street bet, sits *between seat and board* conceptually, rendered inline after cards |
| `►`   | seat to act (bold + theme accent + pulse, prefixed to name) |
| `▓▓ ▓▓` | face-down hole cards |
| `· ·` | folded — cards gone to the muck (block also dims) |
| `ALL-IN` | replaces stack amount, gold |

### 2.3 Mockup A — preflop, 80×24, full layout

Hero in the BB with A♥K♥; UTG folded, HJ raised to 30, CO/BTN/SB folded.
(Coach at `Full` verbosity.)

```
Hand #17 · 6-max NLHE · blinds 5/10 · PREFLOP                      BB · ? help

     Nia ᴴᴶ 830              Cole ᶜᴼ 615              Ivy ᴮᵀᴺ Ⓓ 990
     ▓▓ ▓▓  ● 30             · ·                      · ·
     raises to 30            folds                    folds

 Tara ᵁᵀᴳ 1,020                                               Sam ˢᴮ 2,395
 · ·                        ·  ·  ·  ·  ·                     · ·  ● 5
 folds                                                        folds

                                  POT 45

                            ╭──╮ ╭──╮
   ► YOU ᴮᴮ 990  ● 10       │A♥│ │K♥│        ace-king suited
                            ╰──╯ ╰──╯

────────────────────────────────────────────────────────────────────────────────
 💡 COACH  Nia opened to 3bb and everyone folded to you. A♥K♥ is a premium
    hand: 3-bet to ~90 to build a pot while you likely have the best hand.
────────────────────────────────────────────────────────────────────────────────
  f fold      c call 20      r raise…          to call 20 · pot odds 2.2:1 (31%)

 Your turn — fold, call, or raise                    esc menu · ? help · v review
```

Notes: the empty board renders as five dim placeholder pips `· · · · ·` so the
board region (and everything anchored around it) never moves when the flop
lands. Folded seats keep all three lines (dimmed) — stacks stay visible because
*reading stacks at all times* is a habit worth teaching.

### 2.4 Mockup B — flop, bet-sizing mode open

Hero 3-bet preflop, Nia called. Flop K♠ 7♦ 2♣, Nia checked, hero is sizing a
bet — the action bar has switched into **sizing mode** (§4.3); the coach strip
is temporarily replaced by the sizing readout only if coach is `Off`, otherwise
the coach compresses to one line and the slider takes the second:

```
Hand #17 · 6-max NLHE · blinds 5/10 · FLOP                         BB · ? help

     Nia ᴴᴶ 740              Cole ᶜᴼ 615              Ivy ᴮᵀᴺ Ⓓ 990
     ▓▓ ▓▓                   · ·                      · ·
     checks

 Tara ᵁᵀᴳ 1,020         ╭──╮ ╭──╮ ╭──╮  ·  ·                  Sam ˢᴮ 2,395
 · ·                    │K♠│ │7♦│ │2♣│                        · ·
 folds                  ╰──╯ ╰──╯ ╰──╯

                                  POT 185

                            ╭──╮ ╭──╮
   ► YOU ᴮᴮ 900             │A♥│ │K♥│        top pair, top kicker
                            ╰──╯ ╰──╯

────────────────────────────────────────────────────────────────────────────────
 💡 COACH  Strong hand, dry board — bet for value; ½ to ⅔ pot is standard here.
 BET  min 10 ├────────────●──────────────────────┤ all-in 900        bet: 120
────────────────────────────────────────────────────────────────────────────────
  1 ⅓ 62   2 ½ 93   3 ⅔ 123   4 pot 185   5 all-in   ←/→ nudge   0-9 type amount

 enter bet 120 · esc cancel            Nia calls 120 → she gets 2.5:1 (needs 28%)
```

### 2.5 Mockup C — turn, two all-ins, side pot

Sam (short stack) is all-in for 285, Nia all-in for 640, hero has the nut
flush and is deciding. The pot line switches to the multi-pot form and names
who is eligible for the side pot — side pots are exactly the kind of rule
beginners find opaque, so the display does the bookkeeping in the open:

```
Hand #23 · 6-max NLHE · blinds 5/10 · TURN                         BB · ? help

     Nia ᴴᴶ ALL-IN           Cole ᶜᴼ 615              Ivy ᴮᵀᴺ Ⓓ 990
     ▓▓ ▓▓  ● 640            · ·                      · ·
     all-in 640              folds                    folds

 Tara ᵁᵀᴳ 1,020         ╭──╮ ╭──╮ ╭──╮ ╭──╮  ·                Sam ˢᴮ ALL-IN
 · ·                    │Q♠│ │J♠│ │4♦│ │9♠│                   ▓▓ ▓▓  ● 285
 folds                  ╰──╯ ╰──╯ ╰──╯ ╰──╯                   all-in 285

                     MAIN 900   ·   SIDE 710 (Nia · you)

                            ╭──╮ ╭──╮
   ► YOU ᴮᴮ 1,480           │A♠│ │T♠│        flush, ace high (nuts)
                            ╰──╯ ╰──╯

────────────────────────────────────────────────────────────────────────────────
 💡 COACH  Sam is all-in for less, so his 285 caps the MAIN pot. Calling Nia's
    640 also contests the SIDE pot against her alone. You hold the nuts: call.
────────────────────────────────────────────────────────────────────────────────
  f fold      c call 640                       to call 640 into 1,610 · 2.5:1

 Your turn — Sam and Nia are all-in                  esc menu · ? help · v review
```

At showdown the same frame is reused: face-down `▓▓ ▓▓` flip to real cards in
the seat blocks, the winning 5-card hand is highlighted (winning cards get the
gold double border, exactly euchre's `CardStyleTrickWinner` treatment), and the
pot line animates per-pot awards: `MAIN 900 → Sam (two pair)` then
`SIDE 710 → you (nut flush)`.

### 2.6 Wide layout (≥104 cols): coach becomes a side panel

At `width ≥ 104` the 2-row coach strip is replaced by a bordered right-hand
panel (28 cols) and the table gets the remaining ≥76 — same breakpoint idea as
euchre's `fullLayoutMinWidth` side HUD. The strip rows are then reclaimed by
the hand-history ticker (last 3 actions, scrolling).

```
Hand #17 · 6-max NLHE · 5/10 · FLOP           BB · ? help  ╭─ 💡 COACH ─────────────╮
                                                           │ Bet for value.         │
     Nia ᴴᴶ 740          Cole ᶜᴼ 615        Ivy ᴮᵀᴺ Ⓓ 990  │                        │
     ▓▓ ▓▓               · ·                · ·            │ You hold top pair, top │
     checks                                                │ kicker on a dry board. │
                                                           │ Few draws can call —   │
 Tara ᵁᵀᴳ 1,020     ╭──╮ ╭──╮ ╭──╮  ·  ·      Sam ˢᴮ 2,395 │ size ½–⅔ pot (93–123). │
 · ·                │K♠│ │7♦│ │2♣│            · ·          │                        │
 folds              ╰──╯ ╰──╯ ╰──╯            folds        │ Outs if called: you're │
                                                           │ ahead of Kx; AA/77/22  │
                          POT 185                          │ beat you (rare).       │
                                                           │                        │
                    ╭──╮ ╭──╮                              │ Last action: PREFLOP   │
   ► YOU ᴮᴮ 900     │A♥│ │K♥│    top pair, top kicker      │ 3-bet to 90 — ✓ A      │
                    ╰──╯ ╰──╯                              │ matches coach line.    │
                                                           ╰────────────────────────╯
──────────────────────────────────────────────────────────
  x check   b bet…                        pot 185 · Nia 740 behind
```

### 2.7 Compact fallback (<80 cols) and minimum size

Below 80 cols the oval dies gracefully into a **ledger layout**: one line per
seat, board inline. This is not a degraded afterthought — a seat-per-line view
is genuinely readable and keeps every number on screen. Breakpoints:

- `width ≥ 104 && height ≥ 28` → wide layout (coach side panel)
- `width ≥ 80 && height ≥ 24` → full oval layout (coach strip)
- `width ≥ 60 && height ≥ 20` → compact ledger
- below **60×20** → `renderTooSmall` screen (euchre pattern: centered
  "Terminal too small — need at least 60×20, have 52×18").

Compact mockup at 60×20:

```
#17 · 5/10 · FLOP                            POT 185
────────────────────────────────────────────────────
  Tara UTG  1,020  · ·    fold
  Nia  HJ     740  ▓▓▓▓   check
  Cole CO     615  · ·    fold
  Ivy  BTNⒹ   990  · ·    fold
  Sam  SB   2,395  · ·    fold
► YOU  BB     900  A♥ K♥
────────────────────────────────────────────────────
 Board  K♠ 7♦ 2♣ · ·        top pair, top kicker
────────────────────────────────────────────────────
 💡 Bet for value — ½ to ⅔ pot is standard here.
────────────────────────────────────────────────────
 x check  b bet…            pot 185 · Nia 740 behind
 Your turn                        esc menu · ? help
```

The compact layout uses inline 2-char cards throughout (§3.2) and drops the
seat action line into the same row as the seat. All state survives a live
resize in either direction — layout is chosen per-`View()` from
`width`/`height`, never stored.

---

## 3. Card rendering

### 3.1 Mini card (the default on the table)

4 wide × 3 tall, rounded corners to distinguish at a glance from euchre's
square 7×5 cards and from panel borders:

```
face up      face down    board placeholder    muck/folded
╭──╮         ╭──╮
│A♠│         │▓▓│              ·                  · ·
╰──╯         ╰──╯
```

Ten renders as `T` (`│T♠│`) — single-char ranks keep every card 4 cells wide
and teach the standard poker notation used by every book and solver.

Card interiors follow euchre's philosophy: **fixed colors, not adaptive** — a
card is white with colored ink no matter the terminal background. Border uses
the muted adaptive color; face-down fill uses the `ColPip` card-back blue.

### 3.2 Inline card (compact layout, history ticker, review, coach text)

Two styled characters: `A♠` — rank in the suit's ink color, bold. Everywhere
prose mentions a card (coach reasoning, hand review, lesson text) it uses the
inline renderer, mirroring euchre's `colorizeCards` so cards look identical in
text and on the table.

### 3.3 Four-color deck (default ON — a learning aid)

Distinguishing suits *fast* is a real beginner skill (flush blindness is the
classic beginner leak). Default to the standard four-color convention:

| Suit | Glyph | Dark bg | Light bg | ASCII |
|------|-------|---------|----------|-------|
| Spades   | ♠ | `#ECF0F1` (near-white) | `#2C3E50` (near-black) | `s` |
| Hearts   | ♥ | `#E74C3C` | `#C0392B` | `h` |
| Diamonds | ♦ | `#60A5FA` (blue) | `#2178C4` | `d` |
| Clubs    | ♣ | `#2ECC71` (green) | `#1E8449` | `c` |

Settings toggle: `Deck colors: Four-color / Two-color` (two-color = red/black,
for players who want to train for live cards). Suit color is resolved through
one function — `theme.SuitStyle(suit)` — so the toggle is a single switch, and
spades/clubs remain distinguishable by glyph alone (colorblind-safe secondary
cue, same principle as euchre's `▾` legal-move marker).

### 3.4 Degraded terminals

Two independent fallbacks, both auto-detected at startup (termenv color
profile; Unicode probe via `LANG`/`LC_ALL` containing `UTF-8`) and both
overridable in Settings:

- **No/low color** (`Ascii`/`ANSI` profile): suit glyphs stay; hearts/diamonds
  get terminal-red if 16 colors exist; at true no-color, suits rely on glyph
  identity and face-down cards use `▒▒` vs the board's `··`.
- **No Unicode**: cards become `[As]` `[Kh]` bracket pairs, face-down `[##]`,
  board placeholders `[--]`, box-drawing rules become `-`/`|`/`+`, badges
  become plain text (`(D)`, `->` for turn marker, `*` for chips-in-front).
  The layout grid is unchanged — every Unicode glyph has a same-width ASCII
  substitute registered in `theme.Glyphs` (a struct swapped wholesale, never
  per-callsite conditionals).

### 3.5 Board vs. hole vs. muck — one vocabulary

| Context | Rendering |
|---------|-----------|
| Community board | Mini cards, dealt slots filled left→right; undealt slots are dim `·` pips so the region never resizes |
| Hero hole cards | Mini cards, always face up, always bottom-center |
| Villain hole cards (live) | `▓▓ ▓▓` face-down pair |
| Villain hole cards (showdown / review) | Inline or mini face-up, per layout |
| Folded | `· ·` and the whole seat block dims — folded hands are *gone*, not hidden, and the visual difference between "face-down live" and "mucked" matters when counting who's left |
| Winning 5 cards at showdown | Gold double border (euchre's trick-winner treatment) on the cards that play, including board cards — teaches "your hand is the best 5 of 7" |

---

## 4. The action bar

### 4.1 States

The action bar is a two-row region with three states, driven strictly by the
engine's `LegalActions(seat)` — the bar never computes legality itself:

```
   ┌────────────┐  r/b pressed   ┌──────────────┐  enter/esc  ┌────────────┐
   │  choosing  │ ─────────────► │    sizing    │ ──────────► │  submitted │
   └────────────┘                └──────────────┘             └────────────┘
        ▲  waiting (not hero's turn): bar renders dimmed legal keys + live math
```

**Choosing** renders one keycap chip per legal action with its cost, euchre
keycap style (`keyCap`):

```
  f fold      c call 20      r raise…          to call 20 · pot odds 2.2:1 (31%)
  x check     b bet…                           pot 185 · Nia 740 behind
```

Only legal actions render (no dimmed-out fold when check is free — absence
teaches legality better than presence-but-disabled; the keybind-legend test
enforces exact correspondence, §8.2).

### 4.2 Keybindings

| Key | Context | Action |
|-----|---------|--------|
| `f` | choosing | Fold |
| `x` | choosing | Check (mnemonic: "check ✗"; `c` is call) |
| `c` | choosing | Call (amount always shown) |
| `b` | choosing, no bet to face | Bet → sizing mode |
| `r` | choosing, facing a bet | Raise → sizing mode |
| `a` | choosing | All-in (shortcut, still confirms: `enter` on pre-filled all-in sizing) |
| `1`–`5` | sizing | Presets: ⅓ pot, ½ pot, ⅔ pot, pot, all-in |
| `←`/`→` (`h`/`l`) | sizing | Nudge by one big blind (`shift` = 5bb) |
| `0`–`9` | sizing | Begin/continue typed amount (typed digits replace slider value) |
| `backspace` | sizing | Delete typed digit |
| `enter` | sizing | Confirm bet/raise |
| `esc` | sizing | Cancel back to choosing |
| `tab` | any | Cycle coach verbosity Off → Mistakes → Full (§5.4) |
| `v` | between hands | Review last hand (`HandReview`) |
| `space`/`enter` | pacing pause | Continue (§6.2) |
| `+`/`-` | table | Speed up / slow down game speed |
| `?` | any | Help overlay (in-place, any key closes — euchre pattern) |
| `esc` | table (choosing) | Menu (confirm dialog: session continues in background? No — confirm leave) |

Vim-style `h/l` aliases exist everywhere arrow keys do (euchre convention).

### 4.3 Sizing mode — the hard interaction

Requirements: fast for the common case (presets), precise when wanted (typed),
always legal (clamped), always educational (live math). Rendering (row 1 of
the bar; row 2 shows presets + hints):

```
 BET  min 10 ├────────────●──────────────────────┤ all-in 900        bet: 120
  1 ⅓ 62   2 ½ 93   3 ⅔ 123   4 pot 185   5 all-in   ←/→ nudge   0-9 type amount
```

Design decisions:

- **Slider + presets + typing are one state, not three.** There is a single
  `amount int`; presets set it, arrows nudge it, digits rewrite it. The slider
  is a *readout* of `amount` within `[min, max]`, not a separate widget with
  focus. No focus juggling in a 2-row bar.
- **Clamping is engine-driven.** `LegalActions` returns
  `BetRange{Min, Max int}` (min-raise rules, all-in-for-less, etc. live in the
  engine). The bar clamps on confirm *and* on every nudge; a typed amount
  outside the range renders the amount in the warning color with the clamp it
  will receive: `bet: 7 → min 10`. Confirming applies the clamp — never an
  error dialog for something the engine can resolve.
- **Preset labels show the resolved amount, not just the fraction** (`½ 93`),
  because translating "half pot" to chips is precisely the arithmetic a
  beginner needs to internalize. Presets facing a bet are computed on the
  *pot after your call* — the coach lesson on why links from here.
- **Live pot odds, from the villain's perspective** (row below the bar):
  `Nia calls 120 → she gets 2.5:1 (needs 28%)`. This is the single most
  educational number in bet sizing: it makes "size to deny draw odds" a
  visible, manipulable quantity. When *facing* a bet, the choosing state shows
  hero's own odds instead (`to call 20 · pot odds 2.2:1 (31%)`).
- The `all-in 900` label doubles as the max clamp; when a raise's min exceeds
  the stack, sizing mode is skipped entirely — the only legal raise is all-in
  and `r` renders as `r all-in 285`.

```go
// action_bar.go
type ActionBarState int

const (
    ActionBarWaiting ActionBarState = iota
    ActionBarChoosing
    ActionBarSizing
)

type ActionBar struct {
    state   ActionBarState
    legal   engine.LegalActions // includes BetRange{Min, Max}, CallAmount
    amount  int                 // current sizing value (chips)
    typed   string              // digits typed so far ("" = slider/preset value shown)
    pot     engine.PotState     // for preset + odds math
    width   int
}

func NewActionBar() *ActionBar
func (b *ActionBar) SetLegal(l engine.LegalActions, pot engine.PotState)
func (b *ActionBar) Update(msg tea.KeyMsg) (action *engine.Action, cmd tea.Cmd)
func (b *ActionBar) View(width int) string // always exactly 2 rows
func (b *ActionBar) presetAmounts() [5]int // ⅓, ½, ⅔, pot, all-in (clamped)
```

---

## 5. The coach panel

### 5.1 Placement

- **80-col layout:** a 2-row strip between the hero row and the action bar
  (mockups A–C). Reserved at all verbosity levels including Off (it shows a
  neutral street summary when the coach is silent) — the layout-stability
  invariant forbids reflowing the table when coaching toggles.
- **≥104-col layout:** a 28-col bordered right panel (mockup §2.6) with room
  for recommendation + reasoning + last-action grade simultaneously.
- **Expansion:** the strip shows recommendation + one line of reasoning; the
  full reasoning (outs enumerated, range talk, the arithmetic) is one
  keypress away — `e` opens a centered overlay (euchre teachable-popup
  pattern: modal over the frozen table, any key closes). The strip shows
  `[e more]` when truncated.

### 5.2 Content model

One source of truth: the same AI that plays the opponents produces the hero
recommendation (euchre's `coach ai.Player` in the hero's seat). The panel
renders a `CoachAdvice` produced by `internal/coach`:

```go
// internal/coach/coach.go (consumed by the TUI; produced outside it)
type Advice struct {
    Recommended engine.Action // what the coach would do
    Line        string        // one-sentence recommendation ("3-bet to ~90")
    Why         string        // full reasoning paragraph (overlay / side panel)
    PotOdds     string        // pre-formatted "2.2:1 (31%)"
    Outs        []engine.Card // when drawing; rendered inline-style
    Equity      float64       // vs estimated range, when cheaply available
}

type Grade struct {
    Score   rune    // 'A'..'D', or '✓'/'•' in strip form
    Matched bool    // action matched coach recommendation
    EVNote  string  // "calling here loses ~12 chips vs folding"
}
```

### 5.3 Grading without nagging

After the hero acts, the grade renders on the **hero's reserved action line**
(row 15) — `3-bet to 90  ✓ good` / `called 190  ✗ C — see review` — and the
coach strip explains it while the AIs respond (euchre's `gradeMsg` flow).
Grades persist into the `HandHistory` so post-hand review can show every
decision's grade on the replay timeline. Mistakes (`Matched == false` with
material EV loss) are what `HandReview` sorts to the top.

### 5.4 Verbosity — the coach must know when to shut up

`CoachMode`, cycled live with `tab` and set in Setup/Settings:

| Mode | Before hero acts | After hero acts |
|------|------------------|-----------------|
| `CoachFull` (default for new profiles) | Recommendation + reasoning line | Grade + explanation, always |
| `CoachMistakes` | Strip shows neutral info (pot odds only — the *numbers*, no *opinion*) | Silent on good moves; grade + explanation only on mistakes |
| `CoachOff` | Neutral street summary | Nothing (grades still recorded for review) |

Rationale: the progression mirrors learning — first you want the answer
explained, then you want to decide alone but be caught when wrong, then you
want a clean game whose mistakes you review afterwards. Pot-odds numbers stay
visible in `CoachMistakes` because they're table stakes information a casino
HUD-less player must compute anyway — showing math isn't coaching, it's the
scoreboard.

Teachable-moment popups carry over from euchre (`tutorial_popups.go` pattern):
first time a side pot forms, first time hero is offered a check-raise, first
string bet attempt, etc. — one modal per concept per profile, tracked in
`shownConcepts`.

---

## 6. Animation & pacing

### 6.1 Principles

Euchre's animation system (named `tea.Tick` msgs per animation, frame counters
on the model, constants block) carries over wholesale. Poker-specific rule:
**animate information arrival, not decoration**. Every animation exists to
give the eye time to register a state change that matters strategically.

| Event | Animation | Teaches |
|-------|-----------|---------|
| Deal | Hole cards fly to seats in order, two passes, ~120ms/card | Deal order starts left of button |
| Blind posting | `● 5` / `● 10` chips appear with a beat between | Who owes what before cards |
| Villain action | "thinking…" for a think-time, then action label lands on the seat's action line | Pace of reading opponents |
| Flop | Three cards flip **one at a time** (~350ms apart at Normal) | Read the board incrementally — flush/straight textures |
| Turn/river | Single flip + a fixed post-flip pause | Board rereading habit |
| Bet/call | Chip glyph slides conceptually: seat `●` amount updates, then a beat | Bet is in front of you until street ends |
| Street end | All `●` amounts sweep into the POT line (3 frames) | Pot accretion; when money is "locked in" |
| Showdown | Reveal seats one at a time in showdown order; winning 5 cards get gold borders; pots awarded one pot at a time | Showdown order rules; side-pot resolution |

### 6.2 The decision pause — pacing that teaches

The most important pacing feature is not an animation: after each street's
cards land, at `SpeedLearn` the game **pauses until the player presses
space**, with the status line reading `flop dealt — space to continue`. A
beginner gets unlimited time to read the board *before* anyone acts, and the
act of dismissing is a micro-commitment to having read it. At faster speeds
the pause becomes a timed beat, then zero.

### 6.3 Speed setting

```go
type Speed int

const (
    SpeedLearn   Speed = iota // pauses on new cards; villain think 1.5–3s; all animations
    SpeedNormal               // no hard pauses; think 0.8–1.5s; all animations
    SpeedFast                 // think 300ms flat; deal/flip animations at 2×
    SpeedInstant              // no delays; state changes render immediately (also what tests use)
)
```

- Adjustable live with `+`/`-` (status line flashes `speed: fast`), persisted.
- Villain think-time at Learn/Normal is drawn from a small range and **scaled
  by decision size** (facing a raise > open fold) — fake, but it models the
  real tell-free rhythm of online play and gives the player reading time
  exactly when the situation is complex.
- All durations live in one constants block (euchre style) *multiplied by a
  per-speed factor*; `SpeedInstant` short-circuits every `tea.Tick` — this is
  the hook that makes golden tests deterministic (§8.3).

---

## 7. Theme & styling

### 7.1 Structure — `internal/ui/theme`

Euchre's single-file theme grew ad-hoc style fields; we keep the shape
(`theme.Current *Theme`, adaptive palette vars) but split by concern and add
the glyph table:

```
internal/ui/theme/
  palette.go   // adaptive color vars (the only place hex codes appear)
  theme.go     // Theme struct of lipgloss styles + Default() + Current
  glyphs.go    // Glyphs struct: Unicode + ASCII sets, chosen at startup
  suits.go     // SuitStyle(suit) — four-color/two-color resolution
```

```go
// palette.go — every color adaptive Light/Dark (euchre convention),
// except card-interior inks which are fixed (a card is a physical object).
var (
    ColAccent  = lipgloss.AdaptiveColor{Light: "#2178C4", Dark: "#3498DB"} // turn marker, borders
    ColFelt    = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"} // pot, money-good
    ColWarn    = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#E74C3C"} // fold, errors, bad grades
    ColGold    = lipgloss.AdaptiveColor{Light: "#B9770E", Dark: "#F1C40F"} // dealer, coach, winners
    ColMuted   = lipgloss.AdaptiveColor{Light: "#7F8C8D", Dark: "#95A5A6"}
    ColText    = lipgloss.AdaptiveColor{Light: "#2C3E50", Dark: "#ECF0F1"}
    ColPip     = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"} // card backs
    // Suit inks (four-color deck): see §3.3 table; spade ink is adaptive,
    // heart/diamond/club inks are fixed pairs resolved via AdaptiveColor.
)

// theme.go
type Theme struct {
    // Table
    SeatName, SeatStack, SeatBet, SeatAction  lipgloss.Style
    SeatFolded, SeatAllIn, SeatToAct          lipgloss.Style
    PotLine, SidePotLine, BoardPlaceholder    lipgloss.Style
    DealerBadge, PosBadge, HeroBadge          lipgloss.Style
    // Cards
    CardBorder, CardBack, CardWinner          lipgloss.Style
    // Panels & chrome
    Header, Rule, StatusLine, Footer          lipgloss.Style
    CoachBox, CoachTitle, GradeGood, GradeBad lipgloss.Style
    ActionKeycap, ActionLabel, SizingSlider   lipgloss.Style
    // Menus / shared screens (euchre parity)
    Title, Subtitle, Body, Help, ContentBox, ScreenBorder     lipgloss.Style
    MenuItem, MenuItemSelected, MenuItemDisabled              lipgloss.Style
}

var Current = Default()
func Default() *Theme

// glyphs.go
type Glyphs struct {
    SuitSpade, SuitHeart, SuitDiamond, SuitClub string // "♠♥♦♣" / "shdc"
    FaceDown, Mucked, BoardSlot                 string // "▓▓" "· ·" "·" / "##" ".." "."
    Dealer, ToAct, ChipsInFront                 string // "Ⓓ" "►" "●" / "(D)" "->" "*"
    CardTL, CardTR, CardBL, CardBR, CardH, CardV string // "╭╮╰╯─│" / "+ + + + - |"
    RuleH                                       string  // "─" / "-"
}
var G Glyphs // set once at startup from termenv/locale probe + settings override
```

Rules: no hex literal outside `palette.go`; no raw glyph outside `glyphs.go`;
components take styles from `theme.Current`, never construct colored styles
inline (euchre drifted on this — `game_play.go` has inline `#FFD700` etc.; we
make it a lint-able convention from day one).

### 7.2 Light/dark

Everything chrome-level uses `AdaptiveColor` (lipgloss resolves against the
detected background). The two traps, handled explicitly: spade/club ink must
flip with background (near-white on dark, near-black on light — see §3.3), and
face-down card fill must contrast with *both* backgrounds (the `ColPip` blue
does). Manual override `--theme light|dark|mono` on the CLI for terminals that
misreport.

---

## 8. Testing the TUI

All three euchre TUI-test ideas carry over, plus goldens. Test-only rendering
runs at `SpeedInstant` with a seeded deck, so any table state is constructible
synchronously (`renderableTable(t, opts…)` helper, mirroring euchre's
`renderableGamePlay`).

### 8.1 Layout stability (`layout_stability_test.go`)

Euchre's `posOf`/`assertAnchorsStable` helpers port unchanged. Anchors for the
table screen: `"POT"`, hero name `"YOU"`, one villain name per row band, and
the action-bar rule line. Mutations that must not move any anchor or change
total view height:

- each seat's action label empty → short → absurdly long (truncation, never wrap)
- board empty → flop → turn → river (placeholder pips guarantee this)
- pot line single pot → main + 2 side pots (one line, truncates eligible-player list)
- coach strip: off / short tip / long tip (fixed 2-row slot, euchre's `coachBoxBodyLines` trick)
- action bar: waiting / choosing / sizing / hero grade line filled
- hand-strength label short ("pair") → long ("straight flush, king high")
- ALL-IN badges, `►` marker on each seat in turn

Run the whole matrix at 80×24, 104×30, and 60×20 (compact anchors: `"POT"`,
`"Board"`, `"YOU"`).

### 8.2 Keybind legend (`keybind_legend_test.go`)

The invariant: **every key the Update loop accepts in a given state is visible
on screen in that state, and vice versa.** Concretely:

- For each `LegalActions` fixture (check/bet, fold/call/raise, call-only,
  all-in-only), the rendered action bar contains exactly the keycaps for those
  actions with correct amounts — no `x check` when facing a bet, no `f fold`
  rendered dimmed.
- Sizing mode: `1`–`5` presets rendered match `presetAmounts()` exactly;
  amounts respect clamps.
- The `?` help sheet lists every binding in §4.2's table (test iterates a
  single source-of-truth `keymap` slice that both the help renderer and the
  test consume — an improvement over euchre, where the legend and handlers
  could drift).
- Action bar height is exactly 2 rows in every state (euchre's
  `handAreaHeight` invariant, poker-sized).

### 8.3 Golden renders (`golden_test.go` + `testdata/*.golden`)

What a golden test looks like here:

```go
func TestGoldenTableFlopSizing(t *testing.T) {
    g := renderableTable(t,
        withSeed(17),                    // deterministic deck
        withScript("f r30 f f f c ..."), // replay actions to reach the state
        withSize(80, 24),
        withSpeed(SpeedInstant),
        withCoach(CoachFull),
    )
    g.actionBar.state = ActionBarSizing
    g.actionBar.amount = 120
    assertGolden(t, "table_flop_sizing_80x24", stripANSI(g.View()))
}
```

- Views are compared **ANSI-stripped** (lipgloss emits no color under the test
  env anyway; stripping makes that explicit) — goldens test *geometry and
  text*, not color. Color correctness is covered by unit tests on
  `theme.SuitStyle` / glyph selection, not by byte-diffing escape codes.
- `-update` flag rewrites goldens; CI fails on diff with a rune-aligned
  first-mismatch report (row/col), because "the whole 24-line blob differs"
  is useless output.
- Golden set: preflop/flop/side-pot/showdown at 80×24, the same four at
  60×20 and 104×30, the too-small screen, the help overlay, one ASCII-glyph
  render (forced `Glyphs` ASCII set) — ~14 files, each ≤3KB. Small enough to
  review in a PR, which is the point: a golden diff *is* the design review.
- Every state is reached **through the engine via a scripted action replay**,
  never by poking private fields (the one exception: action-bar sub-state, which
  is UI-only). This keeps goldens honest against engine changes.

### 8.4 What is not tested in the TUI

Animation frame contents (euchre's lesson: assert *stability across frames*,
not pixel content of frame N), and think-time randomness (`SpeedInstant`
bypasses it).

---

## 9. Package / file layout & key types

```
cmd/holdem/main.go                  # urfave/cli v2 wrapper (euchre parity): flags --theme, --ascii, --speed

internal/app/
  app.go                            # App root model, Screen consts, NavigateMsg/QuitMsg/EndSessionMsg
  main_menu.go                      # MainMenu
  game_setup.go                     # GameSetup → TableConfig
  table.go                          # Table model: session loop, msg plumbing, View() composition
  table_layout.go                   # pure layout fns: full/wide/compact assembly, breakpoints, renderTooSmall
  action_bar.go                     # ActionBar (§4)
  coach_panel.go                    # strip/panel/overlay rendering of coach.Advice + Grade
  hand_review.go                    # HandReview: replay timeline, revealed cards, EV/grade annotations
  lessons.go                        # Lessons (curriculum shell; content in internal/lessons)
  trainer.go                        # Trainer drills (hand ranking / outs / equity quizzes)
  quick_reference.go                # tabbed cheat sheet (euchre QuickReference pattern)
  settings.go                       # Settings screen (writes usersettings)
  tutorial_popups.go                # teachable-moment concepts + modal (euchre pattern)
  keymap.go                         # single source of truth for bindings (help sheet + tests consume)
  layout_stability_test.go          # §8.1
  keybind_legend_test.go            # §8.2
  golden_test.go + testdata/        # §8.3

internal/ui/components/
  card.go                           # MiniCard, InlineCard, FaceDown, BoardRow (5 slots + placeholders)
  seat.go                           # SeatView: 3-row block + compact 1-row form
  pot.go                            # pot line incl. side pots + award animation frames
  slider.go                         # sizing slider readout (pure fn of min/max/value/width)
  menu.go                           # ported from euchre
  animation.go                      # tick msg types, frame helpers, speed scaling

internal/ui/theme/
  palette.go  theme.go  glyphs.go  suits.go        # §7

internal/usersettings/settings.go   # persisted prefs (XDG path): speed, coach mode, deck colors, glyph set
```

Engine/AI/coach/lessons packages are specified in their own design docs; the
TUI consumes `engine.TableState`, `engine.LegalActions`, `engine.HandHistory`,
`coach.Advice`, `coach.Grade`.

### 9.1 Key model signatures

```go
// table.go
type Table struct {
    session  *engine.Session   // stacks, seats, hand lifecycle (persists across hands)
    ai       []ai.Player       // seats 1..5
    coach    *coach.Coach      // advice + grading (same strategy engine as ai)
    settings TableConfig

    actionBar  *ActionBar
    coachPanel *CoachPanel
    popups     *PopupTracker            // teachable moments shown this profile

    // presentation state
    seatAnim   [6]SeatAnimState         // deal/chip/reveal frame counters
    boardShown int                      // cards currently face-up (flip animation)
    pauseFor   PauseReason              // §6.2 decision pause (PauseNone when free)
    grades     []coach.Grade            // this hand, indexed by hero decision
    lastAdvice *coach.Advice

    width, height int
    showHelp      bool
    speed         Speed
}

func NewTable(cfg TableConfig) *Table
func (t *Table) Init() tea.Cmd
func (t *Table) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (t *Table) View() string

// table_layout.go — pure, unit-testable layout functions
type LayoutMode int
const (
    LayoutWide    LayoutMode = iota // ≥104×28: coach side panel
    LayoutFull                      // ≥80×24: coach strip
    LayoutCompact                   // ≥60×20: ledger
    LayoutTooSmall
)
func layoutFor(w, h int) LayoutMode
func (t *Table) viewFull() string
func (t *Table) viewWide() string
func (t *Table) viewCompact() string
func renderTooSmall(w, h int) string

// components/seat.go
type SeatView struct {
    Name       string
    Position   engine.Position // BTN/SB/BB/UTG/HJ/CO
    Stack, Bet int
    Cards      SeatCards       // FaceDown | Folded | Revealed([2]engine.Card)
    IsDealer, IsHero, ToAct, AllIn bool
    LastAction string          // truncated to slot width, never wraps
}
func (s SeatView) Render(g theme.Glyphs) string        // 3 rows × ≤20 cols
func (s SeatView) RenderCompact(g theme.Glyphs) string // 1 row

// components/card.go
func MiniCard(c engine.Card) string            // 4×3
func MiniCardBack() string
func InlineCard(c engine.Card) string          // 2 styled cells (3 in ASCII mode)
func BoardRow(cards []engine.Card, slots int) string // dealt + placeholder pips

// components/pot.go
func PotLine(pots []engine.Pot, names func(seat int) string, maxWidth int) string

// hand_review.go
type HandReview struct {
    hand     *engine.HandHistory
    grades   []coach.Grade
    step     int    // current action index on the replay timeline
    returnTo Screen
    width, height int
}
```

### 9.2 Message flow on the table (euchre pattern, poker names)

`villainActMsg` (post think-time) → engine apply → `streetEndMsg` → chip-sweep
ticks → `dealBoardMsg` per card → decision pause → `heroTurnMsg` (action bar
arms, coach computes advice via `tea.Cmd` so equity math never blocks render)
→ hero acts → `gradeMsg` → … → `handEndMsg` → showdown reveal ticks → pot
award ticks → `nextHandMsg`. Every delay is a named `tea.Tick` constant scaled
by `Speed`, exactly like euchre's animation constants block.

---

## Appendix: open questions (deliberately deferred)

- **Mouse support** for the sizing slider — Bubble Tea supports it; deferred
  until the keyboard flow proves out (keyboard is the primary interface, and
  the presets likely make the slider a readout in practice).
- **Hand-strength label wording** ("top pair, top kicker") comes from the
  coach package; whether it hides at `CoachOff` (purists) is a settings
  decision to make after playtesting.
- **VHS tapes**: one `.tape` per mockup in this doc (preflop, sizing, side
  pot) so the README GIFs and the design stay in lockstep.
