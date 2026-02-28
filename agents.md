# Agent Notes

## Read-Only ClickHouse Inspection

Use the local helper command with `go run` when you need to inspect database state:

```bash
go run ./cmd/chquery --query "SELECT * FROM crypto_currencies ORDER BY ticker LIMIT 20"
```

The command is intentionally read-only and only allows a single `SELECT`, `SHOW`, or `EXPLAIN` statement.
It rejects write keywords and `INTO OUTFILE`.

### Useful examples

```bash
go run ./cmd/chquery --query "SHOW TABLES"
go run ./cmd/chquery --query "SELECT from_currency_ticker, to_currency_ticker, value FROM crypto_currency_prices ORDER BY time DESC LIMIT 10"
go run ./cmd/chquery --query "EXPLAIN SELECT * FROM crypto_currency_prices WHERE from_currency_ticker = 'BTC'"
```

By default, long values are truncated for readability.
Add `--full` to print full values:

```bash
go run ./cmd/chquery --query "SELECT * FROM crypto_currency_prices ORDER BY time DESC LIMIT 3" --full
```
