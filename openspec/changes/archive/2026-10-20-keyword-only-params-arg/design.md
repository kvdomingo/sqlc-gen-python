# Design

## Context

`addArgs` in `internal/gen.go` special-cases a single struct argument and
appends it to positional `Args`; every other parameter goes to `KwOnlyArgs`. The
struct's `QueryValue` is always named `arg`, and the sequence-command bodies
hard-code `arg` (`if not arg`, `for a in arg`). Querier protocols are built from
the same `pyast.Arguments`, so they follow automatically.

## Goals / Non-Goals

**Goals:**
- One rule for every generated parameter: keyword-only.
- Name follows the command shape: `args` for sequence-taking commands, `arg`
  otherwise.

**Non-Goals:**
- Changing the annotation to alt's `list[Params]`; `Sequence` stays, since it
  accepts lists and tuples and is already what callers type against.
- A config flag to keep the old positional form.

## Decisions

- **Drop the struct special case in `addArgs`; put it in `KwOnlyArgs`.** The
  `Sequence` wrapping stays. Alternative: a separate code path per command —
  rejected, the existing `takesSequence(q.Cmd)` predicate already splits the
  two shapes.
- **Pick the name where the params `QueryValue` is built** (`"args"` when
  `takesSequence(query.Cmd)`, else `"arg"`), and have the sequence-command
  bodies read `q.Args[0].Name` instead of the literal `"arg"`. Keeps the name in
  one place.
- **No collision handling needed.** The params-class argument is the method's
  only parameter besides `self`, and `arg`/`args` are not module-level names the
  body reads.

## Risks / Trade-offs

- [Breaking for every caller of a params-class method] → Called out in the
  README migration section; the change lands before a tagged release that
  documents the old form as stable.
- [Hand-written protocol fakes in tests drift] → `_check.py` and
  `examples/src/tests/test_library.py` are type-checked against the protocols,
  so a missed fake fails the check.
