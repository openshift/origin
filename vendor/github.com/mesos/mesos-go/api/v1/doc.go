// Package v1 contains library and example subpackages for working with the Mesos v1 HTTP API.
// Clients should not consume this package directly.
//
// Library
//
// The v1 API is accessible via the "lib" subpackage. Consumers should import
// "github.com/mesos/mesos-go/api/v1/lib" and refer to its funcs and types via the "mesos" package,
// for example `mesos.Resource`.
//
// Examples
//
// See subpackage "cmd" for sample frameworks.
// See directory "docker" for an illustration of framework deployment via Docker and Marathon.
package v1
