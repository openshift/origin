// Package auth_proxy deploys forward proxies for authentication end-to-end tests.
// It does not configure consumers: callers publish the URLs and CA through the
// relevant API.
//
// Kubernetes requests and polling honor context cancellation, but a command already
// running through exutil.CLI cannot be interrupted.
//
// A failed deployment can still own a namespace, which the caller must Delete.
package auth_proxy
