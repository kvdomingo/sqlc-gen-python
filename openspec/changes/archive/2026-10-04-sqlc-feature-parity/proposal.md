# Proposal

## Why

sqlc-gen-python has not tracked sqlc since v1.28.0. sqlc's own Go codegen (at
v1.31.1) and the plugin protocol now carry features the Python plugin ignores:
type overrides, renames, `sqlc.embed()`, `:copyfrom`, query comments, and more.
Some inputs also produce Python that will not import, such as a column named
`from` or an empty params class. Users hit these walls and work around them, or
wait on unmerged upstream PRs (#83, #89, #92, #95, #100, #103, #105).

## What Changes

**Config parity with sqlc's Go codegen**

- New `overrides` option: map a `db_type` or a `column` to a `py_type`, given
  either as a dotted Python path or as `py_type` plus `py_import` (the form
  alt-sqlc-gen-python uses). The plugin adds the import. It also reads overrides
  from sqlc's global `options`.
- New `rename` option: map a database identifier to the Python name used for a
  class, field, or enum member.
- New `omit_unused_structs` option: leave out enums and models that no query
  references.
- New `omit_sqlc_version` option: leave the sqlc version out of the file header.

**New opt-in output styles**

- New `emit_modern_types` option: emit Python 3.10+ annotations (`X | None`,
  `list[X]`, and `collections.abc` iterators and sequences) instead of
  `Optional`, `List`, and the `typing` aliases.
- New `emit_aware_datetime` option: annotate `timestamptz` columns and
  parameters as `pydantic.AwareDatetime`. It requires `emit_pydantic_models`.
- New `emit_generic_querier` option: emit `Querier` and `AsyncQuerier` as PEP
  695 generic classes (`class Querier[_ConnT: Connection | Session]`) with
  `_conn: _ConnT`, so `Querier(session)._conn` is typed as `Session`. It
  requires Python 3.12+.
- New `emit_querier_protocol` option: emit `QuerierProtocol` and
  `AsyncQuerierProtocol` (`typing.Protocol`) classes. They mirror the generated
  queriers, so a fake can stand in for one in tests.
- New `emit_query_errors` option: generate an `errors.py` module of typed
  exceptions, all subclasses of `QueryError`. Querier methods re-raise
  constraint violations (`UniqueViolationError`, `ForeignKeyViolationError`,
  `CheckViolationError`, `NotNullViolationError`, `ExclusionViolationError`) and
  operational failures (`StatementTimeoutError`, `DeadlockError`,
  `SerializationError`) as the matching type, carrying `query_name`, `cause`,
  and `constraint_name`. The names match alt-sqlc-gen-python's option of the
  same name, so code written against that fork keeps working.

**Query features**

- `sqlc.embed(table)`: the row class gets one field typed as the table's model,
  filled from that table's columns.
- Comments on a query (the `--` lines above `-- name:`) become the docstring of
  each querier method.
- `:copyfrom`: emits a method that takes a sequence of params objects and runs
  one `executemany`. It returns the row count. Today this command fails
  generation.
- `:batchexec`: emits a method that takes a sequence of params objects and
  writes them all with one `executemany`.
- `:batchone`, `:batchmany`: emit a method that takes a sequence of params
  objects and runs the statement once per item. Results come back lazily, one
  per input, in input order.
- Today all three batch commands fail generation with a panic.
- Queriers also accept `sqlalchemy.orm.Session` and `AsyncSession`, not only
  connections, and declare a typed `_conn`.
- Values read from result rows are wrapped in `typing.cast(<type>, row[i])`, so
  strict type checkers accept the generated code.
- Multi-dimensional array columns (`int[][]`) emit nested `List[...]` types.

**Existing options keep working with every new feature**

- `emit_sync_querier`, `emit_async_querier`, `emit_pydantic_models`, and
  `emit_str_enum` get specs and fixtures. Every new query feature (embed,
  copyfrom, batch, docstrings, typed errors, protocols) must generate valid,
  type-consistent code under each of them.

**Correctness fixes (generated output may change)**

- Identifiers that are Python keywords (`from`, `class`, …) get a trailing `_`.
- A class with no fields gets a `pass` body instead of an empty body, which is a
  syntax error.
- Enum values that sanitize to an empty, duplicate, or digit-leading name get a
  valid unique member name.
- Multi-line table and column comments print as valid multi-line Python
  comments.
- String literals (enum values, docstrings) escape backslashes and quotes, so a
  `"` in an enum value or comment no longer breaks the module.
- PostgreSQL type aliases that sqlc's Go codegen maps but this plugin types as
  `Any` get real types: `varchar`, `character varying`, `bpchar`, `character`,
  `name`, `timestamp`, `time`, `timetz`, `pg_catalog.json`/`jsonb`, and the
  `… with/without time zone` spellings.

**Tooling**

- CI, examples, and test fixtures move to sqlc v1.31.1. Local Go moves to 1.23
  because `GOOS=wasip1` does not build on Go 1.19.

Behavior that is not a bug fix sits behind an option and is off by default. Two
exceptions, both annotation-only with no runtime effect: the widened querier
connection types and the `cast()` wrappers. Bug fixes and type-map gains change
generated output for affected schemas. Code that relied on those outputs (for
example, `Any` for a `varchar` column) will see different annotations.

## Non-goals

- MySQL and SQLite engine support, and `sqlc.slice()`.
- Real driver pipelining for `:batchone`/`:batchmany`. Their items run one
  statement at a time over the given connection.
- Always-on modern syntax, aware datetimes, and PEP 695 generic queriers, as
  alt-sqlc-gen-python does them. Here each is opt-in, so Python 3.9 users see no
  change.
- PostgreSQL range and multirange types. They stay `Any`, and overrides can type
  them.
- Changing `bytea` from `memoryview` to `bytes`, or changing async `:many` from
  `stream` to `execute`.

## Capabilities

### New Capabilities

- `codegen-config`: the plugin options `overrides`, `rename`,
  `omit_unused_structs`, `omit_sqlc_version`, `emit_pydantic_models`,
  `emit_str_enum`, `emit_modern_types`, and `emit_aware_datetime`, and how
  global options merge with them.
- `query-codegen`: how queries become Python: sync and async queriers, querier
  protocols, typed error wrapping, embedded tables, query docstrings,
  `:copyfrom`, batch commands, and multi-dimensional arrays.
- `generated-code-validity`: guarantees that generated modules are valid Python
  (identifiers, empty classes, enum member names, comments).
- `postgresql-type-mapping`: the PostgreSQL-to-Python type map, including the
  added aliases and the aware-datetime mapping.

### Modified Capabilities
<!-- None: openspec/specs/ is empty. -->

## Impact

- Code: `internal/config.go`, `internal/gen.go`, `internal/imports.go`,
  `internal/postgresql_type.go`, `internal/printer/printer.go`, and
  `protos/ast/ast.proto` (new `ListComp`, `With`, `BinOp`, and ellipsis nodes).
  New helper files as needed.
- Generated package: can now include an `errors.py` module (with
  `emit_query_errors`). A query file named `errors.sql` then conflicts with it
  and fails generation.
- Runtime requirements: `emit_modern_types` needs Python 3.10+,
  `emit_generic_querier` needs Python 3.12+ (and mypy 1.12+ or a recent pyright
  to type-check), and `emit_aware_datetime` needs pydantic 2+.
- Tests: new `internal/endtoend/testdata/*` fixtures for each feature. Existing
  fixture headers get regenerated for sqlc v1.31.1.
- Docs: new README sections for each option.
- Tooling: `.github/workflows/*.yml` (sqlc 1.31.1), `mise.toml` (Go 1.23), and
  regenerated `examples/`.
- Compatibility: generated output changes as listed under the correctness fixes.
  The config stays backward compatible, because every new option defaults to off
  or empty.
