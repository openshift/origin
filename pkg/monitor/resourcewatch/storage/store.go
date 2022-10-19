package storage

import "k8s.io/client-go/tools/cache"

type ResourceWatchStore interface {
	cache.ResourceEventHandler
	End()
}
