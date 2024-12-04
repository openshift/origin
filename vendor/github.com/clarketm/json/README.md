# [json](https://godoc.org/github.com/clarketm/json)
> Mirrors [golang/go](https://github.com/golang/go) [![Golang version](https://img.shields.io/badge/go-1.12.7-green)](https://github.com/golang/go/releases/tag/go1.12.7)

Drop-in replacement for Golang [`encoding/json`](https://golang.org/pkg/encoding/json/) with additional features.

## Installation
```shell
$ go get -u github.com/clarketm/json
```

## Usage
Same usage as Golang [`encoding/json`](https://golang.org/pkg/encoding/json/).

## Features
- Support zero values of structs with `omitempty`: [golang/go#11939](https://github.com/golang/go/issues/11939).
> If `omitempty` is applied to a struct and all the children of the struct are *empty*, then on marshalling it will be **omitted** from the encoded json.

## License
Refer to the [Golang](https://github.com/golang/go/blob/master/LICENSE) license. See [LICENSE](LICENSE) for more information.
