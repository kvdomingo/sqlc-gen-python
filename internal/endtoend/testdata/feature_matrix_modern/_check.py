from collections.abc import Iterator
from typing import assert_type

import pydantic
import sqlalchemy.engine
import sqlalchemy.ext.asyncio
import sqlalchemy.orm

from db import errors, models, query


def sync(conn: sqlalchemy.engine.Connection, session: sqlalchemy.orm.Session) -> None:
    q = query.Querier(session)
    assert_type(q._conn, sqlalchemy.orm.Session)
    p: query.QuerierProtocol = q
    c: query.QuerierProtocol = query.Querier(conn)
    try:
        created = q.create_author(name="a", bio=None)
    except errors.UniqueViolationError as e:
        name: str | None = e.constraint_name
        return
    if created is not None:
        assert_type(created.created_at, pydantic.AwareDatetime)
    found: Iterator[models.Author | None] = q.get_authors_by_id([query.GetAuthorsByIDParams(id=1)])
    books: Iterator[list[models.Book]] = q.list_books_by_author([query.ListBooksByAuthorParams(author_id=1)])
    status: models.BookStatus = models.BookStatus.OUT_OF_PRINT


async def asynchronous(session: sqlalchemy.ext.asyncio.AsyncSession) -> None:
    q = query.AsyncQuerier(session)
    assert_type(q._conn, sqlalchemy.ext.asyncio.AsyncSession)
    p: query.AsyncQuerierProtocol = q
    async for books in q.list_books_by_author([query.ListBooksByAuthorParams(author_id=1)]):
        for b in books:
            print(b.title, b.from_)
