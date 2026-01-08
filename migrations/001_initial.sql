CREATE TABLE IF NOT EXISTS crypto_user_login
(
    user_id Int64,
    username LowCardinality(String),
    password_hash String,
    created_at DateTime64(9),
    updated_at DateTime64(9),
    is_active UInt8
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, username);

CREATE TABLE IF NOT EXISTS crypto_currency
(
    id Int64,
    ticker LowCardinality(String),
    name LowCardinality(String),
    updated_at DateTime64(9)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (ticker, id);

CREATE TABLE IF NOT EXISTS crypto_currency_prices
(
    `time` DateTime64(9),
    `from_currency_id` Int64,
    `from_currency_ticker` LowCardinality(String),
    `from_currency_name` LowCardinality(String),
    `to_currency_id` Int64,
    `to_currency_ticker` LowCardinality(String),
    `to_currency_name` LowCardinality(String),
    `value` Float64,
    `yearmonth` UInt32 DEFAULT toInt32((toYear(time) * 100) + toMonth(time))
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
    from_currency_id Int64,
    from_currency_ticker LowCardinality(String),
    from_currency_name LowCardinality(String),
    to_currency_id Int64,
    to_currency_ticker LowCardinality(String),
    to_currency_name LowCardinality(String),
    value Float64,
    above UInt8,
    alert_time DateTime64(9),
    sent UInt8,
    updated_at DateTime64(9),
    is_deleted UInt8
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, alert_id, updated_at);
