CREATE TYPE mood AS ENUM ('happy', 'sad');

COMMENT ON TYPE mood IS 'How someone feels.
Two values only.';

CREATE TABLE authors (
  id   BIGSERIAL PRIMARY KEY,
  name text NOT NULL,
  mood mood
);

COMMENT ON TABLE authors IS 'People who write books.
Second line with "quotes".';

COMMENT ON COLUMN authors.name IS 'first line
second line';
