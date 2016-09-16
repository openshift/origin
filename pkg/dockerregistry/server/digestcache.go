package server

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"

	"github.com/docker/distribution/digest"

	"k8s.io/kubernetes/pkg/util/clock"
)

// digestToRepositoryCache maps image digests to recently seen remote repositories that
// may contain that digest. Each digest is bucketed and remembering new repositories will
// push old repositories out.
type digestToRepositoryCache struct {
	*lru.Cache
	clock clock.Clock
}

// newDigestToRepositoryCache creates a new LRU cache of image digests to possible remote
// repository strings with the given size. It returns an error if the cache
// cannot be created.
func newDigestToRepositoryCache(size int) (digestToRepositoryCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return digestToRepositoryCache{}, err
	}
	return digestToRepositoryCache{
		Cache: c,
		clock: &clock.RealClock{},
	}, nil
}

const bucketSize = 16

// RememberDigest associates a digest with a repository.
func (c digestToRepositoryCache) RememberDigest(dgst digest.Digest, ttl time.Duration, repo string) {
	key := dgst.String()
	value, ok := c.Get(key)
	if !ok {
		value = &repositoryBucket{clock: c.clock}
		if ok, _ := c.ContainsOrAdd(key, value); ok {
			// the value exists now, get it
			value, ok = c.Get(key)
			if !ok {
				// should not happen
				return
			}
		}
	}
	repos := value.(*repositoryBucket)
	repos.Add(ttl, repo)
}

// ForgetDigest removes an association between given digest and repository from the cache.
func (c digestToRepositoryCache) ForgetDigest(dgst digest.Digest, repo string) {
	key := dgst.String()
	value, ok := c.Peek(key)
	if !ok {
		return
	}
	repos := value.(*repositoryBucket)
	repos.Remove(repo)
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

func (c digestToRepositoryCache) RepositoryHasBlob(repo string, dgst digest.Digest) bool {
	value, ok := c.Get(dgst.String())
	if !ok {
		return false
	}
	repos := value.(*repositoryBucket)
	return repos.Has(repo)
}

// repositoryBucket contains a list of repositories with eviction timeouts.
type repositoryBucket struct {
	mu    sync.Mutex
	clock clock.Clock
	list  []bucketEntry
}

// Has returns true if the bucket contains this repository.
func (b *repositoryBucket) Has(repo string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.evictStale()

	return b.getIndexOf(repo) >= 0
}

// Add one or more repositories to this bucket.
func (b *repositoryBucket) Add(ttl time.Duration, repos ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.evictStale()
	evictOn := b.clock.Now().Add(ttl)

	for _, repo := range repos {
		index := b.getIndexOf(repo)
		arr := b.list

		if index >= 0 {
			// repository already exists, move it to the end with highest eviction time
			entry := arr[index]
			copy(arr[index:], arr[index+1:])
			if entry.evictOn.Before(evictOn) {
				entry.evictOn = evictOn
			}
			arr[len(arr)-1] = entry

		} else {
			// repo is a new entry
			if len(arr) >= bucketSize {
				copy(arr, arr[1:])
				arr = arr[:bucketSize-1]
			}
			arr = append(arr, bucketEntry{
				repository: repo,
				evictOn:    evictOn,
			})
		}
		b.list = arr
	}
}

// Remove removes all the given repos from repository bucket.
func (b *repositoryBucket) Remove(repos ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, repo := range repos {
		index := b.getIndexOf(repo)

		if index >= 0 {
			copy(b.list[index:], b.list[index+1:])
			b.list = b.list[:len(b.list)-1]
		}
	}
}

// getIndexOf returns an index of given repository in bucket's array. If not found, -1 will be returned.
func (b *repositoryBucket) getIndexOf(repo string) int {
	for i, entry := range b.list {
		if entry.repository == repo {
			return i
		}
	}

	return -1
}

// evictStale removes stale entries from the list and shifts all the survivalists to the front.
func (b *repositoryBucket) evictStale() {
	now := b.clock.Now()
	arr := b.list[:0]

	for _, entry := range b.list {
		if entry.evictOn.Before(now) {
			continue
		}
		arr = append(arr, entry)
	}

	b.list = arr
}

// Copy returns a copy of the contents of this bucket in a thread-safe fashion.
func (b *repositoryBucket) Copy() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.evictStale()

	out := make([]string, len(b.list))
	for i, e := range b.list {
		out[i] = e.repository
	}
	return out
}

// bucketEntry holds a repository name with eviction timeout.
type bucketEntry struct {
	repository string
	evictOn    time.Time
}
