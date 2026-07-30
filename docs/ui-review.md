# UI/UX Review — the whole product, seen at once

Every frame quoted below is a real render, captured by driving the built app
(`App` + screens) through Bubble Tea at 80×24, 104×30, and 60×20, plus the
checked-in goldens in `internal/app/testdata/`. Where this review disagrees
with `docs/design-tui.md`, the built app is the subject; the design doc is
cited only to show where a good call was dropped or a bad one kept.

---

## 1. Verdict

This is a genuinely good product with one systemic failure mode: **the app is
excellent at computing what a beginner needs and mediocre at making sure they
actually see it.** The engine, the coach's reasoning, the frozen grades, the
review's Then/Now split — the hard intellectual work is done and done right.
But at the default 80×24 size the coach's teaching sentence is cut off mid-
thought on essentially every decision; the grade of your action is visible for
a second or two and then gone; range charts crop silently with no cue that
rows are missing; and the compact layout silently drops pot odds — the one
number the curriculum is built around. Individually each looks like a small
rendering compromise. Together they mean the learning content — the reason
this app exists — is systematically the first thing sacrificed to layout
discipline.

The second-order problem is coherence: the screens split into two visual
families (bordered floating menus vs. full-bleed working screens) that flip
mid-flow, and the product presents five equal doors to a person who cannot
yet know which door to open.

Nothing here needs a redesign. The bones — layout stability, one keymap/help
source of truth, the ledger fallback, the review screen's structure — are
worth protecting. The fixes are mostly about *promotion*: taking content the
app already computes and giving it the rows, the persistence, and the
signposting it deserves.

## 2. What works — protect these

- **The 60×20 compact ledger** is a real design, not an afterthought. Six
  seats, board, coach line and action bar in 20 rows, and it stays stable.
  Most TUIs fail this; this one doesn't.
- **Layout stability everywhere.** Nothing reflows when the coach toggles,
  a drill is answered, or a bet lands. The eye always knows where to look.
- **One source of truth for keys.** Keymap → dispatch → help overlay → legend
  tests. The help sheet never lies. Keep this invariant at any cost.
- **The hand review screen** is the best screen in the app: frozen `Then`
  vs. dimmed, tagged `Now`, the DECISIONS/OUTCOME split footer, and lines
  like *"Right play, bad result. Over time this play prints money."* This is
  the product's thesis rendered in four rows.
- **The coach's actual prose.** *"77 is in the BTN open chart. From the
  Button you act last — position is money."* *"Risk 35 to win 50 → need
  41%."* Concrete, numeric, warm. (The problem is display, not writing.)
- **Locked lessons explain themselves** (`Locked · finish How a Hand Works
  first`) instead of being dead rows.
- **The sizing bar's villain-price preview** (`Tara calls 15 · gets 3.0:1
  (needs 25%)`) teaches the *other* side of a bet — quietly brilliant.
- **The lesson content voice** — "the money you didn't lose is identical to
  money won" — is consistently excellent.

---

## 3. Findings

Ordered by impact on a beginner's ability to learn poker, not by effort.

### F1 — HIGH: At the default size, the coach's teaching is an ellipsis

Every 80×24 golden shows the same thing — the strip's second row clips the
reasoning at ~70 characters:

```
 COACH  Raise to 25
        77 is in the BTN open chart. From the Button you act last — position is…
```
```
 COACH  Fold
        You need 33% to call (85 into 170) and have about 17% against his likel…
```

The full sentence — the arithmetic, the range read, the *lesson* — exists
(the 104-col side panel proves it: *"The price is 1.4:1 — risk 35 to win 50 →
need 41%. I put him on ≈16% range (opened from HJ)."*) but at the target
terminal size the beginner never sees the second half of any explanation, on
any street, of any hand. There is no way to expand it: `design-tui.md` §5.1
specified an `e` overlay with an `[e more]` marker when truncated, and the
implementation dropped both. This is the single most damaging cut in the app,
because the live coach is the product's headline feature.

Meanwhile the same frame has two entirely blank rows between the action bar
and the status line:

```
  f fold      c call 10      r raise…          to call 10 · pot odds 1.5:1 (40%)
                                                                    ← blank
                                                                    ← blank
 Your turn · fold, call, or raise
```

**Recommendation** (either, ideally both):

1. Give the strip its rows back. A 3-row reasoning budget fits ~210 chars —
   enough for nearly every rationale in the corpus:

```
────────────────────────────────────────────────────────────────────────────────
 COACH  Fold
        You need 33% to call (85 into 170) and have about 17% against his
        likely range. Bad price for a weak draw — fold and keep the 85.
────────────────────────────────────────────────────────────────────────────────
  f fold      c call 85      r raise…          to call 85 · pot odds 2.0:1 (33%)
 Your turn · fold, call, or raise
                                                   esc menu · ? help
```

2. Restore the designed `e` expand overlay, and show a visible marker
   whenever clipping happened: `…position is[e]`. An invisible truncation is
   the worst of both worlds — the beginner doesn't even know there was more.

### F2 — HIGH: Grades are computed forever and shown for a second

Principle #1 of the whole design is "grade the decision, not the outcome" —
and the live table barely shows the grade. Today the grade renders only on
the coach strip's second row *while the villains respond*, is cleared the
moment your next turn arms (`armBar`: `m.lastGrade = nil`), and between hands
the strip is replaced by a blank "between hands" region:

```
────────────────────────────────────────────────────────────────────────────────
 between hands
                                                                    ← blank row
────────────────────────────────────────────────────────────────────────────────
```

At Fast/Instant speed the grade frame is never rendered at all (verified: a
deliberately bad preflop call at SpeedInstant produced no visible grade in
any frame). The design's durable surface — the grade on the hero's reserved
action line (`design-tui.md` §5.3: `called 190  ✗ C — see review`) — is an
acknowledged TODO (`table.go:324`). So `CoachMistakes` mode's promise, "be
caught only when you're wrong," currently means "be caught for about two
seconds, if your speed setting is slow enough."

**Recommendation:**

1. Wire the hero-line grade (the row is already reserved and usually blank):

```
                            ╭──╮ ╭──╮
   ► YOU ᴮᵀᴺ Ⓓ 965          │T♥│ │K♠│        high card, king
                            ╰──╯ ╰──╯
   called 25  ✗ Mistake — fold was the play (−1.2bb)
```

2. Make the between-hands strip earn its two rows with the hand's verdict:

```
 hand over · 2 decisions: 1 Best, 1 Mistake (−1.2bb) · v for the full review
```

This also gives `v review` a reason to be pressed — today nothing on the
between-hands screen tells you the review has anything interesting in it.

### F3 — HIGH: Five equal doors, and no path between them

The product has a curriculum (lessons → table-with-coach → trainer → review)
but the main menu presents five undifferentiated rows to someone who, by the
product's own premise, doesn't know what "equity" or a "6-max cash game" is:

```
│    > Play                                          │
│      Lessons                                       │
│      Trainer                                       │
│      Quick Reference                               │
│      Settings                                      │
│      Quit                                          │
│  6-max cash game against five AI opponents         │
```

Specific failures of guidance, all captured:

- Nothing distinguishes first launch from the hundredth. No progress, no
  "start here", and the default cursor is on Play — the door a true beginner
  should open *last*.
- After completing lesson 1, the list returns with the cursor still on the
  *completed* lesson (`> ✔ 1. Hand Rankings`), not on the newly unlocked
  lesson 2. `resumeIndex` runs only at construction. The app literally
  un-guides you at the exact moment you follow its path.
- The trainer menu resets its cursor to the top after every session, so
  "practice the next skill" always requires re-navigation.
- The detail line that would explain each door sits at the *bottom of the
  box*, directly under "Quit", where it reads as a caption for the wrong row
  (same pattern on Game Setup, where "Deal in with the settings below"
  appears under "Back").

**Recommendation.** No new screens needed — put state into the rows the menu
already has, and fix the two cursor bugs:

```
│    > Play             hand #38 · coach: Full       │
│      Lessons          next: 2. How a Hand Works    │
│      Trainer          weakest: outs counting       │
│      Quick Reference                               │
│      Settings                                      │
│      Quit                                          │
```

For an empty profile, the Lessons row reads `start here · 13 lessons` and the
cursor starts on it. After a lesson completes, land the list cursor on the
next unlocked lesson. Move the detail line directly beneath the selected row
(or into a fixed slot above the hint line, visually separated from the list).

### F4 — HIGH: Range charts crop silently — the beginner can't know

Lesson 5's under-the-gun grid at 80×24 renders 9 of its 13 rows and stops:

```
 Under-the-gun opening range (~9% of hands)
 AA  AKs AQs AJs ATs A9s A8s A7s A6s A5s A4s A3s A2s
 ...
 A6o K6o Q6o J6o T6o 96o 86o 76o 66  65s 64s 63s 62s
                                                          ← rows 10–13 missing
                                     left/right sections · esc lessons · ? help
```

The last visible row is a clean grid row, so the chart *looks complete* — a
beginner will study a 13×13 range chart that is missing 55, 44, 33, 22 and
four full rows of offsuit hands, and never suspect it. Scrolling exists
(`up/k`, `down/j` are in the keymap and the `?` sheet) but the on-screen
footer says only `left/right sections · esc lessons · ? help` — the one place
a stuck reader will look does not mention the one key they need. At 60×20 the
same section loses 5+ rows. The trainer and table never have this problem
because their budgets were designed to content; the lesson prose renderer
(`renderProse`) just returns `lines[scroll:]` and lets the frame crop.

**Recommendation.** When `maxScroll > 0`, show it and say it:

```
 A6o K6o Q6o J6o T6o 96o 86o 76o 66  65s 64s 63s 62s
 ▼ 4 more rows — j/k or arrows scroll
                          j/k scroll · left/right sections · esc lessons · ? help
```

Also consider letting the range-grid visual own its height: a 13-row grid
that can't fit could drop the prose above it to a single line rather than
cropping the *chart*, which is the section's payload.

### F5 — HIGH: The sizing UI can't produce the size the coach teaches

The coach's standard open is 2.5bb: `COACH Raise to 25` (preflop golden).
Open the sizing bar in that exact spot and the presets are pot fractions:

```
 RAISE  min 20 ├●───────────────────────────────────┤ all-in 1,000
  1 1/3 20  2 1/2 23  3 2/3 27  4 pot 35  5 all-in       arrows nudge · 0-9 type
```

20, 23, 27, 35 — the recommended 25 is not on the row, and the arrow nudge
moves one big blind (20 → 30), so it skips 25 too. The only way to make the
play the coach just taught is to type `2 5` digit-by-digit. The same mismatch
appears in the lesson 5 scripted hand, where the text says *"Open to 2.5 big
blinds"* directly above a preset row that cannot produce it. Fractions of pot
are also the wrong preflop vocabulary: every chart and lesson in this app
talks about preflop raises in big blinds; pot fractions are a postflop
language. (Postflop, the presets are exactly right — keep them.)

**Recommendation.** Preflop presets in the taught vocabulary, with the
coach's number as a first-class citizen:

```
 RAISE  min 20 ├──●─────────────────────────────────┤ all-in 1,000
  1 2bb 20   2 2.5bb 25   3 3bb 30   4 3.5bb 35   5 all-in     coach: 25
```

When facing a raise, presets in multiples of the open (2.2x / 3x / 3.5x).
Consider seeding the slider at the coach's recommendation in `CoachFull`
mode — the default `enter raise to 58` when the coach said fold-or-25 is
noise.

### F6 — MEDIUM: Pot odds vanish entirely at 60×20

At 80×24 the price lives in the action bar info (`to call 35 · pot odds
1.4:1 (41%)`). At 60×20, both that info cell and the coach's numbers are
dropped; the entire frame contains no price:

```
 COACH  Fold
────────────────────────────────────────────────────────────
  f fold      c call 10      r raise…
```

The curriculum's central skill — "always know the price" (lesson 7, quickref
tab 3, the trainer's equity quiz) — is unpracticeable in the compact layout,
which is exactly the layout where a learner squeezing the game next to notes
would live. There is room: the coach line clips the headline into a 60-wide
row that is mostly blank after `Fold`.

**Recommendation.** Fold the price into the compact coach line — it is one
short string the strip already computes:

```
 COACH  Fold · to call 10 · 1.5:1 (40%)
```

Related compact nit, from `table_flop_sizing_60x20.golden`: in sizing mode
the cancel hint is truncated to `esc…` while the (less critical) villain
preview keeps its full text — `enter bet 15 · esc… Tara calls 15 · gets
3.0:1 (needs 25%)`. Keep the interaction hints whole and clip the preview.

### F7 — MEDIUM: Two visual families, and the flip happens mid-flow

Screens split into (a) a full-screen rounded border with a small floating box
(main menu, setup, settings, lessons *list*, trainer *menu/summary*, empty
review), and (b) full-bleed, header + horizontal rule (table, lesson *view*,
trainer *quiz*, quick reference, hand review). The border is not a menu/play
distinction — Quick Reference is menu-like but borderless; the trainer flips
family twice inside one activity (menu → quiz → summary), and Lessons flips
when you open a lesson. On a 104×30 terminal the bordered family draws a
frame around 30 rows of mostly nothing (the menu box occupies ~9 of them).

The table's own bordlessness is right — it needs all 80 columns, and the rule
lines give it structure. The problem is that the app never decided *what the
border means*, so it reads as six authors' defaults, which is what it is.

**Recommendation.** Pick one rule and apply it everywhere. The cheapest
coherent rule, given the table can't have a border: **no screen borders at
all** — every screen gets the table's grammar (title row, rule, content,
status/footer), and the rounded border is reserved for true overlays (help,
future confirms/popups). The main menu at 80×24 becomes:

```
 TEXAS HOLD'EM · ♠ ♥ ♦ ♣                                                ? help
────────────────────────────────────────────────────────────────────────────────
   Learn no-limit hold'em one decision at a time

   > Play              hand #38 · coach: Full
     Lessons           next: 2. How a Hand Works
     Trainer           weakest: outs counting
     Quick Reference
     Settings
     Quit
────────────────────────────────────────────────────────────────────────────────
 up/down move · enter select · ? help · esc quit
```

This also fixes the compact-bordered screens, where the box currently sits
flush against the outer frame (`│Game Setup` touching the border at 60×20).

### F8 — MEDIUM: The review's one *new* fact is the part that gets clipped

The `Now` (hindsight) row spends its width re-listing hole cards that are
already face-up in the seat ledger four rows above, then truncates:

```
  Tara ˢᴮ 995  T♥ 4♠  ● 5          ← cards already revealed here
  Nia ᴮᴮ 990  Q♠ K♣  ● 10
  ...
 Now   Tara held T♥ 4♠, Nia held Q♠ K♣, Cole held Q♦ Q♣, Ivy held A…  hindsight
```

At 60×20 it's worse — `your true …` is exactly the clipped part, i.e. the
true-equity number the hindsight layer exists to deliver. The frame has two
blank rows above its footer.

**Recommendation.** The ledger already does the revealing; make `Now` say
only what is new:

```
 Now   your true equity was 11% — Cole's QQ had you dominated       hindsight
```

Same fix applies to `Then` (also `clip`ped) — or wrap both to two rows using
the spare footer rows.

### F9 — MEDIUM: The app's own explanations disagree with each other

Three sources of strategic/format truth drift apart in rendered output:

- **Sizing:** the coach opens to 2.5bb (25), the lesson says "open to 2.5
  big blinds", the spot quiz offers `Raise to 20` (a min-open) as *the*
  raise choice, and the sizing presets offer none of these (F5). To a
  beginner these read as four different opinions about the same act.
- **Notation:** the coach writes `9×4 ≈ 36%` and `→ need 41%`
  (`coach/explain.go`, raw glyphs — note this also bypasses the
  glyphs-only-in-`glyphs.go` convention and the ASCII-fallback promise);
  the trainer writes `8 outs x 4 ~ 32%` (`trainer/quiz_outs.go`). Same rule,
  two typographies, shown to the same user minutes apart.
- **Wording:** the spot quiz says "Folded to you. You are UTG" — preflop UTG
  is *first to act*; nobody folded. A poker app must not say false things
  about positions, least of all to someone learning positions. The feedback
  block also mixes voices: *"Fold was my pick too."* (first person) directly
  above *"Coach: Fold - J2o is not in the UTG open chart"* (third person,
  and a bare `-` where the coach uses `—`).

**Recommendation.** Route trainer explanation formatting through the same
formatting helpers the coach uses (one `×/≈` policy, one voice), special-case
the UTG prompt ("You open the action"), and have the spot quiz label its
raise choice with the coach's actual open size rather than the least-bad
min-raise candidate.

### F10 — MEDIUM: Teachable moments are built, tested, and never fire

`internal/coach/moments.go` implements the first-side-pot / first-check-raise
teachable-moment system (with `PendingMoment`), and the README's package
table claims moments among the shipped coach features — but nothing in
`internal/app` references it. The captured side-pot frame shows the raw
mechanics with no explanation:

```
          MAIN 180   ·   SIDE 480 (Tara · Nia)   ·   SIDE 2 400 (Nia)
```

A beginner meeting their first side pot gets three labeled numbers and no
sentence. This was the euchre pattern the brief explicitly asked to carry
over, the engine surfaces the data, the coach can produce the text — only
the last wire is missing.

**Recommendation.** Wire `PendingMoment` into the table's decision loop as
the designed one-per-concept modal (the help overlay already proves the
overlay pattern). Until then, soften the README claim.

### F11 — LOW: Key and phrasing inconsistencies (a bundle)

None of these alone matters; together they are the texture of a multi-author
app. All captured:

- `esc`/`q` means "quit the app, no confirmation" on the main menu, "back"
  on every other shell screen, and `q` does nothing at the table. A beginner
  mashing esc to go "up" exits the program.
- Leaving the table via esc silently keeps the session (good) but nothing
  says so at the moment it happens, and `TODO(confirm-leave)` notes the
  designed confirm was dropped. The main menu shows no sign a live session
  exists to return to (Play → setup → Start would build a *new* table; the
  cached-session resume path has no visible door).
- Between-hands footer says both `v reviews` (status line) and `v review`
  (right footer) in one frame.
- The lesson drill footer reads `enter check` even after the answer, when
  enter means continue; the completion splash's footer says `enter continues`
  while its body says "enter returns to the lesson list."
- Pressing → on the last lesson section jumps *backwards* to the first
  unanswered drill (`section 5 of 5` → `section 4 of 5`) — correct behavior,
  disorienting counter. Say why: `answer this drill to finish the lesson` is
  already printed, but the jump needs the counter to stop reading like a
  regression (e.g. `section 4 of 5 · 1 drill left`).
- Status-line capitalization drifts: table `Your turn`, lesson script
  `your turn`, statuses `hand complete` / `between hands` vs. titles
  `COACH`, `LESSON`, `HAND REVIEW`.
- The lessons list detail line clips at 46 columns on an 80-column screen:
  `Know the ten hand categories cold, and that t…` — the list is fixed-width
  for stability, but the detail row could use the frame's real width.

### F12 — LOW: Dead space where teaching could live

- The trainer quiz at 80×24 uses ~13 of 21 content rows; the spot quiz
  renders an eight-row "Board" visual of five placeholders for *preflop*
  questions. The summary screen shows four numbers in a 24-row frame. Room
  exists for the one thing the trainer never shows: what the *levels* mean
  and what leveling up changes ("level 2 adds backdoor draws").
- The 104×30 wide table: the coach panel's border encloses ~13 blank rows,
  and the left column has ~8 blank rows below the action bar. The panel
  could absorb the recent-action log, or the session stat the DESIGN
  headline promises (decision accuracy vs. chips won — currently shown
  nowhere during play).
- Menu screens at 104×30 are unchanged 80-col layouts floating in a larger
  border (see F7).

### F13 — LOW: First-contact jargon on the table

The first frame a player ever sees includes `6-max NLHE`, `BTN · ? help`
(header), and superscript seat tags `ᴮᴮ ᵁᵀᴳ ᴴᴶ ᶜᴼ` — three notations for
positions before the player has had lesson 4. Quick Reference tab 2 explains
all of it, but nothing on the table points there. The superscript Unicode is
also typographically fragile (tiny, font-dependent); the ASCII fallback
handles bare terminals but not squinting. This is half-mitigated by the
lessons (position is lesson 4) — and would be fully mitigated by F10's
first-time popups ("You're on the Button — you act last. `?` explains the
seat tags."). Flagged separately so the fix for F10 is evaluated against it.

---

## 4. Prioritized plan

If effort is limited, in order — the first three change what a beginner
*sees every single decision*:

1. **F1 — un-truncate the coach** (3-row strip and/or the `e` overlay with a
   visible marker). Highest teaching value per line of code in the app.
2. **F2 — make grades persist** (hero-line grade + between-hands verdict).
   The TODO already marks the spot; the review data already exists.
3. **F5 — preflop presets in big blinds, coach size reachable.** Without it,
   the app recommends a play its own controls can't make.
4. **F4 — scroll cue in lessons.** A one-line render change; silently
   cropped range charts actively mis-teach.
5. **F6 — pot odds on the compact coach line.** One string concatenation.
6. **F3 — signposting** (menu detail rows with state, cursor-to-next-lesson,
   trainer cursor memory). Small, but converts five doors into a path.
7. **F10 — wire teachable moments** (subsumes most of F13).
8. **F8/F9/F11 — review-line rewrite, one formatting policy, key/phrase
   sweep.** Batch these; each is small and none is urgent.

**Deliberately leave alone:** the compact ledger's structure (it works); the
table's borderless full-bleed layout and fixed row budgets (the discipline is
the feature); the help/keymap/legend machinery; the review screen's Then/Now
architecture (only its line-level text needs work); the four-color deck and
glyph fallback system; the lesson prose itself. F7 (chrome unification) and
F12 (dead space) are worth doing but only as a considered pass, not as
piecemeal touch-ups — half-unifying the chrome would be worse than the
current honest split.

## 5. Open questions for the owner

1. **Who is the 104-column layout for?** The side panel fixes F1 for wide
   terminals, and the effort of making wide genuinely richer (log + session
   stats + full reasoning) only pays if you actually play wide. If your
   daily terminal is ~80 cols, F1's 3-row strip matters more and the wide
   panel can stay as-is.
2. **Should esc from the main menu quit without confirmation?** Muscle
   memory from every other screen says esc = up. A `press esc again to quit`
   status flash is cheap — but it's your muscle memory that counts.
3. **Where should the decision-accuracy-vs-chips headline live?** DESIGN.md
   §1 calls it the headline session statistic; today it exists per-hand in
   review and nowhere else. Between-hands strip? A session line in the
   header? A stats screen is a bigger commitment than the principle needs.
4. **How locked should the curriculum be?** 12 of 13 locked on first open is
   defensible pedagogy (the detail line explains every lock), but you are
   the one learner this app has: if lockstep ever feels like friction rather
   than structure, an "unlock all" setting costs nothing to your discipline
   and saves the app from fighting you.
5. **Is `CoachMistakes`'s silent strip intentional understatement?** Right
   now it prints the same pot-odds string the action bar already shows two
   rows lower (captured: identical text twice in one frame). If Mistakes
   mode instead showed *nothing* until you err, the silence itself would be
   information. That's a taste call about your own second stage of learning.
6. **Trainer levels: hidden difficulty or visible ladder?** The menu shows
   `level 1/3` but nothing says what levels change or how close you are.
   Showing the mechanism motivates some people and clutters it for others.
