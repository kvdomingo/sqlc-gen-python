# postgresql-type-mapping Specification

## Purpose

Defines which Python type annotation the plugin emits for each PostgreSQL type,
aligned with the spellings that sqlc's Go codegen recognizes.

## Requirements

### Requirement: PostgreSQL type aliases

The plugin SHALL map these PostgreSQL type spellings to these Python types. It
SHALL do so whether or not the spelling is qualified with `pg_catalog.`:

| PostgreSQL spellings | Python type |
|---|---|
| `text`, `varchar`, `character varying`, `bpchar`, `character`, `name`, `citext`, `string` | `str` |
| `time`, `timetz`, `time without time zone`, `time with time zone` | `datetime.time` |
| `timestamp`, `timestamptz`, `timestamp without time zone`, `timestamp with time zone` | `datetime.datetime` |
| `json`, `jsonb` | `Any` |

Existing mappings for the other types SHALL stay the same. With
`emit_aware_datetime: true`, the `timestamptz` spellings (`timestamptz`,
`timestamp with time zone`) SHALL map to `pydantic.AwareDatetime` instead. The
codegen-config capability defines that option.

#### Scenario: Unqualified varchar

- **WHEN** a column's type reaches the plugin as `varchar`
- **THEN** it is annotated `str`, not `Any`

#### Scenario: Timestamp without time zone

- **WHEN** a column's type reaches the plugin as `timestamp without time zone`
- **THEN** it is annotated `datetime.datetime`

#### Scenario: Aware mapping only for timestamptz

- **WHEN** `emit_aware_datetime` and `emit_pydantic_models` are true, and a
  table has `a timestamptz NOT NULL` and `b timestamp NOT NULL`
- **THEN** `a` is annotated `pydantic.AwareDatetime` and `b` is annotated
  `datetime.datetime`

### Requirement: Unknown types fall back to Any

Any type that has no mapping, is not a known enum, and has no matching override
SHALL be annotated `Any`, and the plugin SHALL log it as an unknown PostgreSQL
type.

#### Scenario: Range type

- **WHEN** a column is `during tstzrange`
- **THEN** it is annotated `Any`
