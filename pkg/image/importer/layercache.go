package importer

import (
	"github.com/hashicorp/golang-lru"
)

const (
	DefaultImageStreamLayerCacheSize = 2048
)

type ImageStreamLayerCache struct {
	*lru.Cache
}

// ImageStreamLayerCache creates a new LRU cache of layer digests
func NewImageStreamLayerCache(size int) (ImageStreamLayerCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return ImageStreamLayerCache{}, err
	}
	return ImageStreamLayerCache{
		Cache: c,
	}, nil
}
