-- All schema is held in ClickHouse so we don't need additional systems for
-- storing credentials.

CREATE TABLE IF NOT EXISTS crypto_users
(
    user_id Int64,
    username LowCardinality(String),
    password_hash FixedString(60),
    created_at DateTime64(9),
    updated_at DateTime64(9),
    is_active UInt8
)
ENGINE = MergeTree
ORDER BY (username, updated_at);

CREATE TABLE IF NOT EXISTS crypto_currencies
(
    ticker LowCardinality(String),
    name LowCardinality(String),
    updated_at DateTime64(9)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (ticker);

CREATE TABLE IF NOT EXISTS crypto_currency_prices
(
    time DateTime64(9),
    from_currency_ticker LowCardinality(String),
    from_currency_name LowCardinality(String),
    to_currency_ticker LowCardinality(String),
    to_currency_name LowCardinality(String),
    value Decimal(40, 20),
    yearmonth UInt32 DEFAULT toInt32((toYear(time) * 100) + toMonth(time))
)
ENGINE = MergeTree
PARTITION BY yearmonth
ORDER BY (yearmonth, time, from_currency_ticker, to_currency_ticker)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS crypto_alert
(
    alert_id Int64,
    user_id Int64,
    username LowCardinality(String),
    from_currency_ticker LowCardinality(String),
    from_currency_name LowCardinality(String),
    to_currency_ticker LowCardinality(String),
    to_currency_name LowCardinality(String),
    value Decimal(40, 20),
    above UInt8,
    alert_time DateTime64(9),
    sent UInt8,
    updated_at DateTime64(9),
    is_deleted UInt8
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, alert_id, updated_at);

CREATE TABLE IF NOT EXISTS crypto_portfolio
(
    user_id Int64,
    username LowCardinality(String),
    currency_ticker LowCardinality(String),
    currency_name LowCardinality(String),
    cash Decimal(40, 20),
    updated_at DateTime64(9),
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, currency_ticker);

CREATE TABLE IF NOT EXISTS crypto_asset
(
    user_id Int64,
    username LowCardinality(String),
    currency_ticker LowCardinality(String),
    currency_name LowCardinality(String),
    purchased Decimal(40, 20),
    amount Decimal(40, 20),
    updated_at DateTime64(9),
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, currency_ticker);
