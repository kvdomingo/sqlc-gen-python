import datetime
import os
from typing import Iterator, List, Optional, Sequence

import pydantic
import pytest
import sqlalchemy
import sqlalchemy.event
import sqlalchemy.ext.asyncio
import sqlalchemy.orm

from dbtest.migrations import apply_migrations, apply_migrations_async
from library import errors, models, query
from library_modern import errors as modern_errors
from library_modern import models as modern_models
from library_modern import query as modern_query

SCHEMA = os.path.dirname(__file__) + "/../library/schema.sql"


class StatementLog:
    def __init__(self, conn: sqlalchemy.engine.Connection):
        self.statements: List[bool] = []
        sqlalchemy.event.listen(conn, "before_cursor_execute", self._record)

    def _record(self, conn, cursor, statement, parameters, context, executemany):
        self.statements.append(executemany)


@pytest.fixture
def q(db: sqlalchemy.engine.Connection) -> query.Querier:
    apply_migrations(db, [SCHEMA])
    return query.Querier(db)


@pytest.fixture
async def aq(async_db: sqlalchemy.ext.asyncio.AsyncConnection) -> query.AsyncQuerier:
    await apply_migrations_async(async_db, [SCHEMA])
    return query.AsyncQuerier(async_db)


def test_copyfrom_and_batchexec_use_one_executemany(db, q):
    log = StatementLog(db)
    assert q.create_authors([]) == 0
    q.create_books([])
    assert log.statements == []

    count = q.create_authors([
        query.CreateAuthorsParams(name="Ursula", bio=None),
        query.CreateAuthorsParams(name="Octavia", bio="Kindred"),
    ])
    assert count == 2
    authors = {a.name: a for a in q.list_authors()}
    log.statements.clear()
    q.create_books([
        query.CreateBooksParams(author_id=authors["Ursula"].id, title="The Dispossessed", status=models.BookStatus.IN_PRINT),
        query.CreateBooksParams(author_id=authors["Octavia"].id, title="Kindred", status=models.BookStatus.DRAFT),
    ])
    assert log.statements == [True]


def test_batchone_is_lazy_and_ordered(db, q):
    a = q.create_author(name="Ursula", bio=None)
    b = q.create_author(name="Octavia", bio=None)
    assert a is not None and b is not None
    log = StatementLog(db)

    results = q.get_authors_by_id([
        query.GetAuthorsByIDParams(id=a.id),
        query.GetAuthorsByIDParams(id=-1),
        query.GetAuthorsByIDParams(id=b.id),
    ])
    assert log.statements == []
    assert next(results) == a
    assert len(log.statements) == 1
    assert list(results) == [None, b]
    assert len(log.statements) == 3


def test_batchmany_yields_a_list_per_input(q):
    a = q.create_author(name="Ursula", bio=None)
    b = q.create_author(name="Octavia", bio=None)
    assert a is not None and b is not None
    first = q.create_book(author_id=a.id, title="The Dispossessed")
    second = q.create_book(author_id=a.id, title="The Lathe of Heaven")

    results = list(q.list_books_by_author([
        query.ListBooksByAuthorParams(author_id=a.id),
        query.ListBooksByAuthorParams(author_id=b.id),
    ]))
    assert results == [[first, second], []]


def test_embed(q):
    a = q.create_author(name="Ursula", bio="Earthsea")
    assert a is not None
    book = q.create_book(author_id=a.id, title="The Dispossessed")
    assert book is not None
    row = q.get_book_with_author(id=book.id)
    assert row == query.GetBookWithAuthorRow(books=book, authors=a)


def test_query_docstring():
    assert query.Querier.create_author.__doc__ == "Create one author."


def test_typed_errors(db, q):
    a = q.create_author(name="Ursula", bio=None)
    assert a is not None

    with pytest.raises(errors.UniqueViolationError) as unique:
        with db.begin_nested():
            q.create_author(name="Ursula", bio=None)
    assert unique.value.query_name == "create_author"
    assert unique.value.constraint_name == "authors_name_key"
    assert isinstance(unique.value.cause, sqlalchemy.exc.IntegrityError)
    assert unique.value.__cause__ is unique.value.cause

    with pytest.raises(errors.ForeignKeyViolationError) as fk:
        with db.begin_nested():
            q.create_book(author_id=-1, title="Orphan")
    assert fk.value.constraint_name == "books_author_id_fkey"

    with pytest.raises(errors.NotNullViolationError):
        with db.begin_nested():
            q.create_author(name=None, bio=None)  # type: ignore[arg-type]

    book = q.create_book(author_id=a.id, title="The Dispossessed")
    assert book is not None
    with pytest.raises(errors.CheckViolationError) as check:
        with db.begin_nested():
            q.set_book_title(id=book.id, title="")
    assert check.value.constraint_name == "books_title_check"

    with pytest.raises(errors.QueryError):
        with db.begin_nested():
            q.create_authors([query.CreateAuthorsParams(name="Ursula", bio=None)])


def test_error_while_iterating_is_wrapped(db, q):
    results = q.insert_authors_returning([
        query.InsertAuthorsReturningParams(name="Ursula"),
        query.InsertAuthorsReturningParams(name="Ursula"),
    ])
    with db.begin_nested():
        first = next(results)
    assert first is not None and first.name == "Ursula"
    with pytest.raises(errors.UniqueViolationError) as unique:
        with db.begin_nested():
            next(results)
    assert unique.value.query_name == "insert_authors_returning"


async def test_async_error_while_iterating_is_wrapped(async_db, aq):
    names = [query.InsertAuthorsReturningParams(name="Ursula")] * 2
    with pytest.raises(errors.UniqueViolationError):
        async with async_db.begin_nested():
            async for _ in aq.insert_authors_returning(names):
                pass


def test_other_errors_pass_through(db, q):
    with pytest.raises(sqlalchemy.exc.ProgrammingError):
        with db.begin_nested():
            db.execute(sqlalchemy.text("DROP TABLE books"))
            list(q.list_books_by_author([query.ListBooksByAuthorParams(author_id=1)]))


async def test_async_batches_and_errors(async_db, aq):
    a = await aq.create_author(name="Ursula", bio=None)
    assert a is not None
    assert await aq.create_authors([query.CreateAuthorsParams(name="Octavia", bio=None)]) == 1

    found = [x async for x in aq.get_authors_by_id([query.GetAuthorsByIDParams(id=a.id), query.GetAuthorsByIDParams(id=-1)])]
    assert found == [a, None]
    books = [x async for x in aq.list_books_by_author([query.ListBooksByAuthorParams(author_id=a.id)])]
    assert books == [[]]

    with pytest.raises(errors.UniqueViolationError) as unique:
        async with async_db.begin_nested():
            await aq.create_author(name="Ursula", bio=None)
    assert unique.value.constraint_name == "authors_name_key"

    with pytest.raises(errors.ForeignKeyViolationError) as fk:
        async with async_db.begin_nested():
            await aq.create_books([query.CreateBooksParams(author_id=-1, title="Orphan", status=models.BookStatus.DRAFT)])
    assert fk.value.constraint_name == "books_author_id_fkey"

    with pytest.raises(errors.NotNullViolationError):
        async with async_db.begin_nested():
            await aq.create_author(name=None, bio=None)  # type: ignore[arg-type]


def test_session_backed_querier(db):
    apply_migrations(db, [SCHEMA])
    with sqlalchemy.orm.Session(bind=db) as session:
        q = query.Querier(session)
        a = q.create_author(name="Ursula", bio=None)
        assert a is not None
        assert q.create_authors([query.CreateAuthorsParams(name="Octavia", bio=None)]) == 1
        assert [x.name for x in q.list_authors()] == ["Octavia", "Ursula"]
        assert list(q.get_authors_by_id([query.GetAuthorsByIDParams(id=a.id)])) == [a]


class FakeQuerier:
    def __init__(self) -> None:
        self.authors: dict = {}

    def create_author(self, *, name: str, bio: Optional[str]) -> Optional[models.Author]:
        author = models.Author(id=len(self.authors) + 1, name=name, bio=bio, created_at=datetime.datetime.now(datetime.timezone.utc))
        self.authors[author.id] = author
        return author

    def create_author_at(self, *, name: str, created_at: datetime.datetime) -> Optional[models.Author]:
        raise NotImplementedError

    def list_authors(self) -> Iterator[models.Author]:
        yield from sorted(self.authors.values(), key=lambda a: a.name)

    def create_authors(self, arg: Sequence[query.CreateAuthorsParams]) -> int:
        for a in arg:
            self.create_author(name=a.name, bio=a.bio)
        return len(arg)

    def create_books(self, arg: Sequence[query.CreateBooksParams]) -> None:
        raise NotImplementedError

    def create_book(self, *, author_id: int, title: str) -> Optional[models.Book]:
        raise NotImplementedError

    def set_book_title(self, *, id: int, title: str) -> None:
        raise NotImplementedError

    def get_authors_by_id(self, arg: Sequence[query.GetAuthorsByIDParams]) -> Iterator[Optional[models.Author]]:
        for a in arg:
            yield self.authors.get(a.id)

    def list_books_by_author(self, arg: Sequence[query.ListBooksByAuthorParams]) -> Iterator[List[models.Book]]:
        for _ in arg:
            yield []

    def get_book_with_author(self, *, id: int) -> Optional[query.GetBookWithAuthorRow]:
        return None

    def insert_authors_returning(self, arg: Sequence[query.InsertAuthorsReturningParams]) -> Iterator[Optional[models.Author]]:
        raise NotImplementedError


def author_names(q: query.QuerierProtocol) -> List[str]:
    return [a.name for a in q.list_authors()]


def test_fake_satisfies_protocol():
    fake = FakeQuerier()
    fake.create_authors([query.CreateAuthorsParams(name="b", bio=None), query.CreateAuthorsParams(name="a", bio=None)])
    assert author_names(fake) == ["a", "b"]


def test_modern_generic_session_querier(db):
    apply_migrations(db, [os.path.dirname(__file__) + "/../library_modern/schema.sql"])
    with sqlalchemy.orm.Session(bind=db) as session:
        q = modern_query.Querier(session)
        assert q._conn is session
        a = q.create_author(name="Ursula", bio=None)
        assert a is not None
        assert a.created_at.tzinfo is not None
        book = q.create_book(author_id=a.id, title="The Dispossessed")
        assert book is not None
        assert book.status is modern_models.BookStatus.DRAFT
        row = q.get_book_with_author(id=book.id)
        assert row is not None
        assert isinstance(row, pydantic.BaseModel)
        assert row.authors == a
        with pytest.raises(modern_errors.UniqueViolationError):
            with session.begin_nested():
                q.create_author(name="Ursula", bio=None)


def test_modern_aware_datetime_rejects_naive():
    with pytest.raises(pydantic.ValidationError):
        modern_models.Author(id=1, name="a", bio=None, created_at=datetime.datetime(2024, 1, 1))
