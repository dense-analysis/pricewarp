CREATE TABLE IF NOT EXISTS crypto_portfolio
(
    user_id Int64,
    username LowCardinality(String),
    currency_id Int64,
    currency_ticker LowCardinality(String),
    currency_name LowCardinality(String),
    cash Float64,
    updated_at DateTime64(9),
    is_deleted UInt8
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, currency_id);

CREATE TABLE IF NOT EXISTS crypto_asset
(
    user_id Int64,
    username LowCardinality(String),
    currency_id Int64,
    currency_ticker LowCardinality(String),
    currency_name LowCardinality(String),
    purchased Float64,
    amount Float64,
    updated_at DateTime64(9),
    is_deleted UInt8
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (user_id, currency_id);
