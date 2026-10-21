// Package cache provides a generic thread-safe cache with hit/miss tracking.
package cache

import (
	"sync"
	"sync/atomic"
)

type Cache[K comparable, V any] struct {
	entries map[K]V
	// pending holds the keys a caller is computing. A second caller for one of
	// them waits on it rather than computing again, and callers for other keys
	// are not held up: the lock covers the maps, never the work.
	pending map[K]*computation[V]
	mu      sync.RWMutex  //exhaustruct:optional
	hits    atomic.Uint64 //exhaustruct:optional
	misses  atomic.Uint64 //exhaustruct:optional
}

// computation is one in-flight fill. The caller that opened it writes value,
// marks it filled and closes done; every other caller reads both after done is
// closed. A fill that panics closes done unfilled, and its waiters compute for
// themselves.
type computation[V any] struct {
	done   chan struct{}
	value  V    //exhaustruct:optional
	filled bool //exhaustruct:optional
}

func New[K comparable, V any](initialCapacity int) *Cache[K, V] {
	return &Cache[K, V]{
		entries: make(map[K]V, initialCapacity),
		pending: make(map[K]*computation[V]),
	}
}

func (c *Cache[K, V]) Get(key K) (v V, ok bool) {
	c.mu.RLock()

	v, ok = c.entries[key]

	c.mu.RUnlock()

	if ok {
		c.hits.Add(1)
	}

	return v, ok
}

// Peek returns the value without updating hit/miss counters. Use it when the
// caller has already recorded the miss that triggered the fill and a follow-up
// read for the just-written entry must not inflate the hit rate.
func (c *Cache[K, V]) Peek(key K) (v V, ok bool) {
	c.mu.RLock()

	v, ok = c.entries[key]

	c.mu.RUnlock()

	return v, ok
}

// Set stores value and increments miss counter (caller computed the value).
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()

	c.entries[key] = value
	c.misses.Add(1)

	c.mu.Unlock()
}

// GetOrSet returns the value stored under key, computing it once when it is
// absent. Callers racing for one key all receive the value that computation
// published: what the first of them computed, or what a Set that landed while
// it computed stored, since that Set is the later of the two writes. A Set
// after the publication is later still and leaves what was returned as stale
// as any Get result. Callers for other keys run alongside them, since compute
// is never called with the cache locked.
//
// A compute that panics releases the key: the panic reaches its own caller, and
// every caller waiting on that fill computes for itself. Nothing is stored for
// the key until one of them returns.
func (c *Cache[K, V]) GetOrSet(key K, compute func() V) V {
	for {
		// A filled key is the common answer and needs no more than the read lock
		// a Get takes, which keys with nothing in common do not hold each other
		// out of. The exclusive lock is for opening or joining a computation.
		if v, ok := c.Get(key); ok {
			return v
		}

		c.mu.Lock()

		if v, ok := c.entries[key]; ok {
			c.mu.Unlock()
			c.hits.Add(1)

			return v
		}

		if pending, ok := c.pending[key]; ok {
			c.mu.Unlock()

			<-pending.done

			if pending.filled {
				c.hits.Add(1)

				return pending.value
			}

			continue
		}

		pending := &computation[V]{done: make(chan struct{})}

		c.pending[key] = pending
		c.misses.Add(1)

		c.mu.Unlock()

		return c.fill(key, pending, compute)
	}
}

// fill computes the value claimed under key and publishes it. The claim is
// released and done is closed whether compute returns or panics, so a fill that
// panics leaves no waiter parked and no key claimed for good.
func (c *Cache[K, V]) fill(key K, pending *computation[V], compute func() V) (v V) {
	defer func() {
		c.mu.Lock()

		// A Set may have landed while the value was being computed. It is the
		// later write of the two, and publishing over it would leave the key
		// holding what neither order of the two produces.
		if pending.filled {
			if stored, ok := c.entries[key]; ok {
				pending.value = stored
			} else {
				c.entries[key] = pending.value
			}
		}

		delete(c.pending, key)

		c.mu.Unlock()

		close(pending.done)

		v = pending.value
	}()

	pending.value = compute()
	pending.filled = true

	return pending.value
}

func (c *Cache[K, V]) Stats() (hits, misses, size uint64) {
	c.mu.RLock()

	size = uint64(len(c.entries))

	c.mu.RUnlock()

	return c.hits.Load(), c.misses.Load(), size
}
