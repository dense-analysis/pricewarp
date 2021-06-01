# pricewarp

This project contains some code so w0rp can play around with Go and React for
fun.

## Getting Started - Development

You need to follow some simple steps to get everything to run.

1. Create an environment variable file with `cp env.sh.skeleton env.sh`, and
   input data for your system.
2. Run database migrations with `./migrate.sh`.
3. Set up Go in the usual way.
4. Run `yarn` to install all of the Node packages for the front end.
5. Copy `example-nginx.conf` to `/etc/nginx/sites-available`, tweak it, and
   reload nginx.
6. Run the back end API with `go run cmd/pricewarp/main.go`.
7. Watch for changes in the front end with `yarn watch`.

## Production Builds

Run the following commands to produce production builds.

1. `make`
2. `yarn build`

The production go executable will be available as `bin/pricewarp`, and the
production front end files will be in `dist`.

You will need to create an environment file to run the executable with.
