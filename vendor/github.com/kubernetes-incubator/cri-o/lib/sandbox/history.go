package sandbox

import "sort"

// History is a convenience type for storing a list of sandboxes,
// sorted by creation date in descendant order.
type History []*Sandbox

// Len returns the number of sandboxes in the history.
func (history *History) Len() int {
	return len(*history)
}

// Less compares two sandboxes and returns true if the second one
// was created before the first one.
func (history *History) Less(i, j int) bool {
	sandboxes := *history
	// FIXME: state access should be serialized
	return sandboxes[j].created.Before(sandboxes[i].created)
}

// Swap switches sandboxes i and j positions in the history.
func (history *History) Swap(i, j int) {
	sandboxes := *history
	sandboxes[i], sandboxes[j] = sandboxes[j], sandboxes[i]
}

// sort orders the history by creation date in descendant order.
func (history *History) sort() {
	sort.Sort(history)
}
