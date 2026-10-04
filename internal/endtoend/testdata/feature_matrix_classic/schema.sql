CREATE TYPE book_status AS ENUM ('draft', 'in-print', 'out_of_print', '=');

COMMENT ON TYPE book_status IS 'Lifecycle of a book.
"Out of print" books stay listed.';

CREATE TABLE authors (
  id         BIGSERIAL PRIMARY KEY,
  name       text NOT NULL UNIQUE,
  bio        text,
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE authors IS 'People who write books.';

CREATE TABLE books (
  id        BIGSERIAL PRIMARY KEY,
  author_id bigint NOT NULL REFERENCES authors (id),
  title     text NOT NULL,
  status    book_status NOT NULL DEFAULT 'draft',
  "from"    date,
  tags      text[],
  CHECK (title <> '')
);

COMMENT ON COLUMN books."from" IS 'First published.
NULL until the book is released.';
