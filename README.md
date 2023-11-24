# rAPI

[![Go Reference](https://pkg.go.dev/badge/github.com/goinsane/rapi.svg)](https://pkg.go.dev/github.com/goinsane/rapi)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=goinsane_rapi&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=goinsane_rapi)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=goinsane_rapi&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=goinsane_rapi)

**rAPI** is a Go (Golang) package that simplifies building and consuming RESTful APIs. It provides an HTTP handler for
creating APIs and a client for making API requests.

## Features

### Handler features

- Handling by pattern and method
- Accepting query string or request body on GET and HEAD methods
- Setting various options by using HandlerOption's
- Middleware support as a HandleOption

### Caller features

- Calling by endpoint and method
- Ability to force request body in GET and HEAD methods
- Setting various options by using CallOption's

## Installation

You can install **rAPI** using the `go get` command:

```sh
go get github.com/goinsane/rapi
```

## Contributing

We welcome contributions from the community to improve and expand project capabilities. If you find a bug, have a
feature request, or want to contribute code, please follow our guidelines for contributing
([CONTRIBUTING.md](CONTRIBUTING.md)) and submit a pull request.

## License

This project is licensed under the [BSD 3-Clause License](LICENSE).
