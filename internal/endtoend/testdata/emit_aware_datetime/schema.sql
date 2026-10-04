CREATE TABLE events (
  id         BIGSERIAL PRIMARY KEY,
  a          timestamptz NOT NULL,
  b          timestamp NOT NULL,
  c          timestamp with time zone,
  d          timestamptz[]
);
