from typing import Iterator, List, Optional

import sqlalchemy.engine
import sqlalchemy.ext.asyncio
import sqlalchemy.orm

from db import errors, models, query


def sync(conn: sqlalchemy.engine.Connection, session: sqlalchemy.orm.Session) -> None:
    q: query.QuerierProtocol = query.Querier(conn)
    s: query.QuerierProtocol = query.Querier(session)
    try:
        created = q.create_author(name="a", bio=None)
    except errors.UniqueViolationError as e:
        name: Optional[str] = e.constraint_name
        return
    except errors.QueryError as e:
        query_name: str = e.query_name
        return
    if created is not None:
        row = q.get_book_with_author(id=created.id)
        if row is not None:
            author: models.Author = row.authors
            book: models.Book = row.books
    found: Iterator[Optional[models.Author]] = q.get_authors_by_id([query.GetAuthorsByIDParams(id=1)])
    books: Iterator[List[models.Book]] = q.list_books_by_author([query.ListBooksByAuthorParams(author_id=1)])
    count: int = q.create_authors([query.CreateAuthorsParams(name="b", bio=None)])
    status: models.BookStatus = models.BookStatus.IN_PRINT


async def asynchronous(conn: sqlalchemy.ext.asyncio.AsyncConnection) -> None:
    q: query.AsyncQuerierProtocol = query.AsyncQuerier(conn)
    async for author in q.get_authors_by_id([query.GetAuthorsByIDParams(id=1)]):
        if author is not None:
            print(author.name)
    await q.create_books([query.CreateBooksParams(author_id=1, title="t", status=models.BookStatus.DRAFT, from_=None)])
