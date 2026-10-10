# gallant

Gallant is a customizable linter built to utilize Decision Models (i.e. Jev, Clef, and other system one models) to lint your code, documents, and any other text-based files.

## Why Gallant?

Some rules that would be great to express as lints in a codebase are very hard to perform in traditional deterministic systems -- "Does the comments in this file repeat what the code already says". As such, these type of checks are left either to human reviewers in PRs or to expensive and slow LLM models. 

Decision Models are fast, cheap(er), and can work on more fuzzy rules that traditional software has not been able to. Gallant is a solution to provide a simple to use cli program with no runtime requirements and minimal dependencies. 

#### Why not X other Decision Model linter?

I looked at the other available tools doing this type of work, and I was not left impressed. All the tools I saw were immense slop dumps, required runtimes like python/npm, servers you had to continually run, terrible CLIs, or had many many dependencies (aka supply chain attack vectors!)

I build Gallant to (try) and avoid all of those things. Gallant requires no runtime as it is in go. The CLI adhears to standard unix utility principles such as working off files/filepaths, minimizing configuration needed, and doing one thing well. 

Please, if you find it useful give it a share in your favorite tech community!

## Installation

Releases are available to install on the [Releases](https://github.com/madxmike/gallant/releases/) page. 

#### Build From Source

Install the latest release with Go:

```sh
go install github.com/madxmike/gallant/cmd/gallant@latest
```

To install a specific release, pin its version:

```sh
go install github.com/madxmike/gallant/cmd/gallant@v0.1.0
```

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
