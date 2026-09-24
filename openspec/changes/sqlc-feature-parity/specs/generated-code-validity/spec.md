# Spec Delta

## Purpose

Guarantees that every generated module is syntactically valid Python that imports cleanly, whatever the identifiers, comments, and enum values in the schema are.

## ADDED Requirements

### Requirement: Python keywords are escaped
Any generated field, parameter, or method name that is a Python keyword SHALL get a trailing underscore (for example `from` becomes `from_`, and `class` becomes `class_`). A trailing underscore SHALL also apply when the name was set through `rename`. SQL text and parameter placeholders SHALL be unchanged.

#### Scenario: Keyword column
- **WHEN** a table has a column named `from`
- **THEN** the model declares `from_`, and queries that return the column build the model with `from_=cast(..., row[n])`

#### Scenario: Keyword parameter
- **WHEN** a query parameter is named `class`
- **THEN** the method has the keyword-only parameter `class_`, and the parameter dict maps the placeholder to `class_`

### Requirement: Empty class bodies
A generated class with no fields and no docstring SHALL have the body `pass`.

#### Scenario: Params class for a query without parameters
- **WHEN** `query_parameter_limit: 0` is set and a query has no parameters
- **THEN** the generated params class body is `pass`, and the module imports without a `SyntaxError`

### Requirement: Valid enum member names
Every enum member name SHALL be a valid, non-keyword Python identifier, and member names within one enum SHALL be unique. A value that sanitizes to an empty string SHALL be named `VALUE_<n>`, where `<n>` is the value's 1-based position. A name that starts with a digit SHALL get the prefix `VALUE_`. A name that duplicates an earlier member SHALL get the suffix `_<k>`, starting at `_2`. Member values SHALL keep the original strings.

#### Scenario: Symbol-only value
- **WHEN** an enum has values `=` and `<>`, in that order
- **THEN** the members are `VALUE_1 = "="` and `VALUE_2 = "<>"`

#### Scenario: Colliding values
- **WHEN** an enum has values `in-progress` and `in_progress`
- **THEN** the members are `IN_PROGRESS = "in-progress"` and `IN_PROGRESS_2 = "in_progress"`

#### Scenario: Digit-leading value
- **WHEN** an enum has the value `1st`
- **THEN** the member is `VALUE_1ST = "1st"`

### Requirement: Multi-line comments
A table, column, or enum comment that spans more than one line SHALL be emitted so that each line stays inside a Python comment or docstring. No comment text SHALL appear as bare code.

#### Scenario: Multi-line column comment
- **WHEN** a column comment is `"first line\nsecond line"`
- **THEN** the field is followed by `# first line` and a second line `# second line`, and the module parses

### Requirement: String literals are escaped
Every string literal the plugin emits SHALL be a valid Python literal that evaluates to exactly the source text. This covers enum values, docstrings from table, enum, and query comments, and SQL text. Backslashes and quote characters in the source text SHALL be escaped.

#### Scenario: Enum value with a double quote
- **WHEN** an enum has the value `say "hi"`
- **THEN** the member's value evaluates in Python to the string `say "hi"`

#### Scenario: Multi-line table comment
- **WHEN** a table comment spans two lines
- **THEN** the model's docstring is one triple-quoted string that contains both lines
