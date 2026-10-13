# go-jspointer

[![Build Status](https://travis-ci.org/lestrrat-go/jspointer.svg?branch=master)](https://travis-ci.org/lestrrat-go/jspointer)

[![GoDoc](https://godoc.org/github.com/lestrrat-go/jspointer?status.svg)](https://godoc.org/github.com/lestrrat-go/jspointer)

JSON pointer for Go

# Features

* Compile and match against Maps, Slices, Structs (or pointers to those)
* Set values in each of those

# Usage

```go
p, _ := jspointer.New(`/foo/bar/baz`)
result, _ := p.Get(someStruct)
```

# Credits

This is almost a fork of https://github.com/xeipuuv/gojsonpointer.

# References

| Name                                                     | Notes                            |
|:--------------------------------------------------------:|:---------------------------------|
| [go-jsval](https://github.com/lestrrat-go/jsval)         | Validator generator              |
| [go-jsschema](https://github.com/lestrrat-go/jsschema)   | JSON Schema implementation       |
| [go-jshschema](https://github.com/lestrrat-go/jshschema) | JSON Hyper Schema implementation |
| [go-jsref](https://github.com/lestrrat-go/jsref)         | JSON Reference implementation    |


