Output examples/json/.vhs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 640
Set Padding 20
Set TypingSpeed 10ms
Set CursorBlink false

# Build the CLI and put it on PATH without showing it.
Hide
Type "go build -o gallant ./cmd/gallant && export PATH=$PWD:$PATH && clear"
Enter
Sleep 3s
Show

Type "gallant --provider opencode --model jev-1.13 --json --rules examples/json/rules examples/json/sample.go"
Enter
# Let the model answer, then hold on the findings before the GIF ends.
Sleep 3s
Sleep 5s

Hide
Type "rm gallant"
Enter
