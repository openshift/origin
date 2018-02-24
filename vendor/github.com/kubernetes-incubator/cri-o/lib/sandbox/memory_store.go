package sandbox

import "sync"

// memoryStore implements a Store in memory.
type memoryStore struct {
	s map[string]*Sandbox
	sync.RWMutex
}

// NewMemoryStore initializes a new memory store.
func NewMemoryStore() Storer {
	return &memoryStore{
		s: make(map[string]*Sandbox),
	}
}

// Add appends a new sandbox to the memory store.
// It overrides the id if it existed before.
func (c *memoryStore) Add(id string, cont *Sandbox) {
	c.Lock()
	c.s[id] = cont
	c.Unlock()
}

// Get returns a sandbox from the store by id.
func (c *memoryStore) Get(id string) *Sandbox {
	var res *Sandbox
	c.RLock()
	res = c.s[id]
	c.RUnlock()
	return res
}

// Delete removes a sandbox from the store by id.
func (c *memoryStore) Delete(id string) {
	c.Lock()
	delete(c.s, id)
	c.Unlock()
}

// List returns a sorted list of sandboxes from the store.
// The sandboxes are ordered by creation date.
func (c *memoryStore) List() []*Sandbox {
	sandboxes := History(c.all())
	sandboxes.sort()
	return sandboxes
}

// Size returns the number of sandboxes in the store.
func (c *memoryStore) Size() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.s)
}

// First returns the first sandbox found in the store by a given filter.
func (c *memoryStore) First(filter StoreFilter) *Sandbox {
	for _, cont := range c.all() {
		if filter(cont) {
			return cont
		}
	}
	return nil
}

// ApplyAll calls the reducer function with every sandbox in the store.
// This operation is asynchronous in the memory store.
// NOTE: Modifications to the store MUST NOT be done by the StoreReducer.
func (c *memoryStore) ApplyAll(apply StoreReducer) {
	wg := new(sync.WaitGroup)
	for _, cont := range c.all() {
		wg.Add(1)
		go func(sandbox *Sandbox) {
			apply(sandbox)
			wg.Done()
		}(cont)
	}

	wg.Wait()
}

func (c *memoryStore) all() []*Sandbox {
	c.RLock()
	sandboxes := make([]*Sandbox, 0, len(c.s))
	for _, cont := range c.s {
		sandboxes = append(sandboxes, cont)
	}
	c.RUnlock()
	return sandboxes
}

var _ Storer = &memoryStore{}
