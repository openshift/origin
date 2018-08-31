package add

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	digest "github.com/opencontainers/go-digest"

	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	"github.com/openshift/origin/pkg/image/dockerlayer"
)

// get base manifest
// check that I can access base layers
// find the input file (assume I can stream)
// start a streaming upload of the layer to the remote registry, while calculating digests
// get back the final digest
// build the new image manifest and config.json
// upload config.json
// upload the rest of the layers
// tag the image

const (
	// dockerV2Schema2LayerMediaType is the MIME type used for schema 2 layers.
	dockerV2Schema2LayerMediaType = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	// dockerV2Schema2ConfigMediaType is the MIME type used for schema 2 config blobs.
	dockerV2Schema2ConfigMediaType = "application/vnd.docker.container.image.v1+json"
)

// DigestCopy reads all of src into dst, where src is a gzipped stream. It will return the
// sha256 sum of the underlying content (the layerDigest) and the sha256 sum of the
// tar archive (the blobDigest) or an error. If the gzip layer has a modification time
// it will be returned.
// TODO: use configurable digests
func DigestCopy(dst io.ReaderFrom, src io.Reader) (layerDigest, blobDigest digest.Digest, modTime *time.Time, size int64, err error) {
	algo := digest.Canonical
	// calculate the blob digest as the sha256 sum of the uploaded contents
	blobhash := algo.Hash()
	// calculate the diffID as the sha256 sum of the layer contents
	pr, pw := io.Pipe()
	layerhash := algo.Hash()
	ch := make(chan error)
	go func() {
		defer close(ch)
		gr, err := gzip.NewReader(pr)
		if err != nil {
			ch <- fmt.Errorf("unable to create gzip reader layer upload: %v", err)
			return
		}
		if !gr.Header.ModTime.IsZero() {
			modTime = &gr.Header.ModTime
		}
		_, err = io.Copy(layerhash, gr)
		if err != nil {
			io.Copy(ioutil.Discard, pr)
		}
		ch <- err
	}()

	n, err := dst.ReadFrom(io.TeeReader(src, io.MultiWriter(blobhash, pw)))
	if err != nil {
		return "", "", nil, 0, fmt.Errorf("unable to upload new layer (%d): %v", n, err)
	}
	if err := pw.Close(); err != nil {
		return "", "", nil, 0, fmt.Errorf("unable to complete writing diffID: %v", err)
	}
	if err := <-ch; err != nil {
		return "", "", nil, 0, fmt.Errorf("unable to calculate layer diffID: %v", err)
	}

	layerDigest = digest.NewDigestFromBytes(algo, layerhash.Sum(make([]byte, 0, layerhash.Size())))
	blobDigest = digest.NewDigestFromBytes(algo, blobhash.Sum(make([]byte, 0, blobhash.Size())))
	return layerDigest, blobDigest, modTime, n, nil
}

func NewEmptyConfig() *docker10.DockerImageConfig {
	config := &docker10.DockerImageConfig{
		DockerVersion: "",
		// Created must be non-zero
		Created:      (time.Time{}).Add(1 * time.Second),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
	return config
}

func AddScratchLayerToConfig(config *docker10.DockerImageConfig) distribution.Descriptor {
	layer := distribution.Descriptor{
		MediaType: dockerV2Schema2LayerMediaType,
		Digest:    digest.Digest(dockerlayer.GzippedEmptyLayerDigest),
		Size:      int64(len(dockerlayer.GzippedEmptyLayer)),
	}
	AddLayerToConfig(config, layer, dockerlayer.EmptyLayerDiffID)
	return layer
}

func AddLayerToConfig(config *docker10.DockerImageConfig, layer distribution.Descriptor, diffID string) {
	if config.RootFS == nil {
		config.RootFS = &docker10.DockerConfigRootFS{Type: "layers"}
	}
	config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, diffID)
	config.Size += layer.Size
}

func UploadSchema2Config(ctx context.Context, blobs distribution.BlobService, config *docker10.DockerImageConfig, layers []distribution.Descriptor) (*schema2.DeserializedManifest, error) {
	// ensure the image size is correct before persisting
	config.Size = 0
	for _, layer := range layers {
		config.Size += layer.Size
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return putSchema2ImageConfig(ctx, blobs, dockerV2Schema2ConfigMediaType, configJSON, layers)
}

// putSchema2ImageConfig uploads the provided configJSON to the blob store and returns the generated manifest
// for the requested image.
func putSchema2ImageConfig(ctx context.Context, blobs distribution.BlobService, mediaType string, configJSON []byte, layers []distribution.Descriptor) (*schema2.DeserializedManifest, error) {
	b := schema2.NewManifestBuilder(blobs, mediaType, configJSON)
	for _, layer := range layers {
		if err := b.AppendReference(layer); err != nil {
			return nil, err
		}
	}
	m, err := b.Build(ctx)
	if err != nil {
		return nil, err
	}
	manifest, ok := m.(*schema2.DeserializedManifest)
	if !ok {
		return nil, fmt.Errorf("unable to turn %T into a DeserializedManifest, unable to store image", m)
	}
	return manifest, nil
}
