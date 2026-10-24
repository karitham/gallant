# JSON output

Emits findings as a JSON array for tools to consume.

![demo](.vhs/demo.gif)

- `rules/comments.md` — the rule.
- `sample.go` — the file being reviewed.

Set an API key, then run from the repository root:

```sh
export OPENCODE_API_KEY=...
gallant --provider opencode --model jev-1.13-free --json \
  --rules examples/json/rules examples/json/sample.go
```

Each result is an object with `file`, `rule`, `type`, `result`, and
`confidence`. Confidence is the probability of that result. Every outcome the
model weighed is emitted: fail and pass for a noul, every option for a choice,
every level for a score. Without `--json`, gallant prints the same results as
lines; see the [multiple files example](../multiple).

The file is evaluated by a model, so the API key is required.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/json/.vhs/demo.tape
```
