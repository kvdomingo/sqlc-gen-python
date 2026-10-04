CREATE TYPE book_status AS ENUM ('available', 'checked_out', 'overdue');


CREATE TABLE books (
          id     BIGSERIAL PRIMARY KEY,
          title  text      NOT NULL,
          status book_status DEFAULT 'available'
);

CREATE TYPE op AS ENUM ('=', '<>', 'in-progress', 'in_progress', '1st', 'say "hi"', 'back\slash');

CREATE TABLE rules (
  id BIGSERIAL PRIMARY KEY,
  op op NOT NULL
);
