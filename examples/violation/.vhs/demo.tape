Output examples/violation/.vhs/demo.gif

Set Shell bash
Set FontSize 20
Set Width 1200
Set Height 460
Set Padding 20
Set TypingSpeed 10ms
Set CursorBlink false

# Build the CLI and put it on PATH without showing it.
Hide
Type "go build -o gallant ./cmd/gallant && export PATH=$PWD:$PATH && clear"
Enter
Sleep 3s
Show

Type "gallant --provider opencode --model jev-1.13 --rules examples/violation/rules examples/violation/sample.go"
Enter
# Let the model answer, then hold on the failing verdict.
Sleep 8s

# A passing rule is hidden until it is asked for.
Type "gallant --provider opencode --model jev-1.13 --include-passing --rules examples/violation/rules examples/violation/clean.go"
Enter
Sleep 8s

Hide
Type "rm gallant"
Enter
