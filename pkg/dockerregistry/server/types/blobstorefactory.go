package types

import "github.com/docker/distribution"

// BlobStoreFactory creates middleware for BlobStore
type BlobStoreFactory interface {
	BlobStore(bs distribution.BlobStore) distribution.BlobStore
}
