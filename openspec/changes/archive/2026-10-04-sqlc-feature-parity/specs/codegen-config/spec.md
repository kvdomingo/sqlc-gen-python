# Spec Delta

## Purpose

Defines the plugin's generation options: the ones that match sqlc's Go codegen
(type overrides, renames, omitting unused models, omitting the sqlc version) and
the Python output styles (pydantic models, `StrEnum`, modern type syntax, aware
datetimes). Also defines how per-codegen options combine with sqlc's global
options.

## ADDED Requirements

### Requirement: Type overrides by database type

The plugin SHALL accept an `overrides` list. An entry with `db_type` and
`py_type` SHALL replace the Python type of every column and parameter whose
database type matches `db_type`. This SHALL apply in models, params classes, row
classes, and method signatures. Matching SHALL follow sqlc's rules: a `db_type`
given without a schema also matches the `pg_catalog`-qualified name, and SQL
spellings of built-in types (for example `bigint` or `timestamp with time zone`)
match the names sqlc reports (`int8`, `timestamptz`). By default
an entry SHALL apply only to non-null columns. With `nullable: true` it SHALL
apply only to nullable columns. A nullable column that an entry matches SHALL
keep its `Optional[...]` wrapper, and an array column SHALL keep its `List[...]`
wrapper around the override type.

#### Scenario: Override a non-null jsonb column

- **WHEN** `overrides: [{db_type: jsonb, py_type: my_lib.types.Payload}]` is set
  and table `events` has `payload jsonb NOT NULL`
- **THEN** the `Event` model declares `payload: my_lib.types.Payload`

#### Scenario: Override does not apply to nullable columns by default

- **WHEN** the same override is set and a column is `extra jsonb` (nullable)
- **THEN** that column keeps its default type `Optional[Any]`

#### Scenario: Nullable override

- **WHEN** an entry sets `db_type: jsonb`, `py_type: my_lib.types.Payload`,
  `nullable: true` and a column is `extra jsonb`
- **THEN** the column is declared `extra: Optional[my_lib.types.Payload]`

#### Scenario: SQL spelling of a type

- **WHEN** an entry sets `db_type: bigint` and a column is
  `rank bigint NOT NULL`
- **THEN** the column is annotated with the entry's `py_type`

### Requirement: Type overrides by column

An `overrides` entry with `column` and `py_type` SHALL replace the type of that
column. The `column` value SHALL have the form `table.column` or
`schema.table.column`. `*` SHALL match any run of characters within one segment.
When a column matches both a `column` entry and a `db_type` entry, the `column`
entry SHALL win. Parameters that sqlc attributes to a column SHALL get the same
type as the column.

#### Scenario: Column override wins over db_type override

- **WHEN** overrides contain `{db_type: uuid, py_type: my_ids.Id}` and
  `{column: "users.id", py_type: my_ids.UserId}`
- **THEN** `User.id` is `my_ids.UserId`, and every other `uuid` column is
  `my_ids.Id`

#### Scenario: Parameter follows column override

- **WHEN** `users.id` is overridden to `my_ids.UserId` and a query has
  `WHERE id = $1`
- **THEN** the generated method's `id` parameter is annotated `my_ids.UserId`

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
`py_import` is set, `py_type` SHALL be a single identifier, with no dot.

#### Scenario: Import added only where used

- **WHEN** `my_lib.types.Payload` appears in `models.py` and no query file uses
  it
- **THEN** `models.py` contains `import my_lib.types` and the query files do not

#### Scenario: py_import form

- **WHEN** `{column: "authors.id", py_type: UUID, py_import: uuid}` is set
- **THEN** `Author.id` is annotated `UUID`, and `models.py` contains
  `from uuid import UUID`

#### Scenario: Builtin override needs no import

- **WHEN** `{db_type: bytea, py_type: bytes}` is set
- **THEN** `bytea` columns are annotated `bytes` and no import is added for them

#### Scenario: Generic py_type

- **WHEN**
  `{db_type: "double precision", py_type: "my_lib.Box[decimal.Decimal]"}` is set
- **THEN** files that use the type contain `import my_lib` and `import decimal`

### Requirement: Global overrides

The plugin SHALL read `overrides` and `rename` from sqlc's global options for
this plugin as well as from the codegen `options`. For overrides, entries from
the codegen `options` SHALL be checked before global entries. For renames, a key
present in both SHALL use the global value, which matches sqlc's Go codegen.

#### Scenario: Global override applies

- **WHEN** a global `overrides` entry maps `db_type: citext` to `my_lib.CIText`
  and the codegen options have no overrides
- **THEN** `citext` columns are annotated `my_lib.CIText`

### Requirement: Invalid override entries fail generation

An override entry SHALL fail generation with an error that names the entry when
it sets neither `db_type` nor `column`, sets both, has an empty `py_type`, or
sets `py_import` with a dotted `py_type`.

#### Scenario: Missing target

- **WHEN** an entry has only `py_type: str`
- **THEN** `sqlc generate` fails with an error that identifies the invalid
  override

### Requirement: Rename identifiers

The plugin SHALL accept a `rename` map from database identifier to Python
identifier. Keys SHALL match:

- the singularized table name (or the exact name when `emit_exact_table_names`
  is set), for model class names
- the enum type name, for enum class names
- the column name, for field and parameter names
- the enum value, for enum member names

The renamed identifier SHALL appear in every place that refers to it, including
row-to-object construction and parameter dictionaries.

#### Scenario: Rename a field

- **WHEN** `rename: {spotify_url: spotify_link}` is set and table `artists` has
  column `spotify_url`
- **THEN** the `Artist` model declares `spotify_link`, and queries that return
  it build `Artist(..., spotify_link=cast(Optional[str], row[n]))`

#### Scenario: Rename a model

- **WHEN** `rename: {person: Human}` is set for table `people`
- **THEN** the model class is named `Human`, and query return types refer to
  `models.Human`

### Requirement: Omit unused structs

When `omit_unused_structs` is true, `models.py` SHALL contain only the enums and
models that some query's parameters, result columns, or embedded tables refer
to, directly or through a field type. When it is false or unset, output SHALL be
unchanged.

#### Scenario: Unused table dropped

- **WHEN** the schema has tables `authors` and `audit_log`, only `authors` is
  queried, and `omit_unused_structs: true`
- **THEN** `models.py` defines `Author` and does not define `AuditLog`

#### Scenario: Enum kept through a model field

- **WHEN** an enum `status` is used only as a column of a queried table
- **THEN** the `Status` enum is still emitted

### Requirement: Omit sqlc version

When `omit_sqlc_version` is true, generated files SHALL NOT contain the
`# versions:` and `#   sqlc <version>` header lines. The other header lines
SHALL stay.

#### Scenario: Header without version

- **WHEN** `omit_sqlc_version: true`
- **THEN** each generated file starts with
  `# Code generated by sqlc. DO NOT EDIT.` and contains no line that mentions
  the sqlc version

### Requirement: Pydantic models

With `emit_pydantic_models: true`, every model, params class, and row class
SHALL subclass `pydantic.BaseModel`, and the file SHALL `import pydantic`.
Otherwise these classes SHALL be `@dataclasses.dataclass()` classes. The choice
SHALL apply the same way to classes produced by every feature in this change:
embed row classes, `:copyfrom` and batch params, and renamed or keyword-escaped
fields.

#### Scenario: Pydantic embed row

- **WHEN** `emit_pydantic_models: true` and a query uses `sqlc.embed(authors)`
- **THEN** the row class is a `pydantic.BaseModel` whose `authors` field is
  typed `models.Author`, and constructing it from a query result passes pydantic
  validation

#### Scenario: Default dataclasses

- **WHEN** `emit_pydantic_models` is unset
- **THEN** `models.py` imports `dataclasses` and decorates every model with
  `@dataclasses.dataclass()`

### Requirement: StrEnum enums

With `emit_str_enum: true`, every enum class SHALL subclass `enum.StrEnum`.
Otherwise every enum class SHALL subclass `(str, enum.Enum)`. Enum member
naming, renames, and value escaping SHALL behave the same either way.

#### Scenario: StrEnum with sanitized members

- **WHEN** `emit_str_enum: true` and an enum has values `in-progress` and `=`
- **THEN** the class is `class Status(enum.StrEnum)` with members
  `IN_PROGRESS = "in-progress"` and `VALUE_2 = "="`

### Requirement: Modern type syntax

With `emit_modern_types: true`, generated annotations SHALL use Python 3.10+
syntax:

- `X | None` instead of `Optional[X]`
- `list[X]` instead of `List[X]`
- `Iterator`, `AsyncIterator`, and `Sequence` imported from `collections.abc`
  instead of `typing`
- `Union` replaced with `|` everywhere, including the querier `_conn`
  annotations

`Optional` and `List` SHALL then not be imported. `typing` SHALL still supply
`Any`, `cast`, and `Protocol` when they are used. With the option off,
annotations SHALL use the current `typing` forms. The option SHALL apply to
models, params and row classes, querier signatures, protocols, `cast` targets,
and `errors.py`.

#### Scenario: Nullable array field

- **WHEN** `emit_modern_types: true` and a column is `tags text[]` (nullable)
- **THEN** it is annotated `list[str] | None`, and the file does not import
  `Optional` or `List`

#### Scenario: Iterator import

- **WHEN** `emit_modern_types: true` and a sync `:many` query exists
- **THEN** the query file contains `from collections.abc import Iterator`

### Requirement: Aware datetimes

With `emit_aware_datetime: true`, `timestamptz` columns and parameters (all
spellings listed in the type-mapping capability) SHALL be annotated
`pydantic.AwareDatetime`. Other types SHALL be unchanged; `timestamp` without
time zone stays `datetime.datetime`. Nullability and array wrappers SHALL apply
as usual. Setting `emit_aware_datetime: true` without
`emit_pydantic_models: true` SHALL fail generation, with an error saying that
`emit_aware_datetime` requires `emit_pydantic_models`. A matching override SHALL
take precedence over this option.

#### Scenario: Aware timestamp field

- **WHEN** `emit_aware_datetime` and `emit_pydantic_models` are true and a
  column is `created_at timestamptz NOT NULL`
- **THEN** it is annotated `pydantic.AwareDatetime`, and constructing the model
  with a naive `datetime` raises a pydantic `ValidationError`

#### Scenario: Requires pydantic

- **WHEN** `emit_aware_datetime: true` is set without `emit_pydantic_models`
- **THEN** `sqlc generate` fails with an error naming both options
