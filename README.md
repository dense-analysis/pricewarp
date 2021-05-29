# w0rpboard

This project contains some code so w0rp can play around with Go and React for
fun.

## Getting Started - Development

You need to follow some simple steps to get everything to run.

1. Set up Go in the usual way.
2. Run `yarn` to install all of the Node packages for the front end.
3. Copy `example-nginx.conf` to `/etc/nginx/sites-available`, tweak it, and
   reload nginx.
4. Run the back end API with `go run cmd/w0rpboard/main.go`.
5. Watch for changes in the front end with `yarn watch`.

## Production Builds

Run the following commands to produce production builds.

1. `make`
2. `yarn build`

The production go executable will be available as `bin/w0rpboard`, and the
production front end files will be in `dist`.
