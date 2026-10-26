Output examples/diff/.vhs/demo.gif

Set Shell bash
Set FontSize 20
Set Width 1200
Set Height 520
Set Padding 20
Set TypingSpeed 10ms
Set CursorBlink false

# Build the CLI and put it on PATH without showing it.
Hide
Type "go build -o gallant ./cmd/gallant && export PATH=$PWD:$PATH && clear"
Enter
Sleep 3s
Show

Type "cat examples/diff/change.diff | gallant --provider opencode --model jev-1.13 --rules examples/diff/rules --diff"
Enter
# Let the model answer, then hold on the findings before the GIF ends.
Sleep 3s
Sleep 5s

Hide
Type "rm gallant"
Enter
