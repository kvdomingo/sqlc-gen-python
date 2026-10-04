CREATE TYPE op AS ENUM ('=', '<>', 'in-progress', 'in_progress', '1st', 'say "hi"', 'back\slash');

COMMENT ON TYPE op IS 'Operators with "quoted" names';

CREATE TABLE rules (
  id BIGSERIAL PRIMARY KEY,
  op op NOT NULL
);
