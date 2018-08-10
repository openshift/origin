package image

import (
	"testing"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
)

func TestManifestLayerInfosToBlobInfos(t *testing.T) {
	blobs := manifestLayerInfosToBlobInfos([]manifest.LayerInfo{})
	assert.Equal(t, []types.BlobInfo{}, blobs)

	blobs = manifestLayerInfosToBlobInfos([]manifest.LayerInfo{
		{
			BlobInfo: types.BlobInfo{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
				Size:      32,
			},
			EmptyLayer: true,
		},
		{
			BlobInfo: types.BlobInfo{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909",
				Size:      8841833,
			},
			EmptyLayer: false,
		},
		{
			BlobInfo: types.BlobInfo{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa",
				Size:      291,
			},
			EmptyLayer: false,
		},
		{
			BlobInfo: types.BlobInfo{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
				Size:      32,
			},
			EmptyLayer: true,
		},
	})
	assert.Equal(t, []types.BlobInfo{
		{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Digest:    "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
			Size:      32,
		},
		{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Digest:    "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909",
			Size:      8841833,
		},
		{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Digest:    "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa",
			Size:      291,
		},
		{
			MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Digest:    "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
			Size:      32,
		},
	}, blobs)
}
