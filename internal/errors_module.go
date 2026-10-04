package python

import (
	"strings"

	pyast "github.com/sqlc-dev/sqlc-gen-python/internal/ast"
	"github.com/sqlc-dev/sqlc-gen-python/internal/poet"
	pyprint "github.com/sqlc-dev/sqlc-gen-python/internal/printer"
)

const errorsFileName = "errors.py"

// The class names, constructor and query_name/cause attributes match
// alt-sqlc-gen-python's emit_query_errors so code written against it keeps
// working. {opt_exc}, {opt_str} and {iter_none} are filled in for the active
// type syntax.
const errorsModuleBody = `class QueryError(Exception):
    """Base class for all query errors."""

    def __init__(self, message: str, query_name: str, cause: {opt_exc} = None):
        super().__init__(message)
        self.query_name = query_name
        self.cause = cause
        self.constraint_name: {opt_str} = _constraint_name(cause)


class UniqueViolationError(QueryError):
    """Raised on unique constraint violation (PG 23505)."""


class ForeignKeyViolationError(QueryError):
    """Raised on foreign key constraint violation (PG 23503)."""


class CheckViolationError(QueryError):
    """Raised on check constraint violation (PG 23514)."""


class NotNullViolationError(QueryError):
    """Raised on not-null constraint violation (PG 23502)."""


class ExclusionViolationError(QueryError):
    """Raised on exclusion constraint violation (PG 23P01)."""


class StatementTimeoutError(QueryError):
    """Raised on statement timeout (PG 57014)."""


class DeadlockError(QueryError):
    """Raised on deadlock detected (PG 40P01)."""


class SerializationError(QueryError):
    """Raised on serialization failure (PG 40001)."""


def _sqlstate(e: sqlalchemy.exc.DBAPIError) -> {opt_str}:
    # psycopg2 exposes pgcode, psycopg 3 sqlstate; SQLAlchemy's asyncpg
    # adapter keeps the asyncpg error, which has sqlstate, as __cause__.
    for source in (e.orig, getattr(e.orig, "__cause__", None)):
        code = getattr(source, "pgcode", None) or getattr(source, "sqlstate", None)
        if isinstance(code, str):
            return code
    return None


def _constraint_name(cause: {opt_exc}) -> {opt_str}:
    orig = getattr(cause, "orig", None)
    name = getattr(getattr(orig, "diag", None), "constraint_name", None)
    if name is None:
        name = getattr(getattr(orig, "__cause__", None), "constraint_name", None)
    return name if isinstance(name, str) else None


def _wrap_integrity_error(e: sqlalchemy.exc.IntegrityError, query_name: str) -> QueryError:
    pgcode = _sqlstate(e)
    if pgcode == "23505":
        return UniqueViolationError(str(e), query_name, cause=e)
    if pgcode == "23503":
        return ForeignKeyViolationError(str(e), query_name, cause=e)
    if pgcode == "23514":
        return CheckViolationError(str(e), query_name, cause=e)
    if pgcode == "23502":
        return NotNullViolationError(str(e), query_name, cause=e)
    if pgcode == "23P01":
        return ExclusionViolationError(str(e), query_name, cause=e)
    return QueryError(str(e), query_name, cause=e)


def _wrap_operational_error(e: sqlalchemy.exc.OperationalError, query_name: str) -> QueryError:
    pgcode = _sqlstate(e)
    if pgcode == "57014":
        return StatementTimeoutError(str(e), query_name, cause=e)
    if pgcode == "40P01":
        return DeadlockError(str(e), query_name, cause=e)
    if pgcode == "40001":
        return SerializationError(str(e), query_name, cause=e)
    return QueryError(str(e), query_name, cause=e)


@contextlib.contextmanager
def _wrap_errors(query_name: str) -> {iter_none}:
    """Re-raise integrity and operational errors as typed QueryErrors.

    Only those two SQLAlchemy exceptions are caught, so GeneratorExit from a
    consumer that stops iterating early passes through untouched.
    """
    try:
        yield
    except sqlalchemy.exc.IntegrityError as e:
        raise _wrap_integrity_error(e, query_name) from e
    except sqlalchemy.exc.OperationalError as e:
        raise _wrap_operational_error(e, query_name) from e
`

func buildErrorsModule(ctx *pyTmplCtx) string {
	f := newPyFile(ctx.C, false)
	f.importModule("contextlib")
	f.importModule("sqlalchemy.exc")
	render := func(n *pyast.Node) string {
		return strings.TrimSpace(string(pyprint.Print(n, pyprint.Options{}).Python))
	}
	body := strings.NewReplacer(
		"{opt_exc}", render(f.optional(poet.Name("Exception"))),
		"{opt_str}", render(f.optional(poet.Name("str"))),
		"{iter_none}", render(f.generic("Iterator", poet.Constant(nil))),
	).Replace(errorsModuleBody)

	mod := moduleNode(ctx.SqlcVersion, "", ctx.C.OmitSqlcVersion)
	mod.Body = append(mod.Body, importGroup(f.std), importGroup(f.pkg))
	header := string(pyprint.Print(poet.Node(mod), pyprint.Options{}).Python)
	return header + "\n" + body
}
