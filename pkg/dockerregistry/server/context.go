package server

import (
	"github.com/docker/distribution/context"

	"github.com/openshift/origin/pkg/client"
)

type contextKey string

const (
	// repositoryKey serves to store/retrieve repository object to/from context.
	repositoryKey contextKey = "repository"

	// remoteBlobAccessCheckEnabledKey is the key for the flag in Contexts
	// to allow blobDescriptorService to stat remote blobs.
	remoteBlobAccessCheckEnabledKey contextKey = "remoteBlobAccessCheckEnabled"

	// registryClientKey is the key for RegistryClient values in Contexts.
	registryClientKey contextKey = "registryClient"

	// userClientKey is the key for a origin's client with the current user's
	// credentials in Contexts.
	userClientKey contextKey = "userClient"

	// authPerformedKey is the key to indicate that authentication was
	// performed in Contexts.
	authPerformedKey contextKey = "authPerformed"

	// deferredErrorsKey is the key for deferred errors in Contexts.
	deferredErrorsKey contextKey = "deferredErrors"
)

// withRepository returns a new Context that carries value repo.
func withRepository(parent context.Context, repo *repository) context.Context {
	return context.WithValue(parent, repositoryKey, repo)
}

// repositoryFrom returns the repository value stored in ctx, if any.
func repositoryFrom(ctx context.Context) (repo *repository, found bool) {
	repo, found = ctx.Value(repositoryKey).(*repository)
	return
}

// withRemoteBlobAccessCheckEnabled returns a new Context that allows
// blobDescriptorService to stat remote blobs. It is useful only in case
// of manifest verification.
func withRemoteBlobAccessCheckEnabled(parent context.Context, enable bool) context.Context {
	return context.WithValue(parent, remoteBlobAccessCheckEnabledKey, enable)
}

// remoteBlobAccessCheckEnabledFrom reports whether ctx allows
// blobDescriptorService to stat remote blobs.
func remoteBlobAccessCheckEnabledFrom(ctx context.Context) bool {
	enabled, _ := ctx.Value(remoteBlobAccessCheckEnabledKey).(bool)
	return enabled
}

// WithRegistryClient returns a new Context with provided registry client.
func WithRegistryClient(ctx context.Context, client RegistryClient) context.Context {
	return context.WithValue(ctx, registryClientKey, client)
}

// RegistryClientFrom returns the registry client stored in ctx if present.
// It will panic otherwise.
func RegistryClientFrom(ctx context.Context) RegistryClient {
	return ctx.Value(registryClientKey).(RegistryClient)
}

// withUserClient returns a new Context with the origin's client.
// This client should have the current user's credentials
func withUserClient(parent context.Context, userClient client.Interface) context.Context {
	return context.WithValue(parent, userClientKey, userClient)
}

// userClientFrom returns the origin's client stored in ctx, if any.
func userClientFrom(ctx context.Context) (client.Interface, bool) {
	userClient, ok := ctx.Value(userClientKey).(client.Interface)
	return userClient, ok
}

// withAuthPerformed returns a new Context with indication that authentication
// was performed.
func withAuthPerformed(parent context.Context) context.Context {
	return context.WithValue(parent, authPerformedKey, true)
}

// authPerformed reports whether ctx has indication that authentication was
// performed.
func authPerformed(ctx context.Context) bool {
	authPerformed, ok := ctx.Value(authPerformedKey).(bool)
	return ok && authPerformed
}

// withDeferredErrors returns a new Context that carries deferred errors.
func withDeferredErrors(parent context.Context, errs deferredErrors) context.Context {
	return context.WithValue(parent, deferredErrorsKey, errs)
}

// deferredErrorsFrom returns the deferred errors stored in ctx, if any.
func deferredErrorsFrom(ctx context.Context) (deferredErrors, bool) {
	errs, ok := ctx.Value(deferredErrorsKey).(deferredErrors)
	return errs, ok
}
