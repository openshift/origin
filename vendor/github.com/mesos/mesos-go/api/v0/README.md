# Go bindings for Apache Mesos

Very early version of a pure Go language bindings for Apache Mesos. As with other pure implementation, mesos-go uses the HTTP wire protocol to communicate directly with  a running Mesos master and its slave instances. One of the objectives of this project is to provide an idiomatic Go API that makes it super easy to create Mesos frameworks using Go. 

## Status
The Mesos v0 API version of the bindings are considered **alpha** and won't
see any major development besides critical compatibility and bug fixes.

New projects should use the Mesos v1 API bindings, located in `api/v1`.

### Features
- The SchedulerDriver API implemented
- The ExecutorDriver API implemented
- Stable API (based on the core Mesos code)
- Plenty of unit and integrative of tests
- Modular design for easy readability/extensibility
- Example programs on how to use the API
- Leading master detection
- Authentication via SASL/CRAM-MD5

### Pre-Requisites
- Go 1.3 or higher
- A standard and working Go workspace setup
- Apache Mesos 0.19 or newer

## Installing
Users of this library are encouraged to vendor it. API stability isn't guaranteed at this stage.
```shell
$ go get github.com/mesos/mesos-go
```

## Testing
```shell
$ (cd $GOPATH/src/github.com/mesos/mesos-go/api/v0; go test -race ./...)
```
