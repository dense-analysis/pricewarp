-- Insert daily rollups only if they do not already exist
-- We want to keep only historical daily average prices
--
-- Compute reusable cutoffs
WITH
    toDateTime64(toStartOfDay(now()), 9) AS today_start,   -- midnight today
    toYYYYMM(addMonths(now(), -1))       AS min_yyyymm     -- earliest YYYYMM to scan
INSERT INTO crypto_currency_prices
    (time, from_currency_ticker, from_currency_name,
     to_currency_ticker, to_currency_name, value)
SELECT
    -- Ensure correct precision for DateTime64(9)
    toDateTime64(daily_rollups.day, 9) AS time,
    daily_rollups.from_currency_ticker,
    daily_rollups.from_currency_name,
    daily_rollups.to_currency_ticker,
    daily_rollups.to_currency_name,
    daily_rollups.avg_value AS value
FROM
(
    -- Aggregate raw intraday rows into daily averages
    SELECT
        toStartOfDay(time) AS day,
        from_currency_ticker,
        from_currency_name,
        to_currency_ticker,
        to_currency_name,
        avg(value) AS avg_value
    FROM crypto_currency_prices
    WHERE
        yearmonth >= min_yyyymm        -- limit partitions
        AND time < today_start         -- only days before today
        AND time != toStartOfDay(time) -- exclude midnight rows (rollups)
    GROUP BY
        day,
        from_currency_ticker,
        from_currency_name,
        to_currency_ticker,
        to_currency_name
) AS daily_rollups
LEFT JOIN
(
    -- Existing midnight rollup rows
    SELECT
        time,
        from_currency_ticker,
        from_currency_name,
        to_currency_ticker,
        to_currency_name
    FROM crypto_currency_prices
    WHERE
        yearmonth >= min_yyyymm
        AND time < today_start
        AND time = toStartOfDay(time)  -- only midnight rows
    GROUP BY
        time,
        from_currency_ticker,
        from_currency_name,
        to_currency_ticker,
        to_currency_name
) AS existing_rollups
ON
    -- Match same day and currency pair
    existing_rollups.time = toDateTime64(daily_rollups.day, 9)
    AND existing_rollups.from_currency_ticker = daily_rollups.from_currency_ticker
    AND existing_rollups.from_currency_name   = daily_rollups.from_currency_name
    AND existing_rollups.to_currency_ticker   = daily_rollups.to_currency_ticker
    AND existing_rollups.to_currency_name     = daily_rollups.to_currency_name
-- Insert only if no rollup exists
WHERE existing_rollups.time IS NULL;

-- Delete raw intraday rows after rollups are ensured
ALTER TABLE crypto_currency_prices DELETE
WHERE
    yearmonth >= toYYYYMM(addMonths(now(), -1))  -- same partition window
    AND time < toDateTime64(toStartOfDay(now()), 9)  -- only past days
    AND time != toStartOfDay(time);  -- remove intraday rows, keep rollups
