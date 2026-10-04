CREATE TABLE authors (
  id   BIGSERIAL PRIMARY KEY,
  name text NOT NULL UNIQUE,
  bio  text
);

CREATE TABLE jobs (
  id     BIGSERIAL PRIMARY KEY,
  errors text NOT NULL,
  "cast" text
);
