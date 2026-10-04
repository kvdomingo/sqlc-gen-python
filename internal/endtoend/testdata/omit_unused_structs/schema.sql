CREATE TYPE status AS ENUM ('active', 'retired');
CREATE TYPE audit_kind AS ENUM ('insert', 'delete');
CREATE TYPE priority AS ENUM ('low', 'high');

CREATE TABLE authors (
  id     BIGSERIAL PRIMARY KEY,
  name   text NOT NULL,
  status status NOT NULL
);

CREATE TABLE audit_log (
  id   BIGSERIAL PRIMARY KEY,
  kind audit_kind NOT NULL
);

CREATE TABLE tasks (
  id       BIGSERIAL PRIMARY KEY,
  priority priority NOT NULL,
  title    text NOT NULL
);
