-- Insert daily rollups of average prices per day.
-- This keeps the database smaller in size for historical price storage.
INSERT INTO crypto_currency_prices
(
    time,
    from_currency_ticker,
    from_currency_name,
    to_currency_ticker,
    to_currency_name,
    value
)
SELECT
    toStartOfDay(time),
    from_currency_ticker,
    from_currency_name,
    to_currency_ticker,
    to_currency_name,
    avg(value)
FROM crypto_currency_prices
-- Filter on yearmonth to limit partition scans and impact
WHERE yearmonth >= toYYYYMM(addMonths(now(), -1))
-- Work with data before today
AND time < today()
-- Ignore prices logged on midnight, as they may be previous rollups
AND time != toStartOfDay(time)
GROUP BY ALL;

-- Remove non rolled-up prices
DELETE FROM crypto_currency_prices
-- Filter on yearmonth to limit partition scans and impact
WHERE yearmonth >= toYYYYMM(addMonths(now(), -1))
-- Work with data before today
AND time < today()
-- Only remove non rolled-up prices
AND time != toStartOfDay(time);
