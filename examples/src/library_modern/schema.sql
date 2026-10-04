CREATE TYPE book_status AS ENUM ('draft', 'in-print', 'out_of_print');

CREATE TABLE authors (
  id         BIGSERIAL PRIMARY KEY,
  name       text NOT NULL,
  bio        text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT authors_name_key UNIQUE (name)
);

CREATE TABLE books (
  id        BIGSERIAL PRIMARY KEY,
  author_id bigint NOT NULL,
  title     text NOT NULL,
  status    book_status NOT NULL DEFAULT 'draft',
  CONSTRAINT books_author_id_fkey FOREIGN KEY (author_id) REFERENCES authors (id),
  CONSTRAINT books_title_check CHECK (title <> '')
);
