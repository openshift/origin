package dockerlayer

// GzippedEmptyLayer is a gzip-compressed version of an empty tar file (1024 NULL bytes)
// This comes from github.com/docker/distribution/manifest/schema1/config_builder.go; there is
// a non-zero embedded timestamp; we could zero that, but that would just waste storage space
// in registries, so let’s use the same values.
var GzippedEmptyLayer = []byte{
	31, 139, 8, 0, 0, 9, 110, 136, 0, 255, 98, 24, 5, 163, 96, 20, 140, 88,
	0, 8, 0, 0, 255, 255, 46, 175, 181, 239, 0, 4, 0, 0,
}

const (
	// GzippedEmptyLayerDigest is a digest of GzippedEmptyLayer
	GzippedEmptyLayerDigest = "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	// EmptyLayerDiffID is the tarsum of the GzippedEmptyLayer
	EmptyLayerDiffID = "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
	// DigestSha256EmptyTar is the canonical sha256 digest of empty data
	DigestSha256EmptyTar = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)
