# Tasks

## 1. Generator

- [x] 1.1 In `internal/gen.go`, name the params `QueryValue` `args` when `takesSequence(query.Cmd)`, else `arg`; verify with `go build ./...`
- [x] 1.2 Remove the positional struct special case in `addArgs` so the params argument goes to `KwOnlyArgs` (keeping the `Sequence` wrap); verify with `go build ./...`
- [x] 1.3 Replace the literal `"arg"` in `sequenceParams` and the `:copyfrom` / `:batchexec` / `:batchone` / `:batchmany` bodies with `q.Args[0].Name`; verify with `go build ./...`

## 2. Regenerated output and hand-written callers

- [x] 2.1 Rebuild the plugin (`mise exec -- make all`) and run `sqlc generate` in each `internal/endtoend/testdata/*` dir; verify every params-class method reads `(self, *, arg: ...)` or `(self, *, args: Sequence[...])` with `grep -rn "self, arg" internal/endtoend/testdata` returning nothing
- [x] 2.2 Update `internal/endtoend/testdata/emit_querier_protocol/_check.py` fakes to `(self, *, args: Sequence[...])`; verify the testdata's type check (mypy/pyright as run for that dir) passes
- [x] 2.3 Regenerate `examples/` and update `examples/src/tests/test_library.py` fakes and call sites to `args=[...]`; verify `sqlc diff` in `examples/` is clean and `pytest src/tests` passes against Postgres
- [x] 2.4 Verify `mise exec -- make test` passes

## 3. Docs

- [x] 3.1 Update the README `:batchexec` example to `def create_authors(self, *, args: Sequence[CreateAuthorsParams])` / `for a in args`, and any other params-class example; verify `grep -n "self, arg" README.md` returns nothing
- [x] 3.2 Rewrite the alt-sqlc-gen-python migration bullet: batch/copyfrom methods take keyword-only `args: Sequence[Params]` (alt: `args: list[Params]`); note that non-sequence params-class methods now take keyword-only `arg`
