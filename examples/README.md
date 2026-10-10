# Examples

Each example is a small set of rules and a document to check them against.

## STE100

Checks whether a markdown document follows ASD-STE100 Simplified Technical English.

- `rules/ste100.md` — the rule.
- `doc.md` — a document that violates the rule.

Run it from the repository root:

```sh
gallant -rules examples/rules examples/doc.md
```

The document is evaluated by a model, so set an API key first:

```sh
export OPENCODE_API_KEY=...
```
