from typing import assert_type

import sqlalchemy.engine
import sqlalchemy.ext.asyncio
import sqlalchemy.orm

from classic import query as classic
from modern import query as modern


def sync(session: sqlalchemy.orm.Session, conn: sqlalchemy.engine.Connection) -> None:
    q = classic.Querier(session)
    assert_type(q, classic.Querier[sqlalchemy.orm.Session])
    assert_type(q._conn, sqlalchemy.orm.Session)
    assert_type(classic.Querier(conn)._conn, sqlalchemy.engine.Connection)
    assert_type(modern.Querier(session)._conn, sqlalchemy.orm.Session)

    p: classic.QuerierProtocol = classic.Querier(session)
    mp: modern.QuerierProtocol = modern.Querier(conn)

    classic.Querier(object())  # type: ignore[type-var]
    modern.Querier(object())  # type: ignore[type-var]


def asynchronous(session: sqlalchemy.ext.asyncio.AsyncSession) -> None:
    q = classic.AsyncQuerier(session)
    assert_type(q._conn, sqlalchemy.ext.asyncio.AsyncSession)
    p: classic.AsyncQuerierProtocol = q
    mp: modern.AsyncQuerierProtocol = modern.AsyncQuerier(session)
