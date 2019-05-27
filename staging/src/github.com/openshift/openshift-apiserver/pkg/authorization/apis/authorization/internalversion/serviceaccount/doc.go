// serviceaccount is copied from k8s.io/kubernetes/pkg/serviceaccount to avoid an API dependency on k8s.io/kubernetes
// outside of the api types we rely upon.
// The contents of the package can't change without breaking lots of authentication and authorization.
// Using an internal package prevents leaks.
// Do not add more things here or modify values.
package serviceaccount
