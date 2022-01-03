# pricewarp

This project sets up a self-serve cyrptocurrency price alert system to send
emails when cryptocurrencies go above or below a certain price. _This is a work
in progress._

## Installation

1. Install Go 1.18
2. Create a `.env` file in the project
3. Run `./build.sh`
4. Create a Postgres user and database
5. Apply database migrations with `bin/migrate`

Easy set up for a Postgres user for development looks like so:

```
sudo su postgres
psql

postgres=# CREATE ROLE some_user WITH LOGIN PASSWORD 'some_password';
postgres=# CREATE DATABASE some_database;
postgres=# \c some_database
some_database=# GRANT ALL ON ALL TABLES IN SCHEMA public to some_user;
```

Your `.env` file should look like so.

```
DB_USERNAME=some_user
DB_PASSWORD=some_password
DB_HOST=localhost
DB_PORT=5432
DB_NAME=some_database

SMTP_USERNAME=email_username
SMTP_PASSWORD=email_password
SMTP_FROM=email_username@email.host
SMTP_HOST=email.host
SMTP_PORT=465
```

## Loading Price Data

Run `bin/ingest` to load cryptocurrency price data into the database. This
should be run periodically to get the latest prices for cryptocurrencies.

## Sending Email Alerts

Run `bin/notify` to send price alert emails using the SMTP credentials set in
the `.env` file.

## Running the Server

_TO DO_
