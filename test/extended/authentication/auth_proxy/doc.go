// Package auth_proxy provides forward proxy deployments and proxy configuration helpers
// for authentication end-to-end tests. It does not configure authentication
// consumers: callers publish the proxy URLs and CA through the relevant API.
//
// Functions that accept a context require a non-nil context and reject one that
// is already canceled or expired before starting work. Kubernetes requests and
// polling use that context. Commands run through exutil.CLI check cancellation
// before execution but cannot interrupt a command that is already running.
//
// A failed deployment can return a component with an allocated namespace. The
// caller owns cleanup of that namespace, using Delete with an active context.
package auth_proxy
