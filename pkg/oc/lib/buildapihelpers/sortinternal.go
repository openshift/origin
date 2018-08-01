package buildapihelpers

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// BuildSliceByCreationTimestampInternal implements sort.Interface for []Build
// based on the CreationTimestamp field.
type BuildSliceByCreationTimestampInternal []buildapi.Build

func (b BuildSliceByCreationTimestampInternal) Len() int {
	return len(b)
}

func (b BuildSliceByCreationTimestampInternal) Less(i, j int) bool {
	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func (b BuildSliceByCreationTimestampInternal) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// BuildPtrSliceByCreationTimestampInternal implements sort.Interface for []*Build
// based on the CreationTimestamp field.
type BuildPtrSliceByCreationTimestampInternal []*buildapi.Build

func (b BuildPtrSliceByCreationTimestampInternal) Len() int {
	return len(b)
}

func (b BuildPtrSliceByCreationTimestampInternal) Less(i, j int) bool {
	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func (b BuildPtrSliceByCreationTimestampInternal) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
