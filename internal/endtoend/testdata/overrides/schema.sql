CREATE TABLE users (
  id     uuid PRIMARY KEY,
  org_id uuid NOT NULL,
  extra  jsonb
);

CREATE TABLE authors (
  id      uuid PRIMARY KEY,
  name    text NOT NULL,
  avatar  bytea NOT NULL
);

CREATE TABLE events (
  id      BIGSERIAL PRIMARY KEY,
  payload jsonb NOT NULL,
  extra   jsonb,
  history jsonb[],
  score   double precision NOT NULL,
  rank    bigint NOT NULL
);
