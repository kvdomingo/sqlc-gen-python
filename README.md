## Usage

```yaml
version: "2"
plugins:
  - name: py
    wasm:
      url: https://downloads.sqlc.dev/plugin/sqlc-gen-python_1.3.0.wasm
      sha256: fbedae96b5ecae2380a70fb5b925fd4bff58a6cfb1f3140375d098fbab7b3a3c
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

### Emit Pydantic Models instead of `dataclasses`

Option: `emit_pydantic_models`

By default, `sqlc-gen-python` will emit `dataclasses` for the models. If you
prefer to use [`pydantic`](https://docs.pydantic.dev/latest/) models, you can
enable this option.

with `emit_pydantic_models`

```py
from pydantic import BaseModel

class Author(pydantic.BaseModel):
    id: int
    name: str
```

without `emit_pydantic_models`

```py
import dataclasses

@dataclasses.dataclass()
class Author:
    id: int
    name: str
```

### Use `enum.StrEnum` for Enums

Option: `emit_str_enum`

`enum.StrEnum` was introduce in Python 3.11.

`enum.StrEnum` is a subclass of `str` that is also a subclass of `Enum`. This
allows for the use of `Enum` values as strings, compared to strings, or compared
to other `enum.StrEnum` types.

This is convenient for type checking and validation, as well as for
serialization and deserialization.

By default, `sqlc-gen-python` will emit `(str, enum.Enum)` for the enum classes.
If you prefer to use `enum.StrEnum`, you can enable this option.

with `emit_str_enum`

```py
class Status(enum.StrEnum):
    """Venues can be either open or closed"""
    OPEN = "op!en"
    CLOSED = "clo@sed"
```

without `emit_str_enum` (current behavior)

```py
class Status(str, enum.Enum):
    """Venues can be either open or closed"""
    OPEN = "op!en"
    CLOSED = "clo@sed"
```

## Queriers

With `emit_sync_querier` and/or `emit_async_querier`, each query file gets a
`Querier` and/or `AsyncQuerier` class. `Querier` accepts a
`sqlalchemy.engine.Connection` or a `sqlalchemy.orm.Session`; `AsyncQuerier`
accepts a `sqlalchemy.ext.asyncio.AsyncConnection` or an `AsyncSession`.

```py
with Session(engine) as session:
    querier = query.Querier(session)
    author = querier.get_author(id=1)
```

Values read from result rows are wrapped in `typing.cast(...)`, so the
generated code passes `mypy --strict`.

Comments above a query's `-- name:` line become the method's docstring.

### Generic queriers

Option: `emit_generic_querier` (Python 3.12+)

Emits the queriers as [PEP 695](https://peps.python.org/pep-0695/) generics,
so the connection type you pass in is preserved:

```py
class Querier[T: Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]]:
    _conn: T

    def __init__(self, conn: T):
        self._conn = conn
```

`Querier(session)._conn` is then typed as `Session`. Type-checking the output
needs mypy 1.12+ or a recent pyright.

### Querier protocols

Option: `emit_querier_protocol`

Emits `QuerierProtocol` and `AsyncQuerierProtocol` (`typing.Protocol`)
classes that declare every querier method. The generated queriers satisfy
them structurally, so a hand-written fake can stand in for a querier in tests:

```py
def list_names(q: query.QuerierProtocol) -> list[str]:
    return [a.name for a in q.list_authors()]
```

### Typed query errors

Option: `emit_query_errors`

Generates an `errors.py` module next to `models.py`. Querier methods re-raise
`sqlalchemy.exc.IntegrityError` and `sqlalchemy.exc.OperationalError` as a
subclass of `errors.QueryError`, chosen by SQLSTATE:

| Class | SQLSTATE |
|---|---|
| `UniqueViolationError` | `23505` |
| `ForeignKeyViolationError` | `23503` |
| `CheckViolationError` | `23514` |
| `NotNullViolationError` | `23502` |
| `ExclusionViolationError` | `23P01` |
| `StatementTimeoutError` | `57014` |
| `DeadlockError` | `40P01` |
| `SerializationError` | `40001` |

Any other SQLSTATE raises `QueryError` itself. Each error carries
`query_name`, `cause` (the original SQLAlchemy exception, also set as
`__cause__`) and `constraint_name` (when the driver reports it: psycopg 2/3
and asyncpg do). Errors raised while iterating a `:many`, `:batchone` or
`:batchmany` result are wrapped too; all other exceptions pass through.

```py
try:
    querier.create_author(name="Ursula", bio=None)
except errors.UniqueViolationError as e:
    print(e.constraint_name)  # "authors_name_key"
```

A query file named `errors.sql` conflicts with the generated module and fails
generation.

## Query commands

Besides `:one`, `:many`, `:exec`, `:execrows` and `:execresult`:

- `:copyfrom` takes one positional `Sequence` of a generated params class,
  runs a single `executemany`, and returns the row count. An empty sequence
  returns `0` without a round-trip. The count is whatever the DBAPI driver
  reports; some report `-1` for `executemany`.
- `:batchexec` takes the same kind of sequence, runs a single `executemany`,
  and returns `None`.
- `:batchone` and `:batchmany` take the same kind of sequence and return a
  (async) generator that runs the statement once per item, in order, as you
  iterate: `:batchone` yields the first row or `None` per item, `:batchmany`
  yields a list of rows per item. That is one round-trip per item; for bulk
  reads prefer `:many` with `= ANY($1::bigint[])`.

`sqlc.embed(table)` gives the row class one field typed as the table's model:

```py
@dataclasses.dataclass()
class GetBookWithAuthorRow:
    books: models.Book
    authors: models.Author
```

## Types

### Type overrides

Option: `overrides`

Maps a database type or a column to a Python type, the way sqlc's Go
`overrides` do. `py_type` is either a dotted path, which is imported as its
module, or a bare name with `py_import`, which is imported with `from`:

```yaml
options:
  package: db
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

A `db_type` entry applies to non-null columns unless `nullable: true` is set,
in which case it applies only to nullable ones. `column` entries win over
`db_type` entries and also apply to parameters compared against that column.
Nullable and array columns keep their `Optional[...]`/`List[...]` wrappers.

Overrides (and `rename`) can also be set once for every codegen block in
sqlc's top-level `options`; entries in the codegen block are checked first:

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
table name (model class), an enum type name (enum class), a column name
(field and parameter) or an enum value (member). As in sqlc's Go codegen, it
is one flat map.

```yaml
rename:
  spotify_url: spotify_link
  person: Human
```

Names that are Python keywords get a trailing underscore (`from` becomes
`from_`), whether they come from the schema or from `rename`.

### Modern type syntax

Option: `emit_modern_types` (Python 3.10+)

Emits `X | None`, `list[X]`, and `Iterator`/`AsyncIterator`/`Sequence` from
`collections.abc` instead of `Optional`, `List` and the `typing` aliases.

### Aware datetimes

Option: `emit_aware_datetime` (requires `emit_pydantic_models` and pydantic 2)

Annotates `timestamptz` columns and parameters as `pydantic.AwareDatetime`,
so naive datetimes fail validation. `timestamp` stays `datetime.datetime`.

## Output

### Omit unused structs

Option: `omit_unused_structs`

Leaves out of `models.py` the enums and models that no query's parameters,
result or embedded tables refer to.

### Omit sqlc version

Option: `omit_sqlc_version`

Leaves the `# versions:` / `#   sqlc vX.Y.Z` lines out of the file header.

## Migrating from alt-sqlc-gen-python

- `emit_query_errors` uses the same class names, constructor, `query_name` and
  `cause`, so existing `except` clauses keep working; `constraint_name` is new.
- `overrides` accepts the fork's `py_type` + `py_import` form unchanged.
- `:batchexec` takes one positional `arg: Sequence[Params]` (like `:copyfrom`)
  rather than a keyword-only `args: list[Params]`.
- Modern syntax, aware datetimes and generic queriers are opt-in here: set
  `emit_modern_types`, `emit_aware_datetime` and `emit_generic_querier` to get
  the fork's output style.
