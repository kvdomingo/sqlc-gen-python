CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE members (
  id       BIGSERIAL PRIMARY KEY,
  email    citext NOT NULL,
  nickname text NOT NULL
);
