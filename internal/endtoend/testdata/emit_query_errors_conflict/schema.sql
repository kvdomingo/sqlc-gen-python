CREATE TABLE authors (
  id   BIGSERIAL PRIMARY KEY,
  name text NOT NULL UNIQUE,
  bio  text
);
