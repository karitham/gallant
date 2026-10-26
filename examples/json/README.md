# JSON output

Emits one JSON object per finding for tools to consume.

![demo](.vhs/demo.gif)

- `rules/comments.md` — the rule.
- `sample.go` — the file being reviewed.

Set an API key, then run from the repository root:

```sh
export OPENCODE_API_KEY=...
gallant --provider opencode --model jev-1.13 --json \
  --rules examples/json/rules examples/json/sample.go
```

Each object has `file`, `rule`, `type`, and `results`. Every outcome the model
weighed is listed in `results`, each with a `result` name and a `confidence`,
the probability of that result: fail and pass for a noul, every option for a
choice, every level for a score. A rule that declares a violation condition
also carries `verdict` (`fail` or `pass`) and `reason`, the clause that decided
it; see the [violation example](../violation). Without `--json`, gallant prints
the same findings as lines; see the [multiple files example](../multiple).

The file is evaluated by a model, so the API key is required.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/json/.vhs/demo.tape
```
