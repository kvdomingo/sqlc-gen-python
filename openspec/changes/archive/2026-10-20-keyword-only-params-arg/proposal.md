# Proposal

## Why

When a query's parameters are packed into a params class (more parameters than
`query_parameter_limit`, a limit of `0`, or a sequence-taking command), the
generated method takes it as one positional `arg`. Every other generated
parameter is keyword-only, so this one is the odd case out, and the batch form
also diverges from asavoy/alt-sqlc-gen-python, which uses a keyword-only `args`.

## What Changes

- **BREAKING**: A params-class argument is keyword-only. Callers write
  `q.update_author(arg=...)`, not `q.update_author(...)`.
- **BREAKING**: For sequence-taking commands (`:copyfrom`, `:batchexec`,
  `:batchone`, `:batchmany`) the argument is named `args`:
  `def create_authors(self, *, args: Sequence[CreateAuthorsParams])`.
- Non-sequence commands keep the name `arg`, now keyword-only:
  `def update_author(self, *, arg: UpdateAuthorParams)`.
- Annotation stays `Sequence[Params]` (not alt's `list[Params]`).
- Querier protocols mirror the new signatures.
- README examples and the alt-sqlc-gen-python migration note are updated.

## Capabilities

### New Capabilities

### Modified Capabilities

- `query-codegen`: Copyfrom and Batch commands requirements change from one
  positional argument to keyword-only `args`; a new requirement pins the
  keyword-only `arg` for non-sequence params-class methods.

## Impact

- `internal/gen.go`: `addArgs`, the params-class `QueryValue` name, and the
  sequence-command bodies (`if not arg`, `for a in arg`).
- Regenerated `internal/endtoend/testdata/**` and `examples/src/**` output;
  hand-written `_check.py` fakes and `examples/src/tests` that call these methods.
- `README.md`.
- Downstream callers that pass the params object positionally or by `arg=` on
  batch/copyfrom methods must update.
