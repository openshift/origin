package sandbox

// StoreFilter defines a function to filter
// sandboxes in the store.
type StoreFilter func(*Sandbox) bool

// StoreReducer defines a function to
// manipulate sandboxes in the store
type StoreReducer func(*Sandbox)

// Storer defines an interface that any container store must implement.
type Storer interface {
	// Add appends a new sandbox to the store.
	Add(string, *Sandbox)
	// Get returns a sandbox from the store by the identifier it was stored with.
	Get(string) *Sandbox
	// Delete removes a sandbox from the store by the identifier it was stored with.
	Delete(string)
	// List returns a list of sandboxes from the store.
	List() []*Sandbox
	// Size returns the number of sandboxes in the store.
	Size() int
	// First returns the first sandbox found in the store by a given filter.
	First(StoreFilter) *Sandbox
	// ApplyAll calls the reducer function with every sandbox in the store.
	ApplyAll(StoreReducer)
}
