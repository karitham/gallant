---
appliesTo: "*.go"
type: noul
---
# Encode invariants in types

When a value has rules about what it may be, give it a type that can only hold
legal values instead of a built-in type that gets checked at every use.

A bare `string` or `int` says nothing about its domain. Every rule about it must
be re-checked at each use, and any new code path can forget to. A dedicated type
moves the check to one place — construction — and puts the rule in the
signatures, so the rest of the program can treat the value as already valid.
That is the difference between *validating*, which re-checks the same fact, and
*parsing*, which proves it once and carries the proof forward.

Judge the file by how it handles values that have rules:

- **Violates** — a built-in type stands in for a fixed set of values (a status,
  a kind, a mode) and the set is enforced by scattered comparisons rather than
  by a named type.
- **Violates** — one built-in type carries two concepts that must not be
  interchanged, such as two kinds of identifier, so nothing stops a caller
  swapping them.
- **Violates** — the same shape or range check is repeated at several call sites
  instead of being done once when the value is built.
- **Does not violate** — the file defines a type with a constructor or parser
  that rejects illegal values, and uses that type throughout.
- **Does not violate** — the values carry no domain rules, as in a config struct
  or a serialization DTO. A built-in type is correct there.

Ignore invariants enforced by the compiler, the standard library, or a package
this file cannot change.
