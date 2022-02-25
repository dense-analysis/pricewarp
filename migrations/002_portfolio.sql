CREATE TABLE crypto_portfolio (
    user_id integer NOT NULL,
    currency_id integer NOT NULL,
    cash numeric(40, 20) NOT NULL,
    PRIMARY KEY (user_id),
    FOREIGN KEY (user_id) REFERENCES crypto_user (id),
    FOREIGN KEY (currency_id) REFERENCES crypto_currency (id),
    CONSTRAINT positive_crypto_portfolio_cash CHECK (cash >= 0)
);

CREATE TABLE crypto_asset (
    user_id integer NOT NULL,
    currency_id integer NOT NULL,
    purchased numeric(40, 20) NOT NULL,
    amount numeric(40, 20) NOT NULL,
    PRIMARY KEY (user_id, currency_id),
    FOREIGN KEY (user_id) REFERENCES crypto_user (id),
    FOREIGN KEY (currency_id) REFERENCES crypto_currency (id),
    CONSTRAINT positive_crypto_asset_purchase CHECK (purchased >= 0),
    CONSTRAINT positive_crypto_asset_amount CHECK (amount >= 0)
);
