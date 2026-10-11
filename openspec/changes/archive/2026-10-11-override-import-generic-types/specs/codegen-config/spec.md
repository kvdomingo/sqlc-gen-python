# Spec Delta

## MODIFIED Requirements

### Requirement: Override imports

For every dotted name in `py_type`, the plugin SHALL add `import <module>` to
every generated file that uses the type, where `<module>` is the name without
its last segment. This SHALL also apply to dotted names inside a generic type
such as `dict[str, decimal.Decimal]`. The annotation SHALL be `py_type`
unchanged. Names without a dot (for example `str`, `bytes` or `dict`) SHALL NOT
be imported.

An entry MAY also set `py_import`, as alt-sqlc-gen-python and upstream PR #83
do. Then the plugin SHALL add `from <py_import> import <py_type>` to every file
that uses the type, and the annotation SHALL be the bare `py_type`. When
`py_import` is set, `py_type` SHALL be a single Python identifier. Otherwise the
plugin SHALL reject the entry with an error that names the entry and suggests
`many: true` for a list.

#### Scenario: Import added only where used

- **WHEN** `my_lib.types.Payload` appears in `models.py` and no query file uses
  it
- **THEN** `models.py` contains `import my_lib.types` and the query files do not

#### Scenario: py_import form

- **WHEN** `{column: "authors.id", py_type: UUID, py_import: uuid}` is set
- **THEN** `Author.id` is annotated `UUID`, and `models.py` contains
  `from uuid import UUID`

#### Scenario: py_import with a non-identifier py_type

- **WHEN**
  `{db_type: jsonb, py_type: "list[DomainEntity]", py_import: app.models}` is
  set
- **THEN** the plugin fails with an error that names the entry and mentions
  `many`

#### Scenario: Builtin override needs no import

- **WHEN** `{db_type: bytea, py_type: bytes}` is set
- **THEN** `bytea` columns are annotated `bytes` and no import is added for them

#### Scenario: Generic py_type

- **WHEN**
  `{db_type: "double precision", py_type: "my_lib.Box[decimal.Decimal]"}` is set
- **THEN** files that use the type contain `import my_lib` and `import decimal`

## ADDED Requirements

### Requirement: Override list wrapping

An `overrides` entry MAY set `many`, a boolean that defaults to `false`. When
`many` is `true`, the plugin SHALL annotate every column and parameter that the
entry matches as a list of the entry's type, spelled `list[T]`, or `List[T]`
when `emit_modern_types` is off. Imports for `py_type` SHALL be the same as
without `many`. For an array column, the list from `many` SHALL be the
innermost list. For a nullable column, the list SHALL be wrapped in the
optional type.

#### Scenario: List of an imported type

- **WHEN**
  `{db_type: domain_entity, py_type: DomainEntity, py_import: app.my_module.models.domain, many: true}`
  is set, `emit_modern_types` is on, and a model has
  `domain_entity domain_entity NOT NULL`
- **THEN** the field is annotated `list[DomainEntity]`, and the file contains
  `from app.my_module.models.domain import DomainEntity`

#### Scenario: Legacy list spelling

- **WHEN** the same entry is set and `emit_modern_types` is off
- **THEN** the field is annotated `List[DomainEntity]` and the file imports
  `List` from `typing`

#### Scenario: Array column

- **WHEN** an entry with `many: true` and `py_type: my_lib.Tag` matches a
  column `tags domain_entity[] NOT NULL`
- **THEN** the column is annotated `list[list[my_lib.Tag]]`

#### Scenario: Nullable column

- **WHEN** an entry with `nullable: true`, `many: true` and `py_type: my_lib.Tag`
  matches a nullable column, and `emit_modern_types` is on
- **THEN** the column is annotated `list[my_lib.Tag] | None`

#### Scenario: many defaults to false

- **WHEN** an entry does not set `many`
- **THEN** matched columns are annotated with `py_type` and no list
