# VHS tape for the trainer GIF (assets/holdem-trainer.gif).
# Regenerate with:  vhs assets/tapes/holdem-trainer.tape   (needs `make build`)
#
# Items are generated, so the answers differ every run; the tape deliberately
# picks a fixed choice each time and lets the feedback panel show whichever it
# lands on. Right or wrong, the explanation is the point being demonstrated.

Output assets/holdem-trainer.gif

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

# Main menu -> Trainer (third row).
Sleep 1500ms
Down
Sleep 500ms
Down
Sleep 1200ms
Enter

# The quiz menu.
Sleep 2500ms
Down
Sleep 1500ms
Up
Sleep 1200ms
Enter

# Rankings speed quiz: read the board, answer, read the explanation.
Sleep 3500ms
Type "1"
Sleep 3500ms
Space
Sleep 3s
Type "2"
Sleep 3500ms
Space
Sleep 3s
Type "1"
Sleep 3500ms

# Hold on the final frame.
Sleep 2s
