# pricewarp

This project sets up a self-serve cyrptocurrency price alert system to send
emails when cryptocurrencies go above or below a certain price.

![price alert list](https://user-images.githubusercontent.com/3518142/154852055-39d3111a-6582-4df0-9296-6b7d11068da7.png)
![price alert form](https://user-images.githubusercontent.com/3518142/154852057-89e3120c-3712-40fd-9226-04fc40646246.png)
![example email](https://user-images.githubusercontent.com/3518142/154852169-13587064-2b98-4aa3-a2c2-e85ec3013375.png)

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
DEBUG=true
ADDRESS=:8000

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

SESSION_SECRET=some_32_char_secret_cookie_value
```

The `DEBUG` flag enables serving files from `/static` and other debugging
information. This should be set to `false` in production.

## Loading Price Data

Run `bin/ingest` to load cryptocurrency price data into the database. This
should be run periodically to get the latest prices for cryptocurrencies.

Storing this price data can take up lots of space. You can condense the price
data into daily average prices by running `./condense-prices.sh`.

## Sending Email Alerts

Run `bin/notify` to send price alert emails using the SMTP credentials set in
the `.env` file.

## Creating users

Run `bin/adduser EMAIL PASSWORD` to add a user with a given email address and
password. You should be able to log in to the site after the program completes
successfully.

## Running the Server

This section will describe running the server with nginx.

Copy at least the contents of `bin` and `static` to a directory on a webserver,
and set up nginx to serve your static content and work as a reverse proxy.

```nginx
server {
    # ...

    location /static {
        include expires_headers;
        root /your/dir;
    }

    location / {
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Host $http_host;
        proxy_set_header REMOTE_ADDR $remote_addr;
        proxy_redirect off;

        # Use the port you're running the server with here.
        proxy_pass http://localhost:8000/;
    }
}
```

You can set cron rules to start the server up on boot, and to periodically load
price data and send email alerts.

```cron
@reboot cd /your/dir && bin/pricewarp &> server.log

  0/10 *  *   *   *     cd /your/dir && bin/ingest
  1/10 *  *   *   *     cd /your/dir && bin/notify
  2    0  *   *   *     cd /your/dir && ./condense-prices.sh
```

You could start your server right away with `nohup`.

```bash
nohup bin/pricewarp &> server.log &
```
