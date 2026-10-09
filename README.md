# gallant

Gallant is a linter whose rules are written in Markdown.

Because rules are plain files, they can be versioned alongside the code they
describe and shared across projects.

## Usage

Install the command:

```sh
go install github.com/madxmike/gallant/cmd/gallant@latest
```

Run it from the root of a project:

```sh
gallant
```

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

## License

MIT. See [LICENSE](LICENSE).
