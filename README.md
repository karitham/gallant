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
