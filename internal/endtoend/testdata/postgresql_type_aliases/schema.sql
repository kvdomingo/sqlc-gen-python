CREATE TABLE everything (
  id        BIGSERIAL PRIMARY KEY,
  a_varchar varchar(10) NOT NULL,
  a_charvar character varying NOT NULL,
  a_bpchar  bpchar NOT NULL,
  a_char    character(3) NOT NULL,
  a_name    name NOT NULL,
  a_time    time NOT NULL,
  a_timetz  timetz NOT NULL,
  a_time_wo time without time zone NOT NULL,
  a_time_w  time with time zone NOT NULL,
  a_ts      timestamp NOT NULL,
  a_tstz    timestamptz NOT NULL,
  a_ts_wo   timestamp without time zone NOT NULL,
  a_ts_w    timestamp with time zone NOT NULL,
  a_json    json NOT NULL,
  a_jsonb   jsonb NOT NULL
);
