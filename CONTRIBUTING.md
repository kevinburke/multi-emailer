# Contributing

You'll need a working Go installation: https://golang.org/doc/install

Get the source code for the multi-emailer:

```
go get github.com/kevinburke/multi-emailer
```

This will install it to $GOPATH/src/github.com/kevinburke/multi-emailer.

### Run the tests

Run `make test` to run the tests. Raise a Github issue if they don't pass; they
should pass.

### Run the server

Run `make serve` to start a local server. The server should log the port (8048)
and protocol (HTTP or HTTPS) it's running on.

### Watch/compile assets

Run `make watch` to watch for changes to CSS or templates and compile them into
the binary. They must be compiled into the Go binary (in assets/bindata.go) to
work, it's not enough to change the assets on disk.

## Ping me if you get stuck

Please open an issue or contact me directly if you get stuck at any point.
