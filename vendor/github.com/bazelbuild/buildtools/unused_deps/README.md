# Unused Deps

unused_deps is a command line tool to determine any unused dependencies
in [java_library](https://docs.bazel.build/versions/master/be/java.html#java_library)
rules. targets.  It outputs `buildozer` commands to apply the suggested
prunings.

## Dependencies

1. Protobuf go runtime: to download (if not using bazel)
`go get -u github.com/golang/protobuf/{proto,protoc-gen-go}`


## Installation

1. Change directory to the buildifier/unused_deps

```bash
gopath=$(go env GOPATH)
cd $gopath/src/github.com/bazelbuild/buildtools/unused_deps
```

2. Install

```bash
go install
```

## Usage

```shell
unused_deps TARGET...
```

Here, `TARGET` is a space-separated list of Bazel labels, with support for `:all` and `...`
