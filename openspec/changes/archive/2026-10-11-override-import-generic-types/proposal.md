# Proposal

## Why

An override with `py_import` and a generic `py_type` such as
`{db_type: domain_entity, py_import: app.my_module.models.domain, py_type: list[DomainEntity]}`
generates `from app.my_module.models.domain import list[DomainEntity]`, which is
invalid Python. The plugin uses the whole `py_type` string as the imported name.
Users who want a list of their own type cannot use `py_import` at all.

## What Changes

- New override option `many` (boolean, default `false`). When `true`, the
  generated annotation is a list of `py_type`. With
  `{py_type: DomainEntity, py_import: app.my_module.models.domain, many: true}`
  the plugin emits `from app.my_module.models.domain import DomainEntity` and
  annotates the column `list[DomainEntity]`.
- `many` follows the existing list spelling (`list[T]`, or `List[T]` when
  `emit_modern_types` is off), works with or without `py_import`, and composes
  with array columns and nullability.
- Stricter validation: when `py_import` is set, `py_type` must be a single
  Python identifier. A generic such as `list[DomainEntity]` is rejected with an
  error that points to `many: true`. Such configs never generated valid code,
  so no working config breaks.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `codegen-config`: the "Override imports" requirement enforces a single
  identifier with `py_import`, and a new "Override list wrapping" requirement
  defines `many`.

## Impact

- `internal/config.go`: new `Many` field on `Override`; identifier check in
  `Override.parse`.
- `internal/gen.go` (`makePyType`): apply `many` to the override's type.
- Tests: `internal/config_test.go`, and the `overrides` end-to-end fixture
  under `internal/endtoend/testdata/overrides/`.
- `README.md`: document `many`.
