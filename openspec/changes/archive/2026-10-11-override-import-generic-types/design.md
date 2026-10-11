# Design

## Context

`makePyType` in `internal/gen.go` builds a `pyType` with `InnerType`, `Import`,
`ArrayDims` and `IsNull`. For an override, it sets `InnerType` and `Import` from
the entry. `annotation` in `internal/imports.go` wraps the type name in one
`f.list` per `ArrayDims`, then in `f.optional` if `IsNull`. `f.list` already
picks `list[...]` or `typing.List[...]` from `emit_modern_types`.
`Override.parse` in `internal/config.go` rejects only a dotted `py_type` when
`py_import` is set.

## Goals / Non-Goals

**Goals:**
- `many: true` gives a list of the override type, with the existing list
  spelling and import rules.
- A `py_import` entry with a generic `py_type` fails at config parse time.

**Non-Goals:**
- Other containers (`set`, `dict`, `tuple`). Users can write a dotted or
  builtin generic `py_type` without `py_import`.
- Parsing `py_type` as a Python expression.

## Decisions

- **`many` adds one to `ArrayDims`.** In `makePyType`, when the matched
  override has `Many`, add one to `typ.ArrayDims`. `annotation` already does the
  wrapping, the modern or legacy spelling, the composition with array columns
  and the optional wrap, so `many` needs no new code path.
  - Alternative: extract the imported names from a generic `py_type` such as
    `list[DomainEntity]`. Rejected. It needs a fixed list of builtin names, it
    imports `typing` names such as `Optional` from the wrong module, and it is
    more code.
- **Identifier check:** with `py_import` set, `py_type` must match
  `^[A-Za-z_]\w*$`. This replaces the dot-only check. The error is
  `py_type must be a single identifier when py_import is set; use many: true for a list`.
  `parseConfig` already prefixes the error with the entry. The existing test
  that expects "bare name" changes to the new message.

## Risks / Trade-offs

- [The stricter check rejects any `py_import` `py_type` with brackets.] → Such
  configs always generated invalid imports, so nothing that works breaks.
