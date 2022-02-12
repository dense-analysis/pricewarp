CREATE TABLE crypto_user (
    id serial,
    username text NOT NULL,
    password text NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT unique_username UNIQUE (username)
);

CREATE TABLE crypto_currency (
    id serial,
    ticker text NOT NULL,
    name text NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT unique_ticker UNIQUE (ticker)
);

CREATE TABLE crypto_price (
    id serial,
    "from" integer NOT NULL,
    "to" integer NOT NULL,
    time timestamp without time zone NOT NULL,
    value numeric(40, 20) NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY ("from") REFERENCES crypto_currency (id),
    FOREIGN KEY ("to") REFERENCES crypto_currency (id),
    CONSTRAINT positive_crypto_price CHECK (value > 0)
);

CREATE TABLE crypto_alert (
    id serial,
    user_id integer NOT NULL,
    "from" integer NOT NULL,
    "to" integer NOT NULL,
    value numeric(40, 20) NOT NULL,
    above boolean NOT NULL,
    time timestamp without time zone NOT NULL,
    sent boolean NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES crypto_user (id),
    FOREIGN KEY ("from") REFERENCES crypto_currency (id),
    FOREIGN KEY ("to") REFERENCES crypto_currency (id)
);
