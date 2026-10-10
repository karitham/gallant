# Multiple files

Runs two rules over a directory of Go files. Each file that breaks a rule
produces its own line, so one run reports several findings across files.

![demo](.vhs/demo.gif)

- `rules/useless-comments.md` — flags comments that repeat the code.
- `rules/ignored-errors.md` — flags discarded errors.
- `handler.go`, `config.go`, `validate.go` — the files being reviewed.

Set an API key, then run from the repository root:

```sh
export OPENCODE_API_KEY=...
gallant --provider opencode --model jev-1.13-free \
  --rules examples/multiple/rules examples/multiple
```

The files are evaluated by a model, so the API key is required. See the
[JSON output example](../json) for machine-readable output.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/multiple/.vhs/demo.tape
```
