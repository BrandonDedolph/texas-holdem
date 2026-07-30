# VHS tape for the lessons GIF (assets/holdem-lessons.gif).
# Regenerate with:  vhs assets/tapes/holdem-lessons.tape   (needs `make build`)
#
# The lessons screen is keyboard-driven with no AI timing to race, so fixed
# sleeps are reliable here.

Output assets/holdem-lessons.gif

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

# Main menu. The fresh-profile cursor starts on Lessons, so anchor to
# the top row and count down — the recording keeps working if the
# default cursor moves again.
Sleep 1500ms
Up
Sleep 500ms
Down
Sleep 500ms
Sleep 800ms
Enter

# The curriculum: 13 lessons, locked ones explaining themselves.
Sleep 2500ms
Down
Sleep 1200ms
Down
Sleep 1200ms
Down
Sleep 1800ms

# Back to the top and open lesson 1.
Up
Sleep 400ms
Up
Sleep 400ms
Up
Sleep 1200ms
Enter

# Step through the lesson's sections.
Sleep 3s
Right
Sleep 3s
Right
Sleep 3s
Right
Sleep 3s
Right
Sleep 3s

# Hold on the final frame.
Sleep 2s
