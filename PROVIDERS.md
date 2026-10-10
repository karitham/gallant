# Providers

Choose a provider with `--provider` and a model with `--model`. The default is
`opencode` and its default model is used when `--model` is omitted.

## Table of contents

- [opencode](#opencode)
- [ollama](#ollama)
- [strands-decider](#strands-decider)

## opencode

OpenCode Zen's System One gateway, serving TypeSafe's Jev.

- Models: `jev-1.13`, `jev-1.13-free` (default)
- API key: `OPENCODE_API_KEY`

```sh
export OPENCODE_API_KEY=...
gallant --provider opencode --model jev-1.13-free .
```

## ollama

A local [Ollama](https://ollama.com) server (0.35.0+). No API key; models are
discovered from the server, for example `nimble` (default), `tev1`, `clef`,
`clef-flash`, `laya`.

- Base URL: `OLLAMA_HOST`, default `http://127.0.0.1:11434`

```sh
gallant --provider ollama --model nimble .
```

## strands-decider

A local [Strands Decider](https://github.com/strands-labs/strands-decider)
server (0.1.0+). Strands Decider is AWS Strands Labs' open-source decision model,
served as a System One server. No API key; the served checkpoint is discovered
from the server and defaults to `strands-decider-2B-qwen3.5-v1-2610`.

The server truncates state that exceeds the checkpoint's context window (4,096
tokens for the shipped checkpoints). Start it with `--strict-window` to reject
an oversized file instead, which gallant reports as a per-file error.

- Base URL: `STRANDS_DECIDER_HOST`, default `http://127.0.0.1:8000`

```sh
pip install strands-decider
strands-decider serve StrandsAgents/strands-decider-2B-qwen3.5-v1-2610 --port 8000
gallant --provider strands-decider .
```
