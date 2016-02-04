package server

import (
	"sync"

	"github.com/hashicorp/golang-lru"

	"github.com/docker/distribution/digest"
)

// digestToRepositoryCache maps image digests to recently seen remote repositories that
// may contain that digest. Each digest is bucketed and remembering new repositories will
// push old repositories out.
type digestToRepositoryCache struct {
	*lru.Cache
}

// newDigestToRepositoryCache creates a new LRU cache of image digests to possible remote
// repository strings with the given size. It returns an error if the cache
// cannot be created.
func newDigestToRepositoryCache(size int) (digestToRepositoryCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return digestToRepositoryCache{}, err
	}
	return digestToRepositoryCache{Cache: c}, nil
}

const bucketSize = 10

// RememberDigest associates a digest with a repository.
func (c digestToRepositoryCache) RememberDigest(dgst digest.Digest, repo string) {
	key := dgst.String()
	value, ok := c.Get(key)
	if !ok {
		value = &repositoryBucket{}
		if ok, _ := c.ContainsOrAdd(key, value); !ok {
			return
		}
	}
	repos := value.(*repositoryBucket)
	repos.Add(repo)
}

// RepositoriesForDigest returns a list of repositories that may contain this digest.
func (c digestToRepositoryCache) RepositoriesForDigest(dgst digest.Digest) []string {
	value, ok := c.Get(dgst.String())
	if !ok {
		return nil
	}
	repos := value.(*repositoryBucket)
	return repos.Copy()
}

type repositoryBucket struct {
	mu   sync.Mutex
	list []string
}

// Has returns true if the bucket contains this repository.
func (i *repositoryBucket) Has(repo string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, s := range i.list {
		if s == repo {
			return true
		}
	}
	return false
}

// Add one or more repositories to this bucket.
func (i *repositoryBucket) Add(repos ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	arr := i.list
	for _, repo := range repos {
		if len(arr) >= bucketSize {
			arr = arr[1:]
		}
		arr = append(arr, repo)
	}
	i.list = arr
}

// Copy returns a copy of the contents of this bucket in a threadsafe fasion.
func (i *repositoryBucket) Copy() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]string, len(i.list))
	copy(out, i.list)
	return out
}
