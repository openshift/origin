package restoptions

import (
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
)

// DefaultKeyFunctions sets the default behavior for storage key generation onto a Store.
func DefaultKeyFunctions(store *registry.Store, prefix string, isNamespaced bool) {
	if isNamespaced {
		if store.KeyRootFunc == nil {
			store.KeyRootFunc = func(ctx apirequest.Context) string {
				return registry.NamespaceKeyRootFunc(ctx, prefix)
			}
		}
		if store.KeyFunc == nil {
			store.KeyFunc = func(ctx apirequest.Context, name string) (string, error) {
				return registry.NamespaceKeyFunc(ctx, prefix, name)
			}
		}
	} else {
		if store.KeyRootFunc == nil {
			store.KeyRootFunc = func(ctx apirequest.Context) string {
				return prefix
			}
		}
		if store.KeyFunc == nil {
			store.KeyFunc = func(ctx apirequest.Context, name string) (string, error) {
				return registry.NoNamespaceKeyFunc(ctx, prefix, name)
			}
		}
	}
}
