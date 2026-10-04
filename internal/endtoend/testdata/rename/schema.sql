CREATE TYPE genre AS ENUM ('rock', 'hip-hop');

CREATE TABLE people (
  id    BIGSERIAL PRIMARY KEY,
  name  text NOT NULL,
  class text NOT NULL
);

CREATE TABLE artists (
  id          BIGSERIAL PRIMARY KEY,
  spotify_url text,
  kind        genre NOT NULL
);
