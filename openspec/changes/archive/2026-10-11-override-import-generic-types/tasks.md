# Tasks

## 1. Config

- [x] 1.1 Add `Many bool \`json:"many,omitempty"\`` to `Override` in `internal/config.go`; verify `go build ./...`
- [x] 1.2 In `Override.parse`, replace the dot-only `py_import` check with `^[A-Za-z_]\w*$` and the error from design.md. In `TestParseConfigInvalidOverrides`, update the "bare name" case to the new message and add `{"db_type": "jsonb", "py_type": "list[X]", "py_import": "m"}`; verify with `go test ./internal/`

## 2. Codegen

- [x] 2.1 In `makePyType` (`internal/gen.go`), add one to `typ.ArrayDims` when the matched override has `Many`; verify through task 3.1

## 3. End-to-end and docs

- [x] 3.1 In `internal/endtoend/testdata/overrides/`, add `tags text NOT NULL` and `labels text[]` to `events` in `schema.sql`, and overrides `{column: "events.tags", py_type: Tag, py_import: my_lib.tags, many: true}` and `{column: "events.labels", py_type: Tag, py_import: my_lib.tags, many: true}` in `sqlc.yaml`. Rebuild with `make all`, run `sqlc generate` there, and check that `db/models.py` has `from my_lib.tags import Tag`, `tags: List[Tag]` (the fixture does not set `emit_modern_types`) and `labels: Optional[List[List[Tag]]]`. Verify with `make test`
- [x] 3.2 Document `many` in the README "Type Overrides" section (around line 386) with the `DomainEntity` example, and note that `py_type` must be a single identifier with `py_import`; verify by reading the section
