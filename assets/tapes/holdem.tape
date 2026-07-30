# VHS tape for the gameplay GIF (assets/holdem.gif).
# Regenerate with:  vhs assets/tapes/holdem.tape   (from the repo root; needs `make build`)
#
# Timing note: villain think-time varies (1.5-3s per seat at Learn speed), so a
# fixed sleep cannot reliably land a keypress on the hero's turn. The recipe is
# instead self-correcting: keep alternating space (clears a street pause) with
# an action key on a steady beat. Presses that arrive off-turn are ignored by
# the action bar, and the next one lands. Recording longer than needed and
# holding the final frame absorbs the jitter.

Output assets/holdem.gif

Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 620
Set Padding 12
Set Framerate 24
Set PlaybackSpeed 1.0

Hide
# A throwaway profile: menu cursor and status rows are derived from
# saved progress, so recording against the real one would make the
# output depend on how much the author had played that week.
Type "export XDG_DATA_HOME=$(mktemp -d)"
Enter
Type "./bin/holdem"
Enter
Sleep 1s
Show

# Main menu. A fresh profile starts the cursor on Lessons (the app points a
# new player at the curriculum first), so step up to Play explicitly rather
# than assuming a row.
Sleep 2s
Up
Sleep 800ms
Enter

# Game setup: "Start Game" is preselected.
Sleep 2s
Enter

# The deal, the blinds, and the villains acting.
Sleep 4s

# Steady beat: read the coach, act, clear the street pause, repeat.
Sleep 2500ms
Type "c"
Sleep 2500ms
Space
Sleep 2500ms
Type "x"
Sleep 2500ms
Space
Sleep 2500ms
Type "x"
Sleep 2500ms
Space
Sleep 2500ms
Type "x"
Sleep 2500ms
Space
Sleep 3s

# Showdown, pot award, and on into the next hand.
Enter
Sleep 3s
Type "c"
Sleep 2500ms
Space
Sleep 2500ms
Type "x"
Sleep 2500ms
Space
Sleep 3s

# Hold on the final frame.
Sleep 2s
