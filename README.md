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

Gallant evaluates files with a decision model served by a provider. The default
is `opencode`; see [PROVIDERS.md](PROVIDERS.md) for the available providers.

Gallant takes its input one of three ways.

Pass paths to lint files or directories:

```sh
gallant .
gallant main.go pkg/handler.go
```

Read a unified diff from stdin with `--diff`. Each changed file becomes one
evaluation, named by its path in the diff, so rules match the file they
describe:

```sh
git diff HEAD~1 | gallant --diff
```

Read a single file's content from stdin with `--assume-filename`. The name is
what the rules match, so a pipe can stand in for a file:

```sh
printf 'package main\n// increment this\n' | gallant --assume-filename main.go
```

A bare `gallant` prints usage. `--diff` and `--assume-filename` are mutually
exclusive and take no path operands.

Files are evaluated in parallel, eight at a time by default. Change the number
of concurrent evaluations with `--jobs`:

```sh
gallant --jobs 16 .
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

A file that cannot be evaluated — for example, one too large for the model's
request limit — is reported on standard error as `gallant: <file>: <reason>`. The
remaining files are still evaluated, and the run exits nonzero.

## Limitations

Decision models as of writing (10/10/2026) have relatively small input context windows (orders of a couple hundred to a couple thousand tokens). Some hosted models have larger contexts, but you should expect rejections if the file you are linting is too large. 

If you are facing this you may chunk the file yourself into relevant, smaller, pieces. Gallant does not do this itself, but you can pass arbitrary input from stdin (See Usage above). 

These models are only now becoming mainstream, but are seeing rapid development. The hope is that new models will allow bigger inputs soon enough (as we saw with LLMs). 

## Status

Gallant is under active development.

## Development

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
