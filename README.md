# Pricewarp

A self-serve FOSS cryptocurrency price alert and portfolio tracking system.

With this tool you can keep track of cryptocurrency price changes and your crypto portfolio, without having to share your private information with third parties. This project uses an AGPL licence to ensure that it will always be available for those who need it. No frontend frameworks are employed to complicate matters, and Go is used as the backend to ensure timely and reliable delivery of content.

![price alert list](https://user-images.githubusercontent.com/3518142/155859069-5bd83752-8201-444b-887a-1df436b3531b.png)
![portfolio](https://user-images.githubusercontent.com/3518142/155859067-be96392a-16e8-4bcc-9b1e-a1ba629af1b0.png)
![example email](https://user-images.githubusercontent.com/3518142/154852169-13587064-2b98-4aa3-a2c2-e85ec3013375.png)

## Installation

You can install this application with the following steps.

1. Install Go 1.18
2. Create a `.env` file in the project
3. Run `./build.sh`
4. Create a Postgres user and database
5. Apply database migrations with `bin/migrate`

You may wish to set up a Postgres user for development like so:

```
sudo su postgres
psql

postgres=# CREATE ROLE some_user WITH LOGIN PASSWORD 'some_password';
postgres=# CREATE DATABASE some_database;
postgres=# \c some_database
some_database=# GRANT ALL ON ALL TABLES IN SCHEMA public to some_user;
some_database=# GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO some_user;
```

Your `.env` file should look like so:

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

The `DEBUG` flag enables serving files from `/static`, and other debugging
information. This should be set to `false` in production.

## Loading Price Data

Run `bin/ingest` to load cryptocurrency price data into the database. This
should be run periodically to get the latest prices for cryptocurrencies.
It is recommended to run this program before `bin/notify`.

### Reducing Database Size

Storing this price data can take up lots of space. You can condense the price
data into daily average prices by running `./condense-prices.sh`.

You can keep track of how much space your database is using like so:

```
$ sudo su postgres
$ psql

postgres=# select pg_size_pretty(pg_database_size('pricewarp'));
 pg_size_pretty 
----------------
 29 MB
(1 row)
```

You can consider running `VACUUM FULL ANALYZE;` to compress the price data as 
small as possible. Please refer to 
[the Postgres documentataion](https://www.postgresql.org/docs/current/sql-vacuum.html)
for information on vacuuming.

## Sending Email Alerts

Run `bin/notify` to send price alert emails using the SMTP credentials set in
the `.env` file. You should create test alerts to ensure emails will be delivered.
Popular mail hosts can reject mail for all kinds of reasons.

One easy way to ensure your mail will be delivered is to send with GMail as the SMTP
provider to a GMail address, or similar for other popular email providers.

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

  */10    *  *   *   *     cd /your/dir && bin/ingest
  1-59/10 *  *   *   *     cd /your/dir && bin/notify
  2       0  *   *   *     cd /your/dir && ./condense-prices.sh
```

You should configure Postgres to vacuum deleted rows for the database, or space
will not be reclaimed for condensed prices. You could use the following in the
crontab for the `postgres` user.

```cron
  3       0  *   *   *     psql pricewarp -qc 'vacuum full analyze;'
```

You could start your server right away with `nohup`.

```bash
nohup bin/pricewarp &> server.log &
```
