// Package api presents two independently developed and maintained Mesos API implementations.
// Clients should not consume this package directly.
//
// v0
//
// The "v0" subpackage has been tested for compatibility with the Mesos v0.2x release series and
// utilizes the "unversioned" or "v0" libprocess-based API presented by Mesos. Support for the v0
// API is on life-support and only critical bug fixes may be addressed. All current and future
// development effort focuses on the v1 Mesos API.
//
// v1
//
// The "v1" subpackage is compatible with the Mesos v1.x release series and utilizes the v1 HTTP
// API presented by Mesos. This is the recommended library to use for Mesos framework development.
package api
