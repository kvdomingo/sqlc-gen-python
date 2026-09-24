CREATE TABLE grids (
  id     BIGSERIAL PRIMARY KEY,
  matrix integer[][] NOT NULL,
  cube   integer[][][],
  tags   text[]
);
