BEGIN;

CREATE TEMPORARY TABLE crypto_price_temp (
    "from" integer NOT NULL,
    "to" integer NOT NULL,
    time timestamp without time zone NOT NULL,
    value numeric(40, 20) NOT NULL
);

-- Get the average price per day from before yesterday at midnight.
INSERT INTO crypto_price_temp
SELECT "from", "to", DATE_TRUNC('day', time) AS day, AVG(value)
FROM crypto_price
WHERE time < current_date - interval '1 day'
GROUP BY "from", "to", day
ORDER BY "from", "to", day;

-- Remove old records.
DELETE FROM crypto_price WHERE time < current_date - interval '1 day';

-- Insert aggregated records.
INSERT INTO crypto_price ("from", "to", time, value)
SELECT "from", "to", time, value FROM crypto_price_temp;

DROP TABLE crypto_price_temp;

COMMIT;
