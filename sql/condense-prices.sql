-- Insert aggregated daily averages for older data.
INSERT INTO crypto_currency_prices
    (time, from_currency_ticker, from_currency_name,
     to_currency_ticker, to_currency_name, value)
SELECT
    day AS time,
    from_currency_ticker,
    from_currency_name,
    to_currency_ticker,
    to_currency_name,
    avg(value) AS value
FROM (
    SELECT
        toStartOfDay(time) AS day,
        from_currency_ticker,
        from_currency_name,
        to_currency_ticker,
        to_currency_name,
        value
    FROM crypto_currency_prices
    WHERE time < now() - INTERVAL 1 DAY
        AND time != toStartOfDay(time)
)
GROUP BY
    from_currency_ticker,
    from_currency_name,
    to_currency_ticker,
    to_currency_name,
    day;

-- Remove non-aggregated rows before yesterday.
ALTER TABLE crypto_currency_prices
    DELETE WHERE time < now() - INTERVAL 1 DAY
    AND time != toStartOfDay(time);
