# Stdin

Lints a single file's content read from stdin, named with `--assume-filename`.

![demo](.vhs/demo.gif)

- `rules/commit-message.md` — the rule.
- `message.txt` — a commit message piped in as `commit.msg`.

Rules match by file name, and a pipe has no name of its own, so
`--assume-filename` supplies one. Anything that is not a diff can be linted this
way — here, straight from git:

```sh
export OPENCODE_API_KEY=...
git log -1 --format=%B | gallant --provider opencode --model jev-1.13-free \
  --rules examples/stdin/rules --assume-filename commit.msg
```

The demo uses a saved message instead, so it does not depend on the repository
state:

```sh
cat examples/stdin/message.txt | gallant --provider opencode --model jev-1.13-free \
  --rules examples/stdin/rules --assume-filename commit.msg
```

The text is evaluated by a model, so the API key is required. See the
[diff example](../diff) for linting a unified diff instead.

Re-record the demo GIF with [VHS](https://github.com/charmbracelet/vhs):

```sh
OPENCODE_API_KEY=... vhs examples/stdin/.vhs/demo.tape
```
