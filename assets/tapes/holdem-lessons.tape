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
Type "./bin/holdem"
Enter
Sleep 1s
Show

# Main menu -> Lessons (second row).
Sleep 1500ms
Down
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
