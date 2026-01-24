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
4. Create a ClickHouse database and user
5. Apply `sql/schema.sql` in ClickHouse

You may wish to set up a ClickHouse database and user for development like so:

```
clickhouse-client --multiquery <<'SQL'
CREATE DATABASE some_database;
CREATE USER some_user IDENTIFIED WITH sha256_password BY 'some_password';
GRANT ALL ON some_database.* TO some_user;
SQL
```

Your `.env` file should look like so:

```
DEBUG=true
ADDRESS=:8000

DB_USERNAME=some_user
DB_PASSWORD=some_password
DB_HOST=localhost
DB_PORT=9000
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
data into daily average prices by running `scripts/condense-prices.sh`.

To inspect ClickHouse storage usage, query `system.parts` for the table sizes
and consider adding TTL rules if you want to expire older price data.

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

You could start your server right away with `nohup`.

```bash
nohup bin/pricewarp &> server.log &
```
