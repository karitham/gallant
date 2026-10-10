# Diff

Lints the changed files in a unified diff. Each file becomes one evaluation,
named by its path in the diff, so rules match the file they describe.

![demo](.vhs/demo.gif)

- `rules/leftover-debugging.md` — the rule.
- `change.diff` — a two-file diff, with `handler.go` changed in two places.

`--diff` reads a unified diff from stdin, so `git diff` pipes straight in:

```sh
export OPENCODE_API_KEY=...
git diff HEAD~1 | gallant --provider opencode --model jev-1.13-free \
  --rules examples/diff/rules --diff
```

The demo uses a saved diff instead, so it does not depend on the repository
state:

```sh
cat examples/diff/change.diff | gallant --provider opencode --model jev-1.13-free \
  --rules examples/diff/rules --diff
```

The changed files are evaluated by a model, so the API key is required. See the
[stdin example](../stdin) for linting a single file's content.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/diff/.vhs/demo.tape
```
