CREATE TYPE status AS ENUM ('draft', 'published');

CREATE TABLE posts (
  id         BIGSERIAL PRIMARY KEY,
  title      text NOT NULL,
  tags       text[],
  status     status,
  payload    jsonb,
  created_at timestamptz NOT NULL
);
