# Spec Delta

## ADDED Requirements

### Requirement: Params-class argument

When a non-sequence query's parameters are packed into a params class (more
parameters than `query_parameter_limit`, or `query_parameter_limit: 0`), the
sync and async querier methods and protocol methods SHALL take that params
class as one keyword-only argument named `arg`.

#### Scenario: Over the limit

- **WHEN** `query_parameter_limit: 2` and `UpdateAuthor :exec` has three
  parameters
- **THEN** the querier has
  `def update_author(self, *, arg: UpdateAuthorParams) -> None`, and calling
  `q.update_author(UpdateAuthorParams(...))` positionally is a `TypeError`

#### Scenario: Limit of zero

- **WHEN** `query_parameter_limit: 0` and `GetAuthor :one` has one parameter
- **THEN** the querier has
  `def get_author(self, *, arg: GetAuthorParams) -> Optional[models.Author]`

## MODIFIED Requirements

### Requirement: Copyfrom

A `:copyfrom` query SHALL generate a params class holding every query parameter,
even when there are fewer parameters than `query_parameter_limit`. The sync and
async querier methods SHALL take one keyword-only argument named `args`, a
`Sequence` of that params class. Each method SHALL run the statement once over
all the items as a single `executemany` call and return the driver-reported row
count as `int`. Generation SHALL no longer fail for `:copyfrom`.

#### Scenario: Bulk insert

- **WHEN** a query is `-- name: CreateAuthors :copyfrom` with
  `INSERT INTO authors (name, bio) VALUES ($1, $2)`
- **THEN** the querier has
  `def create_authors(self, *, args: Sequence[CreateAuthorsParams]) -> int`
  (and an `async def` form), which passes a list of parameter dicts, one per
  item, to a single `execute` call

#### Scenario: Empty input

- **WHEN** `create_authors` is called with `args=[]`
- **THEN** it returns `0` without running the statement

### Requirement: Batch commands

A `:batchone`, `:batchmany`, or `:batchexec` query SHALL generate a params class
holding every query parameter, as `:copyfrom` does, whatever
`query_parameter_limit` is. Each method SHALL take one keyword-only argument
named `args`, a `Sequence` of that params class. An empty sequence SHALL run no
statements.

`:batchexec` SHALL run the statement once as a single `executemany` over all
items, and SHALL return `None` (awaitable on `AsyncQuerier`).

`:batchone` and `:batchmany` SHALL run the statement once per item, in order, on
the querier's connection. They SHALL produce results lazily, as the caller
consumes them:

| Command | Sync return | Async return | Value per input item |
|---|---|---|---|
| `:batchone` | `Iterator[Row \| None]` | `AsyncIterator[Row \| None]` | the first row, or `None` when there is no row |
| `:batchmany` | `Iterator[list[Row]]` | `AsyncIterator[list[Row]]` | all rows, possibly empty |

`Row` here means the query's row class, the reused model, or the scalar type,
chosen by the same rules as `:one` and `:many`. The annotations above use modern
spelling. Generated code SHALL spell them in the active type syntax
(`Optional[Row]` and `List[Row]` when `emit_modern_types` is off).

#### Scenario: Batchone yields per input

- **WHEN** `GetAuthorsByID :batchone` is called with three params objects and
  the second id does not exist
- **THEN** iterating the result yields a row, then `None`, then a row

#### Scenario: Batchmany yields a list per input

- **WHEN** `ListBooksByAuthor :batchmany` is called with two params objects
- **THEN** iterating the result yields exactly two lists, one per params object,
  in input order

#### Scenario: Lazy execution

- **WHEN** a `:batchone` method is called and the caller stops after the first
  yielded result
- **THEN** only the first item's statement has run

#### Scenario: Batchexec uses one executemany

- **WHEN** `CreateAuthors :batchexec`
  (`INSERT INTO authors (name, bio) VALUES ($1, $2)`) is called with two params
  objects
- **THEN** the method makes one `execute` call with a list of two parameter
  dicts, and returns `None` once both rows are written

#### Scenario: Keyword-only args

- **WHEN** a `:batchexec` query `CreateAuthors` is generated
- **THEN** the method signature is
  `def create_authors(self, *, args: Sequence[CreateAuthorsParams]) -> None`,
  and passing the sequence positionally is a `TypeError`
