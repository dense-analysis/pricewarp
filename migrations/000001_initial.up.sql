BEGIN;

-- A user on the site
CREATE TABLE "user" (
    id SERIAL PRIMARY KEY,
    email VARCHAR(254) UNIQUE NOT NULL,
    -- Stores a password hash with the hashing mechanism used
    password VARCHAR(128) NOT NULL
);

-- Stores complete Binance dumps, so no information is lost
CREATE TABLE binance_dump (
    id SERIAL PRIMARY KEY,
    data jsonb NOT NULL,
    time timestamp WITH TIME ZONE NOT NULL
);

CREATE INDEX binance_dump_time_index
ON binance_dump (time);

-- A description of some currency
CREATE TABLE currency (
    id SERIAL PRIMARY KEY,
    ticker text NOT NULL UNIQUE,
    name text NOT NULL,
    -- A path to an icon image for this currency
    icon text
);

-- A price from one currency to another
CREATE TABLE price (
    id SERIAL PRIMARY KEY,
    from_currency_id integer REFERENCES currency NOT NULL,
    to_currency_id integer REFERENCES currency NOT NULL,
    price numeric NOT NULL,
    time timestamp WITH TIME ZONE NOT NULL
);

CREATE INDEX price_pair_index
ON price (from_currency_id, to_currency_id, time);

-- A price condition set up by a user
CREATE TABLE price_condition (
    id SERIAL PRIMARY KEY,
    user_id integer REFERENCES "user" NOT NULL,
    from_currency_id integer REFERENCES currency NOT NULL,
    to_currency_id integer REFERENCES currency NOT NULL,
    price numeric NOT NULL,
    above boolean NOT NULL,
    last_notified timestamp WITH TIME ZONE
);

CREATE INDEX price_condition_index
ON price_condition (user_id, last_notified);

COMMIT;
