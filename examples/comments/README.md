# Comments

Classifies the comments in a Go file. This uses a **choice** rule.

![demo](.vhs/demo.gif)

- `rules/comments.md` — the rule.
- `sample.go` — the file being reviewed.

Set an API key, then run from the repository root:

```sh
export OPENCODE_API_KEY=...
gallant --provider opencode --model jev-1.13-free \
  --rules examples/comments/rules examples/comments/sample.go
```

The file is evaluated by a model, so the API key is required.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/comments/.vhs/demo.tape
```
