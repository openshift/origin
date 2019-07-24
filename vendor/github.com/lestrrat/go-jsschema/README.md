# go-jsschema

[![Build Status](https://travis-ci.org/lestrrat-go/jsschema.svg?branch=master)](https://travis-ci.org/lestrrat-go/jsschema)

[![GoDoc](https://godoc.org/github.com/lestrrat-go/jsschema?status.svg)](https://godoc.org/github.com/lestrrat-go/jsschema)

JSON Schema for Go

# SYNOPSIS

```go
package schema_test

import (
  "log"

  "github.com/lestrrat-go/jsschema"
  "github.com/lestrrat-go/jsschema/validator"
)

func Example() {
  s, err := schema.ReadFile("schema.json")
  if err != nil {
    log.Printf("failed to read schema: %s", err)
    return
  }

  for name, pdef := range s.Properties {
    // Do what you will with `pdef`, which contain
    // Schema information for `name` property
    _ = name
    _ = pdef
  }

  // You can also validate an arbitrary piece of data
  var p interface{} // initialize using json.Unmarshal...
  v := validator.New(s)
  if err := v.Validate(p); err != nil {
    log.Printf("failed to validate data: %s", err)
  }
}
```

# DESCRIPTION

This packages parses a JSON Schema file, and allows you to inspect, modify
the schema, but does nothing more.

If you want to validate using the JSON Schema that you read using this package,
look at [go-jsval](https://github.com/lestrrat-go/jsval), which allows you to
generate validators, so that you don't have to dynamically read in the JSON schema
for each instance of your program.

In the same lines, this package does not really care about loading external
schemas from various locations (it's just easier to just gather all the schemas
in your local system). It *is* possible to do this via [go-jsref](https://github.com/lestrrat-go/jsref)
if you really want to do it.

# BENCHMARKS

Latest version of libraries as of Sep 3 2016.

Benchmarks with [gojsonschema](https://github.com/xeipuuv/gojsonschema)
are prefixed with `Gojsonschema`.

Benchmarks without prefixes are about ths package.

```
$ go test -tags benchmark -benchmem -bench=.
BenchmarkGojsonschemaParse-4            5000        357330 ns/op      167451 B/op       1866 allocs/op
BenchmarkParse-4                     1000000          1577 ns/op        1952 B/op          9 allocs/op
BenchmarkParseAndMakeValidator-4      500000          2806 ns/op        2304 B/op         13 allocs/op
PASS
```

# TODO

* Properly resolve ids and $refs (it works in simple cases, but elaborate scopes probably don't work)

# Contributors

* Daisuke Maki
* Nate Finch

# References

| Name                                                     | Notes                            |
|:--------------------------------------------------------:|:---------------------------------|
| [go-jsval](https://github.com/lestrrat-go/jsval)         | Validator generator              |
| [go-jshschema](https://github.com/lestrrat-go/jshschema) | JSON Hyper Schema implementation |
| [go-jsref](https://github.com/lestrrat-go/jsref)         | JSON Reference implementation    |
| [go-jspointer](https://github.com/lestrrat-go/jspointer) | JSON Pointer implementations     |
