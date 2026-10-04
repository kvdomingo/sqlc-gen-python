# sqlc-gen-python

A [sqlc](https://sqlc.dev) plugin that generates typed Python (SQLAlchemy +
dataclasses or pydantic) from PostgreSQL queries.

## Fork notes

This is a fork of [sqlc-gen-python][upstream], brought up to parity with sqlc
v1.31.1. I started this fork because the official sqlc-gen-python and its forks
are no longer receiving active contributions at the time of writing.

Compared to upstream it adds:

- `sqlc.embed()`, query comments as docstrings, multi-dimensional arrays
- `:copyfrom`, `:batchexec`, `:batchone` and `:batchmany`
- Type overrides by database type or column, and renames, including from
  sqlc's global `options`
- Queriers that accept SQLAlchemy `Session`/`AsyncSession`, with a typed
  `_conn` and `typing.cast` on row values, so output passes `mypy --strict`
- Opt-in output styles: modern syntax (`X | None`, `list[X]`), PEP 695
  generic queriers, `pydantic.AwareDatetime`, `typing.Protocol` querier
  interfaces, and typed errors for constraint violations
- Fixes: Python keywords are escaped (`from` → `from_`), enum member names
  are always valid and unique, string literals and multi-line comments are
  escaped, and `varchar`/`timestamp`/`time` and friends no longer map to `Any`

Every new behaviour is behind an option and off by default, so generated code
for an existing config only changes where upstream produced invalid or untyped
output. Only PostgreSQL is supported.

## Usage

Each GitHub release publishes `alt-sqlc-gen-python.wasm` and a matching
`alt-sqlc-gen-python.wasm.sha256`. Pin both in `sqlc.yaml`:

```yaml
version: "2"
plugins:
  - name: py
    wasm:
      url: https://github.com/kvdomingo/sqlc-gen-python/releases/download/v1.0.1/alt-sqlc-gen-python.wasm
      sha256: 3e9767af784b728ec9bd4499619721e5dabdebeb21cf3c7fe86a045d865584d1
sql:
  - schema: "schema.sql"
    queries: "query.sql"
    engine: postgresql
    codegen:
      - out: src/authors
        plugin: py
        options:
          package: authors
          emit_sync_querier: true
          emit_async_querier: true
```

Generated code needs SQLAlchemy and a PostgreSQL driver (psycopg 2/3 or
asyncpg). Some options raise the minimum Python version; see each option.

### Options

| Option | Default | Effect |
|---|---|---|
| `package` | | Package the query files import `models` (and `errors`) from |
| `emit_sync_querier` | `false` | Emit `Querier` |
| `emit_async_querier` | `false` | Emit `AsyncQuerier` |
| `emit_pydantic_models` | `false` | `pydantic.BaseModel` instead of dataclasses |
| `emit_str_enum` | `false` | `enum.StrEnum` (Python 3.11+) instead of `(str, enum.Enum)` |
| `emit_exact_table_names` | `false` | Don't singularize table names for model classes |
| `inflection_exclude_table_names` | `[]` | Table names never singularized |
| `query_parameter_limit` | `4` | Params above this become a params class; `0` always uses one |
| `emit_modern_types` | `false` | `X \| None`, `list[X]`, `collections.abc` (Python 3.10+) |
| `emit_generic_querier` | `false` | PEP 695 generic queriers (Python 3.12+) |
| `emit_aware_datetime` | `false` | `timestamptz` as `pydantic.AwareDatetime` (needs `emit_pydantic_models`) |
| `emit_querier_protocol` | `false` | `QuerierProtocol` / `AsyncQuerierProtocol` |
| `emit_query_errors` | `false` | Generate `errors.py` and raise typed errors |
| `overrides` | `[]` | Python types for database types or columns |
| `rename` | `{}` | Python names for database identifiers |
| `omit_unused_structs` | `false` | Drop models and enums no query uses |
| `omit_sqlc_version` | `false` | Leave the sqlc version out of file headers |

### Sync and Async Queriers

Options: `emit_sync_querier`, `emit_async_querier`

These generate `Querier` and/or `AsyncQuerier` classes that wrap a SQLAlchemy
connection and expose a method for each query.

- `Querier` accepts `sqlalchemy.engine.Connection` or `sqlalchemy.orm.Session`
- `AsyncQuerier` accepts `sqlalchemy.ext.asyncio.AsyncConnection` or
  `sqlalchemy.ext.asyncio.AsyncSession`

```py
with Session(engine) as session:
    querier = query.Querier(session)
    author = querier.get_author(id=1)
```

The query command determines the method signature:

| Command | Sync return type | Async return type |
|---|---|---|
| `:one` | `Optional[Row]` | `Optional[Row]` |
| `:many` | `Iterator[Row]` | `AsyncIterator[Row]` |
| `:exec` | `None` | `None` |
| `:execrows` | `int` | `int` |
| `:execresult` | `sqlalchemy.engine.Result[Any]` | `sqlalchemy.engine.Result[Any]` |
| `:copyfrom` | `int` | `int` |
| `:batchexec` | `None` | `None` |
| `:batchone` | `Iterator[Optional[Row]]` | `AsyncIterator[Optional[Row]]` |
| `:batchmany` | `Iterator[List[Row]]` | `AsyncIterator[List[Row]]` |

`Row` is a reused model, a generated row class, or a scalar type. Comments
above a query's `-- name:` line become the method's docstring.

Generated code with both options enabled (default syntax):

```py
class Querier:
    _conn: Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]

    def __init__(self, conn: Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]):
        self._conn = conn

    def get_user(self, *, id: int) -> Optional[models.User]:
        """Fetch a user by id."""
        row = self._conn.execute(sqlalchemy.text(GET_USER), {"p1": id}).first()
        if row is None:
            return None
        return models.User(
            id=cast(int, row[0]),
            name=cast(str, row[1]),
        )

    def list_users(self) -> Iterator[models.User]:
        result = self._conn.execute(sqlalchemy.text(LIST_USERS))
        for row in result:
            yield models.User(
                id=cast(int, row[0]),
                name=cast(str, row[1]),
            )


class AsyncQuerier:
    _conn: Union[sqlalchemy.ext.asyncio.AsyncConnection, sqlalchemy.ext.asyncio.AsyncSession]

    def __init__(self, conn: Union[sqlalchemy.ext.asyncio.AsyncConnection, sqlalchemy.ext.asyncio.AsyncSession]):
        self._conn = conn

    async def list_users(self) -> AsyncIterator[models.User]:
        result = await self._conn.stream(sqlalchemy.text(LIST_USERS))
        async for row in result:
            yield models.User(
                id=cast(int, row[0]),
                name=cast(str, row[1]),
            )
```

### Generic Queriers

Option: `emit_generic_querier` (Python 3.12+)

Emits the queriers as [PEP 695](https://peps.python.org/pep-0695/) generics,
so the connection type you pass in is preserved: `Querier(session)._conn` is
typed as `Session`. Type-checking the output needs mypy 1.12+ or a recent
pyright.

```py
class Querier[_ConnT: Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]]:
    _conn: _ConnT

    def __init__(self, conn: _ConnT):
        self._conn = conn
```

### Bulk and Batch Commands

Commands: `:copyfrom`, `:batchexec`, `:batchone`, `:batchmany`

All four always generate a params class, whatever `query_parameter_limit` is,
and take one positional `Sequence` of it. An empty sequence runs nothing.

- `:copyfrom` runs a single `executemany` and returns the row count, as
  reported by the driver (some report `-1`).
- `:batchexec` runs a single `executemany` and returns `None`.
- `:batchone` and `:batchmany` return a generator that runs the statement
  once per item, in order, as you iterate. `:batchone` yields the first row or
  `None` per item; `:batchmany` yields a list of rows per item. That is one
  round-trip per item; for bulk reads prefer `:many` with
  `= ANY($1::bigint[])`.

```sql
-- name: CreateAuthors :batchexec
INSERT INTO authors (name, bio) VALUES ($1, $2);
```

```py
@dataclasses.dataclass()
class CreateAuthorsParams:
    name: str
    bio: Optional[str]


class Querier:
    # ...

    def create_authors(self, arg: Sequence[CreateAuthorsParams]) -> None:
        if not arg:
            return None
        self._conn.execute(
            sqlalchemy.text(CREATE_AUTHORS),
            [{"p1": a.name, "p2": a.bio} for a in arg],
        )
```

### Querier Protocols for Testability

Option: `emit_querier_protocol`

Generates `QuerierProtocol` and `AsyncQuerierProtocol` (`typing.Protocol`)
classes that declare every querier method. The queriers satisfy them
structurally, so application code can depend on the protocol and tests can
pass a simple fake. A protocol is emitted only for queriers that are enabled.

```py
class QuerierProtocol(Protocol):
    def get_author(self, *, id: int) -> Optional[models.Author]: ...

    def list_authors(self) -> Iterator[models.Author]: ...
```

```py
def get_author_bio(querier: QuerierProtocol, author_id: int) -> str:
    author = querier.get_author(id=author_id)
    return author.bio if author and author.bio else "Unknown"


class FakeQuerier:
    def get_author(self, *, id: int) -> Optional[models.Author]:
        return models.Author(id=id, name="Test", bio="A bio")

    def list_authors(self) -> Iterator[models.Author]:
        yield from ()
```

### Typed Error Wrapping

Option: `emit_query_errors`

Generates an `errors.py` module next to `models.py`. Every querier method body
is wrapped in `errors._wrap_errors(...)`, which re-raises
`sqlalchemy.exc.IntegrityError` and `sqlalchemy.exc.OperationalError` as a
subclass of `errors.QueryError`, chosen by SQLSTATE:

| Exception | SQLSTATE | From |
|---|---|---|
| `UniqueViolationError` | `23505` | `IntegrityError` |
| `ForeignKeyViolationError` | `23503` | `IntegrityError` |
| `CheckViolationError` | `23514` | `IntegrityError` |
| `NotNullViolationError` | `23502` | `IntegrityError` |
| `ExclusionViolationError` | `23P01` | `IntegrityError` |
| `StatementTimeoutError` | `57014` | `OperationalError` |
| `DeadlockError` | `40P01` | `OperationalError` |
| `SerializationError` | `40001` | `OperationalError` |

Any other SQLSTATE raises `QueryError` itself. Each error has `query_name`,
`cause` (the original SQLAlchemy exception, also set as `__cause__`) and
`constraint_name` (when the driver reports it). This works with psycopg 2,
psycopg 3 and asyncpg. Errors raised while you iterate a `:many`, `:batchone`
or `:batchmany` result are wrapped too; all other exceptions pass through
unchanged.

```py
def create_author(self, *, name: str, bio: Optional[str]) -> Optional[models.Author]:
    with errors._wrap_errors("create_author"):
        row = self._conn.execute(
            sqlalchemy.text(CREATE_AUTHOR), {"p1": name, "p2": bio}
        ).first()
        ...
```

```py
try:
    querier.create_author(name="Ursula", bio=None)
except errors.UniqueViolationError as e:
    print(e.constraint_name)  # "authors_name_key"
```

A query file named `errors.sql` would overwrite the generated module, so it
fails generation.

### Embedded Structs with `sqlc.embed()`

When a query joins tables, `sqlc.embed()` nests whole models in the row
instead of flattening their columns. With a `LEFT JOIN` that finds no match,
the embedded model is built from `None` values even though its fields are
typed as non-null, as in sqlc's Go codegen.

```sql
-- name: GetBookWithAuthor :one
SELECT sqlc.embed(books), sqlc.embed(authors)
FROM books
JOIN authors ON books.author_id = authors.id
WHERE books.id = $1;
```

```py
@dataclasses.dataclass()
class GetBookWithAuthorRow:
    books: models.Book
    authors: models.Author


def get_book_with_author(self, *, id: int) -> Optional[GetBookWithAuthorRow]:
    row = self._conn.execute(sqlalchemy.text(GET_BOOK_WITH_AUTHOR), {"p1": id}).first()
    if row is None:
        return None
    return GetBookWithAuthorRow(
        books=models.Book(
            id=cast(int, row[0]),
            author_id=cast(int, row[1]),
            title=cast(str, row[2]),
        ),
        authors=models.Author(
            id=cast(int, row[3]),
            name=cast(str, row[4]),
        ),
    )
```

### Modern Type Syntax

Option: `emit_modern_types` (Python 3.10+)

Emits `X | None` and `list[X]` instead of `Optional[X]` and `List[X]`, and
imports `Iterator`, `AsyncIterator` and `Sequence` from `collections.abc`
instead of `typing`.

### Aware Datetimes

Option: `emit_aware_datetime` (requires `emit_pydantic_models` and pydantic 2)

Annotates `timestamptz` columns and parameters as `pydantic.AwareDatetime`, so
naive datetimes fail validation. `timestamp` stays `datetime.datetime`.

### Emit Pydantic Models instead of `dataclasses`

Option: `emit_pydantic_models`

```py
class Author(pydantic.BaseModel):
    id: int
    name: str
```

Without the option:

```py
@dataclasses.dataclass()
class Author:
    id: int
    name: str
```

### Use `enum.StrEnum` for Enums

Option: `emit_str_enum` (Python 3.11+)

`enum.StrEnum` members are `str` instances, so they compare equal to strings
and serialize as strings.

```py
class Status(enum.StrEnum):
    """Venues can be either open or closed"""
    OPEN = "op!en"
    CLOSED = "clo@sed"
```

Without the option, enums subclass `(str, enum.Enum)`. Enum values that don't
make valid names become `VALUE_<n>`, get a `VALUE_` prefix, or a `_2` suffix.

### Type Overrides

Option: `overrides`

Maps a database type or a column to a Python type. `py_type` is either a
dotted path, imported as its module, or a bare name plus `py_import`, imported
with `from`:

```yaml
options:
  package: authors
  overrides:
    - db_type: jsonb
      py_type: my_lib.types.Payload     # import my_lib.types
    - db_type: jsonb
      nullable: true
      py_type: my_lib.types.Payload
    - column: "authors.id"              # or schema.table.column; * globs
      py_type: UUID
      py_import: uuid                   # from uuid import UUID
    - db_type: bytea
      py_type: bytes                    # builtin, no import
```

- A `db_type` entry applies to non-null columns, or only to nullable ones with
  `nullable: true`.
- `column` entries win over `db_type` entries, and also apply to parameters
  compared against that column.
- Nullable and array columns keep their `Optional[...]`/`List[...]` wrappers.
- `db_type` accepts SQL spellings such as `bigint` or
  `timestamp with time zone` as well as `int8` or `timestamptz`.
- A generic `py_type` such as `dict[str, decimal.Decimal]` imports the module
  of every dotted name in it. Bare names other than builtins are not imported,
  so write `typing.Any`, not `Any`.

Overrides and `rename` can also be set once for every codegen block in sqlc's
top-level `options`. Codegen `overrides` are checked before global ones, but a
global `rename` entry replaces a codegen entry for the same key, as in sqlc's
Go codegen:

```yaml
options:
  py:
    overrides:
      - db_type: citext
        py_type: my_lib.CIText
```

### Renames

Option: `rename`

Maps a database identifier to the Python name used for it: a singularized
table name (model class), an enum type name (enum class), a column name (field
and parameter) or an enum value (member). As in sqlc's Go codegen, it is one
flat map.

```yaml
rename:
  spotify_url: spotify_link
  person: Human
```

Names that are Python keywords get a trailing underscore (`from` becomes
`from_`), whether they come from the schema or from `rename`. So do keyword
parameters that would shadow a name the method body uses, such as `errors`,
`models`, `cast` or an imported module, and a repeated parameter name gets a
numeric suffix (`id`, `id_2`).

### Omit Unused Structs

Option: `omit_unused_structs`

Leaves out of `models.py` the enums and models that no query's parameters,
result or embedded tables refer to.

### Omit sqlc Version

Option: `omit_sqlc_version`

Leaves the `# versions:` / `#   sqlc vX.Y.Z` lines out of file headers.

## Migrating from upstream or the original alt-sqlc-gen-python

- From upstream sqlc-gen-python: swap the plugin URL and sha256. Options are
  unchanged and new behaviour is opt-in. Expect diffs where upstream produced
  invalid code (keyword names, empty classes, enum names) or `Any`
  (`varchar`, `timestamp`, …), plus the annotation-only `cast()` and
  `_conn: Union[...]` changes.
- From [asavoy/alt-sqlc-gen-python][asavoy]:
  - Set `emit_modern_types`, `emit_aware_datetime` and `emit_generic_querier`
    to get its output style; here they are opt-in.
  - `emit_query_errors` keeps the same class names, constructor, `query_name`
    and `cause`; `constraint_name` is new.
  - `overrides` accepts its `py_type` + `py_import` form unchanged.
  - `:batchexec` takes one positional `arg: Sequence[Params]` instead of a
    keyword-only `args: list[Params]`.

## Development

```sh
mise install              # Go, buf, Python
mise exec -- make all     # bin/sqlc-gen-python.wasm
mise exec -- make test    # unit tests + sqlc diff over internal/endtoend/testdata
```

`make test` needs sqlc on `PATH`. The examples in `examples/` are generated
from the local build; run `sqlc diff` there, and `pytest src/tests` against a
Postgres (see `examples/src/tests/conftest.py` for the `PG_*` variables).

To try a local build in another project, point `sqlc.yaml` at it
(`url: file:///path/to/bin/sqlc-gen-python.wasm`, no `sha256` needed).

## Release process

Releases are automated by `.github/workflows/release.yml`. On every push to
`main`, the workflow reads the merged PR's title:

| PR title | Bump |
|---|---|
| `feat: …` | minor |
| `fix: …` / `hotfix: …` | patch |
| `type!: …`, or `BREAKING CHANGE:` in the body | major |
| anything else | no release |

It then builds `alt-sqlc-gen-python.wasm`, writes its sha256 to
`alt-sqlc-gen-python.wasm.sha256`, and creates a GitHub release tagged
`vX.Y.Z` with both files and generated notes. To force a bump, run the
workflow manually with `bump` set to `patch`, `minor` or `major`.

[upstream]: https://github.com/sqlc-dev/sqlc-gen-python
[asavoy]: https://github.com/asavoy/alt-sqlc-gen-python
