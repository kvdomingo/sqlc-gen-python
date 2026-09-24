from typing import Iterator, Optional, Sequence

import sqlalchemy.engine
import sqlalchemy.ext.asyncio

from db import models
from db import query


def conforms(conn: sqlalchemy.engine.Connection, aconn: sqlalchemy.ext.asyncio.AsyncConnection) -> None:
    q: query.QuerierProtocol = query.Querier(conn)
    aq: query.AsyncQuerierProtocol = query.AsyncQuerier(aconn)


class FakeQuerier:
    def __init__(self) -> None:
        self.authors: dict[int, models.Author] = {}

    def create_authors(self, arg: Sequence[query.CreateAuthorsParams]) -> int:
        for a in arg:
            id = len(self.authors) + 1
            self.authors[id] = models.Author(id=id, name=a.name, bio=a.bio)
        return len(arg)

    def delete_author(self, *, id: int) -> None:
        self.authors.pop(id, None)

    def get_author(self, *, id: int) -> Optional[models.Author]:
        return self.authors.get(id)

    def get_authors_by_id(self, arg: Sequence[query.GetAuthorsByIDParams]) -> Iterator[Optional[models.Author]]:
        for a in arg:
            yield self.authors.get(a.id)

    def list_authors(self) -> Iterator[models.Author]:
        yield from self.authors.values()

    def list_books_by_author(self, arg: Sequence[query.ListBooksByAuthorParams]) -> Iterator[list[models.Book]]:
        for _ in arg:
            yield []


def use(q: query.QuerierProtocol) -> Optional[models.Author]:
    return q.get_author(id=1)


use(FakeQuerier())
