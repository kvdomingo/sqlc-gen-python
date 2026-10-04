CREATE TABLE trips (
  id     BIGSERIAL PRIMARY KEY,
  "from" text NOT NULL,
  "to"   text NOT NULL,
  class  text NOT NULL
);
