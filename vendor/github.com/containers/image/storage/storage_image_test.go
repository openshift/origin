package storage

import (
	"testing"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLayerInfosForCopy(t *testing.T) {
	manifestInfos := []manifest.LayerInfo{
		{BlobInfo: types.BlobInfo{Digest: "sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5bb", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680c", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:5555555555555555555555555555555555555555555555555555555555555555", Size: -1}, EmptyLayer: true},
	}
	physicalInfos := []types.BlobInfo{
		{Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111", Size: 111, MediaType: manifest.DockerV2Schema2LayerMediaType},
		{Digest: "sha256:2222222222222222222222222222222222222222222222222222222222222222", Size: 222, MediaType: manifest.DockerV2Schema2LayerMediaType},
	}

	// Success
	res, err := buildLayerInfosForCopy(manifestInfos, physicalInfos)
	require.NoError(t, err)
	assert.Equal(t, []types.BlobInfo{
		{Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111", Size: 111, MediaType: manifest.DockerV2Schema2LayerMediaType},
		{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: 32},
		{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: 32},
		{Digest: "sha256:2222222222222222222222222222222222222222222222222222222222222222", Size: 222, MediaType: manifest.DockerV2Schema2LayerMediaType},
		{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: 32},
	}, res)

	// PhysicalInfos too short
	_, err = buildLayerInfosForCopy(manifestInfos, physicalInfos[:len(physicalInfos)-1])
	assert.Error(t, err)

	// PhysicalInfos too long
	_, err = buildLayerInfosForCopy(manifestInfos, append(physicalInfos, physicalInfos[0]))
	assert.Error(t, err)
}
