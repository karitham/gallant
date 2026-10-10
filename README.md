# gallant

Gallant is a linter whose rules are written in Markdown.

Because rules are plain files, they can be versioned alongside the code they
describe and shared across projects.

## Installation

Install the latest release with Go:

```sh
go install github.com/madxmike/gallant/cmd/gallant@latest
```

To install a specific release, pin its version:

```sh
go install github.com/madxmike/gallant/cmd/gallant@v0.1.0
```

Prebuilt binaries for Linux, macOS, and Windows (amd64 and arm64) are attached
to every [release](https://github.com/madxmike/gallant/releases). Download and
unpack the archive for your platform:

```sh
VERSION=0.1.0
OS=darwin   # linux, darwin, windows
ARCH=arm64  # amd64, arm64
curl -fsSL -o gallant.tar.gz \
  "https://github.com/madxmike/gallant/releases/download/v${VERSION}/gallant_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf gallant.tar.gz gallant
sudo install gallant /usr/local/bin/
```

Windows archives use the `.zip` extension instead. Every release also ships a
`checksums.txt` so downloads can be verified.

## Usage

Run it from the root of a project:

```sh
gallant
```

Files are evaluated in parallel, eight at a time by default. Change the number
of concurrent evaluations with `--jobs`:

```sh
gallant --jobs 16
```

## Output

Findings print one per line in a fixed, space-separated form:

    <file> <rule> <type> <confidence> <result>

Backslashes, newlines, and other control characters within a field are escaped,
so every finding stays on a single line and no field contains a tab. The result
is the last field and may contain spaces.

For machine consumption, `--json` emits an array of objects instead:

```sh
gallant --json
```

## Status

Gallant is under active development.

## Development

Enable the repository's git hooks, which include a pre-commit check that runs
gallant over the source:

```sh
git config core.hooksPath .githooks
```

The check runs only when `OPENCODE_API_KEY` is set, and skips with a notice
otherwise.

Run the tests:

```sh
go test ./...
```

The `rules` package uses golden files. To regenerate them after an intentional
change:

```sh
go test ./rules -update
```

## Contributing

The examples ship terminal recordings generated with
[VHS](https://github.com/charmbracelet/vhs). Re-recording them requires VHS to
be installed:

```sh
OPENCODE_API_KEY=... vhs examples/ste100/.vhs/demo.tape
```

## License

MIT. See [LICENSE](LICENSE).
