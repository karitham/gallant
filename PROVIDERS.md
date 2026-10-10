# Providers

Choose a provider with `--provider` and a model with `--model`. The default is
`opencode` and its default model is used when `--model` is omitted.

## Table of contents

- [opencode](#opencode)
- [ollama](#ollama)

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
