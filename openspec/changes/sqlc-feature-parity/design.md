# Design

## Context

Generation is a single pass in `internal/gen.go`. The steps run in order: `buildEnums`, then `buildModels`, then `buildQueries` (which also works out row and params structs and reuses a model when its columns match), then `buildModelsTree` and `buildQueryTree`. The last two produce a Python AST (`internal/ast`, protobuf), which `internal/printer` renders.

Types are carried as `pyType{InnerType string, IsArray, IsNull bool}`. `InnerType` is a dotted name such as `datetime.datetime` or `models.Status`, and `importer` (`internal/imports.go`) scans for these names to decide which imports each file needs.

A few facts about the current code shape the plan:
- `Query.Comments` is filled in but never emitted.
- `:copyfrom` returns an error.
- `Column.EmbedTable` and `Column.ArrayDims` are ignored.
- `printClassDef` prints an empty body when there are no fields.
- `AnnAssign.Comment` is printed after `# ` as-is, so a newline in a comment breaks out into code.

sqlc forwards the top-level `options.<plugin-name>` block of `sqlc.yaml` as `GenerateRequest.global_options`. The plugin currently ignores it.

The plugin SDK is already at the latest tag (`plugin-sdk-go` v1.23.0), so no protocol upgrade is needed. Every field used below (`embed_table`, `array_dims`, `global_options`, `comments`) is already in the vendored protobuf.

## Goals / Non-Goals

**Goals:**
- Keep the one-pass pipeline. Add small, focused helpers rather than restructuring `gen.go`.
- Resolve names and types in exactly one place each. This way rename, keyword escaping, and overrides cannot drift between models, row construction, and param dicts.

**Non-Goals:**
- Rewriting the importer or the printer beyond the fixes and the new nodes listed below.
- Matching Go's full override schema (`go_struct_tag`, `unsigned`, the object form of `go_type`). Only `db_type`, `column`, `nullable`, and `py_type` are in scope.

## Decisions

### 1. `pyType` gets `ArrayDims int` in place of `IsArray bool`
`Annotation()` wraps the element type in `List[...]` `ArrayDims` times. `ArrayDims = max(col.ArrayDims, 1 if col.IsArray else 0)`, because older sqlc versions set only `IsArray`. The struct stays comparable with `==`, so model matching in `buildQueries` keeps working.

The importer check for `typing.List` becomes `ArrayDims > 0`.

### 2. Overrides are resolved inside `makePyType`
Config parsing compiles overrides once: column patterns via `path.Match`-style globbing on each dotted segment, and `db_type` matched with and without the `pg_catalog.` prefix. `makePyType(req, conf, col)` checks overrides in this order:
1. `column` entries
2. `db_type` entries whose `nullable` matches `!col.NotNull`
3. the engine type map

The override's `py_type` string becomes `InnerType`.

`makePyType` is also used for parameters, because sqlc sets `Column.Table` and `Column.Name` on parameters it can attribute to a column. That makes column overrides flow to parameters for free.

For imports, the importer already derives modules from dotted `InnerType` names for stdlib types (`datetime`, `uuid`, `decimal`). It will be generalized: any `InnerType` with a dot that is not `models.*` adds `import <prefix>`. The fixed list of stdlib checks is replaced by this rule. Stdlib imports and package (third-party) imports keep separate groups, as today: known stdlib modules go in the std group, and override modules go in the pkg group.

Two forms are accepted:
- A dotted `py_type` with no `py_import` (`my_lib.types.Payload` → `import my_lib.types`). This mirrors Go's `go_type: "pkg/path.Type"`.
- A bare `py_type` plus `py_import` (`UUID` + `uuid` → `from uuid import UUID`). This is what alt-sqlc-gen-python and upstream PR #83 already use, so their configs work unchanged.

The importer already models `from X import Y` (`importSpec.Name`), so the second form becomes a `from` import in the pkg group, or the std group when the module is stdlib.

### 3. Global options merge
`Generate` parses `req.GlobalOptions` into the same `Config` type, keeping only `overrides` and `rename`, and merges as sqlc's Go `opts.Parse` does:
- overrides: `append(local, global...)`. Local entries come first, so they are checked first and win.
- rename: global values are copied over local ones.

The spec pins down both orders.

### 4. One identifier function
`pyIdent(dbName string, conf Config) string` applies `rename` first, then escapes keywords with a trailing `_`, using a hard-coded copy of `keyword.kwlist` for Python 3.12. It is the only source of names for:
- `Field.Name`
- param arg names (`paramName`)
- method names
- model and enum class names (after `modelName`)

`RowNode` and `ArgDictNode` already read `Field.Name` and `QueryValue.Name`, so they pick up the result automatically.

The model-reuse check in `buildQueries` compares `f.Name == columnName(c, i)`. It changes to compare against `pyIdent(columnName(c, i))`, so renamed and escaped models still match.

Soft keywords (`match`, `case`, `type`, `_`) are valid identifiers and are not escaped.

### 5. Embeds
In `buildQueries`, a result column with `EmbedTable != nil` resolves to the model whose `Table` matches the embedded table (by `sdk.SameTableName`). A new `Field.Embed *Struct` records this.

When `Field.Embed` is set:
- `RowNode` emits a nested `models.X(...)` call that uses `len(Embed.Fields)` consecutive row indexes.
- A running index replaces the current `i`.
- The field's annotation is `models.X`.

Model reuse is skipped for any query that has an embed, because the row shape never matches a table. The scalar-return shortcut (`len(query.Columns) == 1`) is also skipped when that single column is an embed, as in Go.

Risk: this assumes sqlc expands each embed into the table's columns, in table order, in the result row. The e2e fixture checks this against real `sqlc` output.

### 6. Copyfrom and batchexec
`:copyfrom` and `:batchexec` share one code path. They always build a params struct (the `qpl == 0` path) and emit the same `executemany` call. The only differences: `:copyfrom` returns `rowcount`, and `:batchexec` returns `None`, which matches alt-sqlc-gen-python's `:batchexec`. The `:copyfrom` method:

```python
def create_authors(self, arg: Sequence[CreateAuthorsParams]) -> int:
    if not arg:
        return 0
    result = self._conn.execute(sqlalchemy.text(CREATE_AUTHORS), [{"p1": a.name, "p2": a.bio} for a in arg])
    return result.rowcount
```

The async version uses `await`. SQLAlchemy runs a list of parameter dicts as `executemany`. `internal/ast` has no list-comprehension nodes (checked: `protos/ast/ast.proto` has no `ListComp`), so they are added along with the other new nodes (decision 14). `ast.pb.go` is regenerated with buf (`buf.gen.yaml` pins `protocolbuffers/go:v1.30.0`). The `ArgDictNode` code is reused with the loop variable `a` as the attribute base.

Alternative considered: an explicit `for` loop that appends to a list. Rejected, because it still needs a new `List` node, and it reads worse in generated code.

Alternative considered: pgcopy or psycopg `copy()`. Rejected, because it is tied to one driver, and the plugin is built on SQLAlchemy's `text()` and is driver-agnostic.

Guard: `Sequence` is imported only when a copyfrom or batch query exists, from `typing`, or from `collections.abc` under modern syntax.

Signature divergence from the fork: alt-sqlc-gen-python's `:batchexec` takes a keyword-only `args: list[Params]`. Here every sequence-taking command takes one positional `arg: Sequence[Params]`, to stay consistent with `:copyfrom` and the single-struct convention in `Query.AddArgs`. The README calls this out for users migrating from the fork.

### 7. Query docstrings
`Query.Comments` becomes the first body statement of each querier method: an `Expr(Constant(strings.Join(comments, "\n")))`. `printFunctionDef` and `printAsyncFunctionDef` need the same docstring handling `printClassDef` already has, printed as `"""..."""`. Leading single spaces left over from `-- ` are trimmed, and no other text changes.

### 8. Printer fixes
- `printClassDef`: when `len(cd.Body) == 0`, print an indented `pass`. This covers params and row classes and a zero-column table.
- Comments on `AnnAssign`: when `Comment` contains `\n`, print the first line inline as today. Each later line prints on its own line at the same indent, prefixed with `# `.
- `printConstant` currently writes string contents unescaped, so a `"` in an enum value, comment, or docstring produces invalid Python. Backslashes and the active quote character are escaped. Docstrings are printed through one helper that always uses `"""` and escapes `\` and `"""`. This replaces the current `""` + constant + `""` trick, which yields five quotes for multi-line comments. `sqlalchemySQL` and the `-- name:` header in `buildQueryTree` currently write the backslash already doubled for Python (they emit `\\:`). Once the printer escapes backslashes, they switch to emitting the runtime text that SQLAlchemy needs (`\:`), and the printer doubles it. This keeps the generated SQL constants byte-identical, and the existing fixtures verify that.

### 9. Enum member names
`pyEnumValueName` stays the sanitizer. `buildEnums` then post-processes the members of each enum:
1. empty → `VALUE_<pos>`
2. leading digit → `VALUE_` + name
3. keyword collision (for example `NONE` is fine, but `None` cannot happen after upper-casing) → no-op, kept only as a check
4. duplicates → `_2`, `_3`, and so on, in value order

### 10. `omit_unused_structs`
This is a port of Go's `filterUnusedStructs`, run after `buildQueries`.
1. Collect a keep-set of `InnerType` names (with `models.` removed) from every query's args, return value, embed fields, and emitted struct fields.
2. Add the enums used by kept models' fields.
3. Filter `models` and `enums` by the keep-set.

The filtered lists feed both `importer` and `tctx`.

### 11. `omit_sqlc_version`
`moduleNode` takes a flag and skips the two version lines.

### 12. Type aliases
This adds the missing Go spellings to the existing `switch` in `postgresql_type.go`. There is no structural change. `json`/`jsonb` keep `Any`, with only the `pg_catalog.` aliases added.

### 13. Tooling
- `mise.toml` pins Go to `1.23`, and adds `buf` and a Python 3.12 with `mypy` for fixture type checks.
- CI (`.github/workflows/*.yml`) moves to sqlc `1.31.1`.
- All fixtures and `examples/` are regenerated with `sqlc generate` so that `sqlc diff` stays clean.

`go.mod` keeps `go 1.19` as its language version. Raising it is not needed, and leaving it avoids forcing downstream builds to change.

### 14. New AST nodes
One proto change, regenerated once, covers every feature that needs new syntax:
- `ListComp` and `Comprehension`, for copyfrom, batchexec, and batchmany
- `With` and `WithItem`, for error wrapping
- `BinOp` with a `BitOr` operator, for `X | None`
- a `Constant.ellipsis` oneof case, for protocol stubs
- `ClassDef.type_params` (a repeated `TypeVar{name, bound}`), printed as `[T: <bound>]` after the class name, for generic queriers

The printer gets a case for each. `If.or_else` already exists in the proto, but `printer.go` never prints it (it would drop an `else:` silently), so the printer gets `else` support, which `:batchone` needs.

Alternative considered: encoding `X | None` or `...` as raw `Name` text. Rejected, because nested annotations (`list[X | None] | None`) then need string building in Go, and that is where precedence bugs hide.

### 15. Type syntax is a render-time switch
`pyType` and `QueryValue` keep a structural form: inner type, `ArrayDims`, `IsNull`. `Annotation()` becomes `Annotation(syntax typeSyntax)`, rendering either `Optional[List[X]]` or `list[X] | None`. It is the only place annotations are built. The generic wrappers used in method returns and params (`Iterator`, `AsyncIterator`, `Sequence`, `Optional`) go through one helper that picks the module (`typing` or `collections.abc`) and spelling. The importer asks the same helper which names it used, instead of keeping its own list of `typing.*` names.

`cast()` targets reuse `Annotation(syntax)`, so they always match the field annotation. With modern syntax, the `cast(str | None, ...)` expressions evaluate `|` at runtime, which is fine on 3.10+, the documented minimum for that option.

### 16. Aware datetimes
The switch lives in `postgresType`: it receives `conf`, and returns `pydantic.AwareDatetime` for `timestamptz` spellings when `emit_aware_datetime` is set. Overrides run before the type map (decision 2), so they still win. `Config` validation runs right after parsing and rejects `emit_aware_datetime` without `emit_pydantic_models`. `pydantic` goes in the pkg import group (third-party). The generalized importer (decision 2) already picks it up from the dotted name.

### 17. Batch one/many
Both commands loop over `arg` in the method body, so the method is a (sync or async) generator, which gives the lazy semantics the spec requires:
- `:batchone`: `row = <execute>.first()`, then `if row is None: yield None` / `else: yield <RowNode>`.
- `:batchmany`: `yield [<RowNode> for row in <execute>]`. Async uses `await self._conn.execute(...)` rather than `stream`, because each item's rows are collected into a list anyway.

The per-item param dict reuses `ArgDictNode` with base `a`, as in decision 6. Both queriers share one body builder that takes an `async bool`, instead of duplicating the switch arms. This also shrinks the existing sync/async duplication in `buildQueryTree` for the arms it touches.

### 18. Queriers accept sessions
The `__init__` annotation and a class-level `_conn: <union>` annotation use:
- sync: `Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]`, or `|` under modern syntax
- async: `sqlalchemy.ext.asyncio.AsyncConnection | sqlalchemy.ext.asyncio.AsyncSession`

`Session.execute` and `AsyncSession.execute`/`stream` accept the same `text()` and parameter arguments, so method bodies do not change.

With `emit_generic_querier`, the same union becomes the bound of a PEP 695 type parameter: `class Querier[T: <union>]`, with `_conn: T` and `conn: T`. The union is spelled by `Annotation(syntax)` (decision 15). The class body is otherwise identical, so the method builder (task 6.1) takes no notice of the flag, and only `querierClassDef`/`asyncQuerierClassDef` branch on it. It is opt-in, not the default, because PEP 695 syntax is a `SyntaxError` before Python 3.12.

Alternative considered: a pre-3.12 `T = TypeVar("T", bound=...)` plus `Generic[T]`. Rejected, because it would add a module-level TypeVar per file and a second spelling to maintain, for a feature whose users are already on 3.12.

### 19. Protocols
`buildQueryTree` builds each querier's `FunctionDef`s first. When `emit_querier_protocol` is set, it derives the protocol from them: same name, args, and returns, and a body of `Expr(Constant(ellipsis))`, with no docstring. Protocol methods for async generators (`:many`, `:batchone`, `:batchmany`) are emitted as a plain `FunctionDef`, not `AsyncFunctionDef`. An async-generator function's call type is `AsyncIterator[T]`, whereas `async def f() -> AsyncIterator[T]` in a stub means `Coroutine[..., AsyncIterator[T]]`, so mypy would reject the real querier. Protocols come before the concrete classes in the file. The concrete classes do not subclass them; conformance is structural. A fixture-level mypy check (a `_check.py` that assigns each querier to its protocol) proves the match.

### 20. Query errors
`errors.py` is a fixed template printed through the AST, so the header and version logic are shared. It matches the fork's classes and its `_wrap_integrity_error` and `_wrap_operational_error` helpers, and adds:
- `constraint_name`, read best-effort from `orig.diag.constraint_name` (psycopg2 and 3) or `orig.__cause__.constraint_name` (asyncpg through SQLAlchemy's adapter), falling back to `None`
- a `_wrap_errors(query_name)` context manager (`contextlib.contextmanager`) that catches `IntegrityError` and `OperationalError` and re-raises through the two helpers `from e`

SQLSTATE is read as `getattr(orig, "pgcode", None) or getattr(orig, "sqlstate", None)`, which covers psycopg2 (`pgcode`) and psycopg 3 and asyncpg (`sqlstate`).

Each querier method body is wrapped in `with errors._wrap_errors("<method>"):`. A sync context manager works inside both `def` and `async def`. Because the `with` encloses the whole body, including `for`/`async for` and `yield`, errors raised while the caller iterates are wrapped too. The fork wraps only the `execute` call. When the consumer stops iterating early, `GeneratorExit` passes through the context manager untouched, because only the two SQLAlchemy types are caught.

Query files add `from <package> import errors` next to `models`. `Generate` fails if any query source maps to `errors.py`.

Alternative considered: try/except in each method, as the fork does. Rejected: it needs `Try` and `ExceptHandler` AST nodes, and it repeats the handler in every method, where one `With` line does the job.

### 21. Existing options are covered by a matrix, not by code changes
`emit_sync_querier`, `emit_async_querier`, `emit_pydantic_models`, and `emit_str_enum` need no new logic. The risk is that new features forget one of them, for example emitting a dataclass for a batch params class in pydantic mode. Two "kitchen-sink" fixtures use one schema and query set that exercises every feature (embed, copyfrom, all batch commands, comments, enums, keywords, and timestamptz):
- `feature_matrix_classic`: both queriers, dataclasses, `(str, enum.Enum)`, typing syntax, protocols and errors on.
- `feature_matrix_modern`: both queriers, pydantic, `StrEnum`, modern syntax, aware datetimes, generic queriers, protocols and errors on.

Each is checked with `py_compile` and `mypy --strict`, plus pytest against Postgres in `examples/`.

## Risks / Trade-offs

- [Output churn from keyword escaping, type aliases, and the sqlc version header across every fixture] → Regenerate fixtures in a separate commit before any feature commits, so reviewers can tell header churn from real changes.
- [The `rowcount` from `executemany` depends on the driver. Some DBAPIs report `-1`] → Document this in the README. The generated code returns what SQLAlchemy reports and does not guess.
- [Existing users with a column named like a keyword get a renamed attribute (`from` → `from_`)] → That code could not have imported before, so nothing that worked breaks. Call it out in the release notes anyway.
- [`cast()` and the widened `_conn` types change every fixture and every user's generated output] → They are annotation-only. Land them in their own commit, after the version-bump regeneration, so their diff is reviewable alone.
- [`constraint_name` extraction depends on driver internals (`diag`, `__cause__`)] → Best-effort with a `None` fallback. The examples' pytest suite exercises psycopg and asyncpg, so a driver change shows up as a failing test, not a crash.
- [`emit_generic_querier` output fails to import on Python < 3.12, and older mypy rejects PEP 695 syntax] → Documented as a requirement of the option. CI runs the fixture checks on Python 3.12 with a current mypy.
- [Protocol and implementation drift] → Both come from the same `FunctionDef` list (decision 19), and mypy checks the fixtures.
- [`:batchone`/`:batchmany` run N round-trips] → Documented. Users who need bulk reads should use `= ANY($1::int[])` with `:many`.
- [Positional mapping for embeds breaks if sqlc changes the expansion order] → The e2e fixture catches it on the next sqlc bump.
- [New AST nodes need buf codegen, and `buf` is not installed locally] → Add `buf` to `mise.toml` tools, and regenerate `ast.pb.go` in its own task and commit.

## Migration Plan

Release this as the next minor version (1.4.0). Every new option defaults to off, so the only upgrade impact is the correctness fixes, type aliases, `cast()` wrappers, and widened `_conn` annotations in generated output, and users see those as a diff when they run `sqlc generate`. To roll back, pin the previous WASM URL and sha256.

