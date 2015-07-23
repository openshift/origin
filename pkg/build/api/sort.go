package api

// ByCreationTimestamp implements sort.Interface for []Build based on the
// CreationTimestamp field.
type ByCreationTimestamp []Build

func (b ByCreationTimestamp) Len() int {
	return len(b)
}

func (b ByCreationTimestamp) Less(i, j int) bool {
	return b[i].CreationTimestamp.Before(b[j].CreationTimestamp)
}

func (b ByCreationTimestamp) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
