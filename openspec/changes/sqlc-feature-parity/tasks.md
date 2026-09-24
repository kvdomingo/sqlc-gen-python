# Tasks

## 1. Tooling and baseline

- [x] 1.1 In `mise.toml`, pin Go to 1.23 and add `buf` and Python 3.12. Add `mypy>=1.12`, `pydantic>=2`, `sqlalchemy>=2`, `psycopg`, and `asyncpg` to `examples/requirements.txt`. Verify `mise install && make all` produces `bin/sqlc-gen-python.wasm`
- [x] 1.2 Bump the sqlc version to `1.31.1` in `.github/workflows/*.yml`, and verify with `grep -rn sqlc-version .github/`
- [x] 1.3 With no code changes, regenerate every `internal/endtoend/testdata/*` fixture and `examples/` using sqlc v1.31.1 (`sqlc generate` in each dir). Commit it on its own and verify `make test` passes and `cd examples && sqlc diff` is clean

## 2. Printer and AST foundations

- [x] 2.1 Escape `\` and the active quote character in `printConstant`. Switch `sqlalchemySQL` and the `-- name:` header in `buildQueryTree` to emit runtime text (single `\:`). Verify `make test` shows no fixture diffs and a new `printer_test.go` case round-trips `say "hi"` and `a\b`
- [x] 2.2 Add one triple-quoted docstring helper and use it in `printClassDef`, `printFunctionDef`, and `printAsyncFunctionDef`. Verify with a printer test for a multi-line class docstring and a function docstring
- [x] 2.3 Print `pass` for a `ClassDef` with an empty body, and verify with a printer test
- [x] 2.4 Print multi-line `AnnAssign.Comment` values as one `# ` line per source line at the field's indent, and verify with a printer test
- [x] 2.5 Add `ListComp`, `Comprehension`, `With`, `WithItem`, `BinOp` (with `BitOr`), a `Constant.ellipsis` case, and `ClassDef.type_params` (`TypeVar{name, bound}`) to `protos/ast/ast.proto`, and regenerate `internal/ast/ast.pb.go` with `buf generate`. Verify `go build ./...` succeeds
- [x] 2.6 Add printer support for each node from 2.5 and for `If.or_else` (`else:`). Verify with printer tests that render `[{"p1": a.x} for a in arg]`, `with errors._wrap_errors("q"):` plus a body, `list[int] | None`, `def f(self) -> int: ...`, `class Q[T: A | B]:`, and an `if`/`else`

## 3. Identifiers and enums

- [x] 3.1 Add the `pyIdent` helper (rename lookup, then keyword escaping using the Python 3.12 hard keywords) and unit tests for `from`→`from_`, `match` unchanged, and rename applied before escaping
- [x] 3.2 Route field names, param names, method names, and model and enum class names through `pyIdent`, and compare against the escaped name in the model-reuse check. Verify with a new fixture `testdata/python_keywords` (a column `from`, a param `class`) whose generated modules pass `python -m py_compile`
- [x] 3.3 Post-process enum member names (`VALUE_<n>` for empty names, a `VALUE_` prefix for a leading digit, `_2`… for duplicates). Verify with a new fixture `testdata/enum_member_names` that covers `=`, `<>`, `in-progress`/`in_progress`, `1st`, and `say "hi"`, and extend the existing `emit_str_enum` fixture with the same values
- [x] 3.4 Add a fixture `testdata/empty_params_class` (`query_parameter_limit: 0` with a no-param query) and a fixture `testdata/multiline_comments` (multi-line table and column comments). Verify `make test` passes and the outputs `py_compile`

## 4. Types and annotation syntax

- [x] 4.1 Change `pyType.IsArray` to `ArrayDims int`, and emit nested list wrappers. Verify with a new fixture `testdata/multidim_arrays` (`int[][]` gives `List[List[int]]`, and `text[]` is unchanged)
- [x] 4.2 Add the missing PostgreSQL aliases (`varchar`, `character varying`, `bpchar`, `character`, `name`, `time`/`timetz`/`timestamp` and the `with/without time zone` spellings, `pg_catalog.json`/`jsonb`) to `postgresql_type.go`. Verify with a new fixture `testdata/postgresql_type_aliases` that shows no `Any` for them
- [x] 4.3 Make `Annotation(syntax)` the only annotation builder, add the generic-wrapper helper (`Optional`/`List`/`Iterator`/`AsyncIterator`/`Sequence` → typing or modern forms, `BinOp` for `|`), and have the importer ask that helper which names were used. Verify existing fixtures show no diff with `emit_modern_types` off
- [ ] 4.4 Add `emit_modern_types`, and verify with a new fixture `testdata/emit_modern_types` (`list[str] | None`, `from collections.abc import Iterator, AsyncIterator`, and no `Optional`/`List` imports)
- [ ] 4.5 Add `emit_aware_datetime` in `postgresType` (only the `timestamptz` spellings), with the config check that requires `emit_pydantic_models`. Verify with a new fixture `testdata/emit_aware_datetime` (`timestamptz` → `pydantic.AwareDatetime`, `timestamp` unchanged) and a fixture `testdata/emit_aware_datetime_no_pydantic` whose `stderr.txt` holds the expected error

## 5. Config options

- [ ] 5.1 Add `Overrides` (with `db_type`, `column`, `nullable`, `py_type`, and `py_import`), `Rename`, `OmitUnusedStructs`, `OmitSqlcVersion`, `EmitModernTypes`, `EmitGenericQuerier`, `EmitAwareDatetime`, `EmitQuerierProtocol`, and `EmitQueryErrors` to `Config`. Parse `req.GlobalOptions` and merge it (local overrides first; global rename values win). Validate override entries (exactly one of `db_type`/`column`, a non-empty `py_type`, and no dotted `py_type` with `py_import`). Verify with Go unit tests for merge order and each validation error
- [ ] 5.2 Resolve overrides in `makePyType` (`column` entries with `*` globbing first, then `db_type` with `nullable`, then the type map) for both columns and params. Verify with a new fixture `testdata/overrides` that covers the db_type, nullable, column-wins, and param-follows-column scenarios
- [ ] 5.3 Generalize the importer so every dotted non-`models.` `InnerType` adds `import <module>`, and every `py_import` override adds `from <py_import> import <py_type>`, both only in the files that use them and in the right std/pkg group. Verify the `overrides` fixture covers both forms (`import my_lib.types` and `from uuid import UUID`), and that existing fixtures show no import diffs
- [ ] 5.4 Add a fixture `testdata/overrides_global` that uses the top-level `options: {py: {overrides: ...}}` in `sqlc.yaml`, and verify the global override applies
- [ ] 5.5 Apply `rename` to model names, enum names, fields, and enum members. Verify with a new fixture `testdata/rename` that covers a renamed field and a renamed model referenced from a query's return type
- [ ] 5.6 Port `filterUnusedStructs` behind `omit_unused_structs`, run after `buildQueries` and before building the importer and context. Verify with a new fixture `testdata/omit_unused_structs` (an unused table dropped, and an enum kept through a model field)
- [ ] 5.7 Honor `omit_sqlc_version` in `moduleNode`. Verify with a new fixture `testdata/omit_sqlc_version` whose files have no `sqlc v` line

## 6. Querier code generation

- [x] 6.1 Refactor the sync and async querier bodies in `buildQueryTree` into one builder with an `async` flag, for all existing commands. Verify every existing fixture regenerates with no diff
- [ ] 6.2 Widen the querier connection types to `Connection | Session` and `AsyncConnection | AsyncSession`, add the class-level `_conn` annotation, and wrap every row value in `cast(<annotation>, row[i])`. Land it as its own commit that regenerates all fixtures and `examples/`, and verify `make test` and `cd examples && sqlc diff` pass
- [ ] 6.3 Add `emit_generic_querier`: emit `class Querier[T: <union>]` and `class AsyncQuerier[T: <union>]` with `_conn: T` and `conn: T`, with the bound in the active syntax. Verify with a new fixture `testdata/emit_generic_querier` (in typing and modern syntax) plus a `_check.py` where `reveal_type(Querier(session)._conn)` is `Session` and a protocol assignment passes under `mypy --strict`
- [ ] 6.4 Emit `Query.Comments` as the docstring of sync and async querier methods. Verify with a new fixture `testdata/query_comments`, and check that uncommented queries have no docstring
- [ ] 6.5 Support `sqlc.embed()`: add `Field.Embed`, build nested `models.X(...)` from consecutive row indexes using a running index (with `cast` on each value), and skip model reuse and the scalar shortcut for embeds. Verify with a new fixture `testdata/sqlc_embed` (an embed plus a plain column, and two embeds) against real sqlc v1.31.1 output, in both dataclass and pydantic mode
- [ ] 6.6 Support `:copyfrom` and `:batchexec` on the shared `executemany` path: force a params struct, take `arg: Sequence[Params]`, return early on empty input, and return `rowcount` or `None` respectively. Verify with new fixtures `testdata/copyfrom` and `testdata/batch_exec`, and check that the old "not implemented" error and the `unknown cmd` panic are gone
- [ ] 6.7 Support `:batchone` (`if`/`else` yield per item) and `:batchmany` (yield a list comprehension per item) as sync and async generators. Verify with a new fixture `testdata/batch_one_many` in both typing and modern syntax
- [ ] 6.8 Emit `QuerierProtocol` and `AsyncQuerierProtocol` behind `emit_querier_protocol`, derived from the built `FunctionDef`s, with async-generator methods as plain `def`. Verify with a new fixture `testdata/emit_querier_protocol` plus a `_check.py` that assigns each querier to its protocol, and that `mypy --strict` passes on it
- [ ] 6.9 Generate `errors.py` behind `emit_query_errors` (the fork's classes, `_wrap_integrity_error`/`_wrap_operational_error`, `constraint_name`, and the `_wrap_errors` context manager), wrap every querier method body in `with errors._wrap_errors("<method>"):`, import `errors` in query files, and fail when a query file maps to `errors.py`. Verify with new fixtures `testdata/emit_query_errors` (in typing and modern syntax) and `testdata/emit_query_errors_conflict` (with `stderr.txt`)

## 7. Integration, docs, release

- [ ] 7.1 Add the fixtures `testdata/feature_matrix_classic` and `testdata/feature_matrix_modern` (design decision 21), each with a `_check.py`. Verify `make test` passes, and `mypy --strict` passes on both
- [ ] 7.2 Move the `db` CI job from Python 3.9 to 3.12 (needed to parse the PEP 695 fixtures), and add a step that runs `python -m py_compile` over every `internal/endtoend/testdata/**/*.py` and `mypy --strict` over the two matrix fixtures, and verify it passes locally
- [ ] 7.3 Extend `examples/` with `:copyfrom`, `:batchexec`, `:batchone`, `:batchmany`, and `sqlc.embed()` queries, `emit_query_errors`, and `emit_querier_protocol`. Add pytest cases against Postgres for:
  - lazy batch results
  - unique, foreign-key, and not-null violations raising the typed errors, with `constraint_name`, through psycopg (sync) and asyncpg (async)
  - a `Session`-backed `Querier`
  - a fake that satisfies `QuerierProtocol`

  Verify `pytest` passes against a local `postgres` container
- [ ] 7.4 Add README sections for:
  - `overrides` (both forms, global `options`, and `nullable`), `rename`, `omit_unused_structs`, and `omit_sqlc_version`
  - `emit_modern_types` (Python 3.10+) and `emit_aware_datetime` (requires pydantic 2)
  - `emit_generic_querier` (Python 3.12+), `emit_querier_protocol`, and `emit_query_errors`
  - `:copyfrom` (driver-dependent `rowcount`), the batch commands (N round-trips for `:batchone`/`:batchmany`), and `sqlc.embed()`
  - Session support
  - a "Migrating from alt-sqlc-gen-python" note (the `:batchexec` signature, and the opt-in modern syntax, aware datetimes, and generic queriers)

  Verify every new option name appears in the README
- [ ] 7.5 Run `make test`, `go vet ./...`, and `cd examples && sqlc diff` for a final check, and confirm all pass
