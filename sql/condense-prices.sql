-- Remove non-aggregated rows before yesterday.
ALTER TABLE crypto_currency_prices
    DELETE WHERE time < now() - INTERVAL 1 DAY
    AND time != toStartOfDay(time);

-- Insert aggregated daily averages for older data.
INSERT INTO crypto_currency_prices
    (time, from_currency_id, from_currency_ticker, from_currency_name,
     to_currency_id, to_currency_ticker, to_currency_name, value, yearmonth)
SELECT
    toStartOfDay(time) AS time,
    from_currency_id,
    from_currency_ticker,
    from_currency_name,
    to_currency_id,
    to_currency_ticker,
    to_currency_name,
    avg(value) AS value,
    toUInt32((toYear(time) * 100) + toMonth(time)) AS yearmonth
FROM crypto_currency_prices
WHERE time < now() - INTERVAL 1 DAY
    AND time != toStartOfDay(time)
GROUP BY
    from_currency_id,
    from_currency_ticker,
    from_currency_name,
    to_currency_id,
    to_currency_ticker,
    to_currency_name,
    toStartOfDay(time);
