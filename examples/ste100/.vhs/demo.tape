Output examples/ste100/.vhs/demo.gif

Set Shell bash
Set FontSize 20
Set Width 1100
Set Height 520
Set Padding 20
Set TypingSpeed 60ms
Set CursorBlink false

# Build the CLI and put it on PATH without showing it.
Hide
Type "go build -o gallant ./cmd/gallant && export PATH=$PWD:$PATH && clear"
Enter
Sleep 3s
Show

Type "gallant --provider opencode --model jev-1.13-free --rules examples/ste100/rules examples/ste100/doc.md"
Enter
Sleep 6s

Hide
Type "rm gallant"
Enter
