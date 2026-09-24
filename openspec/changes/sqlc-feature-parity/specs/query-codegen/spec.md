# Spec Delta

## Purpose

Defines how sqlc queries become Python querier code: sync and async queriers and their protocols, typed error wrapping, typed row values, embedded tables, query comments, bulk inserts with `:copyfrom`, batch commands, and multi-dimensional array types.

## ADDED Requirements

### Requirement: Embedded tables
When a query's result uses `sqlc.embed(<table>)`, the generated row class SHALL have one field for the embed. The field SHALL be named after the embedded table (or its alias) and typed as that table's model (`models.<Model>`). The querier SHALL build the embedded model from that table's columns, in order, at their positions in the result row. The row index of every field after the embed SHALL be shifted to match. A query whose only result column is an embed SHALL still return a row class, not a bare scalar.

#### Scenario: Embed alongside a plain column
- **WHEN** a `:one` query is `SELECT sqlc.embed(authors), books.title FROM authors JOIN books ...` and `authors` has columns `id` and `name`
- **THEN** the row class has fields `authors: models.Author` and `title: str`, and it is built as `Row(authors=models.Author(id=cast(int, row[0]), name=cast(str, row[1])), title=cast(str, row[2]))`

#### Scenario: Two embeds
- **WHEN** a query embeds both `authors` and `books`
- **THEN** the row class has a field for each, each built from its own consecutive column range

### Requirement: Query comments as docstrings
Comment lines written above a query's `-- name:` line SHALL become the docstring of the generated sync and async querier methods, one comment line per docstring line, in order. Queries without comments SHALL produce no docstring.

#### Scenario: Commented query
- **WHEN** a query is preceded by `-- Fetch an author by id.`
- **THEN** the `get_author` method body starts with the docstring `"""Fetch an author by id."""`

### Requirement: Copyfrom
A `:copyfrom` query SHALL generate a params class holding every query parameter, even when there are fewer parameters than `query_parameter_limit`. The sync and async querier methods SHALL take one positional argument, a `Sequence` of that params class. Each method SHALL run the statement once over all the items as a single `executemany` call and return the driver-reported row count as `int`. Generation SHALL no longer fail for `:copyfrom`.

#### Scenario: Bulk insert
- **WHEN** a query is `-- name: CreateAuthors :copyfrom` with `INSERT INTO authors (name, bio) VALUES ($1, $2)`
- **THEN** the querier has `def create_authors(self, arg: Sequence[CreateAuthorsParams]) -> int` (and an `async def` form), which passes a list of parameter dicts, one per item, to a single `execute` call

#### Scenario: Empty input
- **WHEN** `create_authors` is called with an empty sequence
- **THEN** it returns `0` without running the statement

### Requirement: Multi-dimensional arrays
A column or parameter with N array dimensions SHALL be annotated with N nested `List[...]` wrappers around its element type. `Optional[...]`, when the column is nullable, SHALL wrap the outermost list.

#### Scenario: Two-dimensional array
- **WHEN** a column is `matrix integer[][] NOT NULL`
- **THEN** it is annotated `List[List[int]]`

#### Scenario: One-dimensional array unchanged
- **WHEN** a column is `tags text[]`
- **THEN** it is annotated `Optional[List[str]]`, as before

### Requirement: Sync and async queriers
With `emit_sync_querier: true`, each query file SHALL define a `Querier` class whose constructor accepts a `sqlalchemy.engine.Connection` or a `sqlalchemy.orm.Session`. With `emit_async_querier: true`, each query file SHALL define an `AsyncQuerier` class whose constructor accepts a `sqlalchemy.ext.asyncio.AsyncConnection` or a `sqlalchemy.ext.asyncio.AsyncSession`. Each querier SHALL declare its `_conn` attribute with that union type. Both flags MAY be set together. Every query command the plugin supports (`:one`, `:many`, `:exec`, `:execrows`, `:execresult`, `:copyfrom`, `:batchone`, `:batchmany`, `:batchexec`) SHALL generate a method on each enabled querier. The method SHALL have the same name, parameters, and result type on both, except that async methods SHALL be awaitable or async-iterable. With neither flag set, query files SHALL contain only the SQL constants and the params and row classes.

#### Scenario: Session accepted
- **WHEN** a `Querier` is constructed with a `sqlalchemy.orm.Session`, or an `AsyncQuerier` with an `AsyncSession`
- **THEN** every generated method works and type-checks exactly as it does with a connection

#### Scenario: Non-generic by default
- **WHEN** `emit_generic_querier` is unset
- **THEN** `Querier` has no type parameters, and `_conn` is annotated with the connection-or-session union

#### Scenario: Both queriers
- **WHEN** both flags are true and the file has a `:many` query `ListAuthors`
- **THEN** `Querier.list_authors` returns an `Iterator` of rows, and `AsyncQuerier.list_authors` is an async generator returning an `AsyncIterator` of the same row type

### Requirement: Generic queriers
With `emit_generic_querier: true`, each emitted querier SHALL be a PEP 695 generic class whose single type parameter `T` is bounded by that querier's connection-or-session union:
- `class Querier[T: <sync union>]`
- `class AsyncQuerier[T: <async union>]`

`_conn` SHALL be annotated `T`, and `__init__` SHALL take `conn: T`. The bound SHALL be spelled in the active type syntax (`Union[...]` or `|`). Method signatures and bodies SHALL be the same as for non-generic queriers. Querier protocols SHALL stay non-generic, and a generic querier of any valid `T` SHALL still satisfy its protocol.

#### Scenario: Session type is preserved
- **WHEN** `emit_generic_querier` and `emit_sync_querier` are true and code runs `q = Querier(session)` with a `sqlalchemy.orm.Session`
- **THEN** a type checker infers `q` as `Querier[Session]` and `q._conn` as `Session`

#### Scenario: Bound is enforced
- **WHEN** code calls `Querier(object())` with the option on
- **THEN** a type checker reports that the argument violates the bound of `T`

#### Scenario: Works with both syntax styles
- **WHEN** `emit_generic_querier` is true, once with `emit_modern_types` on and once with it off
- **THEN** the bounds read `sqlalchemy.engine.Connection | sqlalchemy.orm.Session` and `Union[sqlalchemy.engine.Connection, sqlalchemy.orm.Session]` respectively, and both files compile on Python 3.12

### Requirement: Typed row values
Every value that a querier method reads from a result row SHALL be wrapped in `typing.cast(<annotation>, row[i])` (`cast` imported from `typing`). `<annotation>` SHALL be the field's or scalar's annotation, spelled in the active type syntax. This applies to scalar returns, row-class fields, reused models, and embedded models. It SHALL have no runtime effect.

#### Scenario: Row field cast
- **WHEN** a `:one` query returns `models.Author` with fields `id: int` and `bio: Optional[str]`
- **THEN** the method builds `models.Author(id=cast(int, row[0]), bio=cast(Optional[str], row[1]))`

#### Scenario: New commands on both queriers
- **WHEN** both flags are true and the file has `:copyfrom` and `:batchone` queries
- **THEN** both commands appear as methods on `Querier` and `AsyncQuerier`

### Requirement: Batch commands
A `:batchone`, `:batchmany`, or `:batchexec` query SHALL generate a params class holding every query parameter, as `:copyfrom` does, whatever `query_parameter_limit` is. Each method SHALL take one positional argument, a `Sequence` of that params class. An empty sequence SHALL run no statements.

`:batchexec` SHALL run the statement once as a single `executemany` over all items, and SHALL return `None` (awaitable on `AsyncQuerier`).

`:batchone` and `:batchmany` SHALL run the statement once per item, in order, on the querier's connection. They SHALL produce results lazily, as the caller consumes them:

| Command | Sync return | Async return | Value per input item |
|---|---|---|---|
| `:batchone` | `Iterator[Row \| None]` | `AsyncIterator[Row \| None]` | the first row, or `None` when there is no row |
| `:batchmany` | `Iterator[list[Row]]` | `AsyncIterator[list[Row]]` | all rows, possibly empty |

`Row` here means the query's row class, the reused model, or the scalar type, chosen by the same rules as `:one` and `:many`. The annotations above use modern spelling. Generated code SHALL spell them in the active type syntax (`Optional[Row]` and `List[Row]` when `emit_modern_types` is off).

#### Scenario: Batchone yields per input
- **WHEN** `GetAuthorsByID :batchone` is called with three params objects and the second id does not exist
- **THEN** iterating the result yields a row, then `None`, then a row

#### Scenario: Batchmany yields a list per input
- **WHEN** `ListBooksByAuthor :batchmany` is called with two params objects
- **THEN** iterating the result yields exactly two lists, one per params object, in input order

#### Scenario: Lazy execution
- **WHEN** a `:batchone` method is called and the caller stops after the first yielded result
- **THEN** only the first item's statement has run

#### Scenario: Batchexec uses one executemany
- **WHEN** `CreateAuthors :batchexec` (`INSERT INTO authors (name, bio) VALUES ($1, $2)`) is called with two params objects
- **THEN** the method makes one `execute` call with a list of two parameter dicts, and returns `None` once both rows are written

### Requirement: Querier protocols
With `emit_querier_protocol: true`, each query file SHALL also define `QuerierProtocol` (when the sync querier is emitted) and `AsyncQuerierProtocol` (when the async querier is emitted). Both SHALL subclass `typing.Protocol`. Each protocol SHALL declare every method of the matching querier, with the same parameters and return annotation and a body of `...`. The generated `Querier` and `AsyncQuerier` SHALL satisfy their protocols structurally under mypy and pyright. For async methods that are async generators (`:many`, `:batchone`, `:batchmany`), the protocol SHALL declare a plain `def` that returns `AsyncIterator[...]`, so that the async-generator implementation matches it. With the option off, no protocol classes SHALL be emitted.

#### Scenario: Protocol mirrors querier
- **WHEN** `emit_querier_protocol` and `emit_sync_querier` are true
- **THEN** `QuerierProtocol` declares `get_author(self, *, id: int) -> Optional[models.Author]: ...`, and `q: QuerierProtocol = Querier(conn)` type-checks

#### Scenario: Async generator method in protocol
- **WHEN** `emit_querier_protocol` and `emit_async_querier` are true and the file has a `:many` query
- **THEN** `AsyncQuerierProtocol` declares it as `def list_authors(self) -> AsyncIterator[models.Author]: ...`, and `AsyncQuerier` type-checks against the protocol

#### Scenario: Hand-written fake
- **WHEN** a test defines a class that implements the `QuerierProtocol` methods with in-memory data
- **THEN** it can be passed where a `QuerierProtocol` is expected and type-checks

### Requirement: Typed query errors
With `emit_query_errors: true`, the plugin SHALL generate `errors.py` in the output package. It SHALL define `QueryError(Exception)`, with a constructor `(message, query_name, cause=None)` and attributes `query_name`, `cause` (the original SQLAlchemy exception), and `constraint_name` (a string when the driver exposes it, otherwise `None`). It SHALL also define these direct subclasses of `QueryError`:

| Class | Raised for | SQLSTATE |
|---|---|---|
| `UniqueViolationError` | `sqlalchemy.exc.IntegrityError` | `23505` |
| `ForeignKeyViolationError` | `sqlalchemy.exc.IntegrityError` | `23503` |
| `CheckViolationError` | `sqlalchemy.exc.IntegrityError` | `23514` |
| `NotNullViolationError` | `sqlalchemy.exc.IntegrityError` | `23502` |
| `ExclusionViolationError` | `sqlalchemy.exc.IntegrityError` | `23P01` |
| `StatementTimeoutError` | `sqlalchemy.exc.OperationalError` | `57014` |
| `DeadlockError` | `sqlalchemy.exc.OperationalError` | `40P01` |
| `SerializationError` | `sqlalchemy.exc.OperationalError` | `40001` |

When a querier method raises `sqlalchemy.exc.IntegrityError` or `sqlalchemy.exc.OperationalError`, it SHALL raise the class matching the SQLSTATE, `from` the original. A SQLSTATE that is not in the table, or missing, SHALL raise `QueryError` itself. This SHALL hold for sync and async methods, for every command, and while a `:many`, `:batchone`, or `:batchmany` result is being iterated. The SQLSTATE SHALL be read in a way that works with psycopg2, psycopg 3, and asyncpg. Every other exception SHALL propagate unchanged.

These class names, the constructor, and `query_name` and `cause` SHALL match alt-sqlc-gen-python's `emit_query_errors`, so code written against that fork keeps working. With the option off, no `errors.py` SHALL be generated and methods SHALL not catch exceptions. When a query file would also produce `errors.py`, generation SHALL fail with an error that names the conflict.

#### Scenario: Unique violation
- **WHEN** `create_author` inserts a duplicate value into a `UNIQUE` column named by constraint `authors_name_key`
- **THEN** it raises `errors.UniqueViolationError`, with `query_name == "create_author"`, `constraint_name == "authors_name_key"`, and `cause` and `__cause__` both set to the original `sqlalchemy.exc.IntegrityError`

#### Scenario: Serialization failure
- **WHEN** a query in a `SERIALIZABLE` transaction fails with SQLSTATE `40001`
- **THEN** it raises `errors.SerializationError`

#### Scenario: Catch by base class
- **WHEN** any wrapped error occurs inside a querier method
- **THEN** `except errors.QueryError` catches it

#### Scenario: Other errors pass through
- **WHEN** a querier method fails with a `sqlalchemy.exc.ProgrammingError`
- **THEN** that same exception propagates, unwrapped

#### Scenario: Error while iterating
- **WHEN** a `:many` method's result stream fails with a deadlock partway through iteration
- **THEN** the consumer's loop raises `errors.DeadlockError`

#### Scenario: Works with psycopg and asyncpg
- **WHEN** the same unique violation happens through a sync psycopg connection and through an async asyncpg connection
- **THEN** both raise `errors.UniqueViolationError`
