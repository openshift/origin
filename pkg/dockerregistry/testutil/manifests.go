package testutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/libtrust"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ManifestSchemaVesion int
type LayerPayload []byte
type ConfigPayload []byte

type Payload struct {
	Config ConfigPayload
	Layers []LayerPayload
}

const (
	ManifestSchema1 ManifestSchemaVesion = 1
	ManifestSchema2 ManifestSchemaVesion = 2
)

// MakeSchema1Manifest constructs a schema 1 manifest from a given list of digests and returns
// the digest of the manifest
// github.com/docker/distribution/testutil
func MakeSchema1Manifest(layers []distribution.Descriptor) (string, distribution.Manifest, error) {
	manifest := schema1.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
		Name: "who",
		Tag:  "cares",
	}

	for _, layer := range layers {
		manifest.FSLayers = append(manifest.FSLayers, schema1.FSLayer{BlobSum: layer.Digest})
		manifest.History = append(manifest.History, schema1.History{V1Compatibility: "{}"})
	}

	pk, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return "", nil, fmt.Errorf("unexpected error generating private key: %v", err)
	}

	signedManifest, err := schema1.Sign(&manifest, pk)
	if err != nil {
		return "", nil, fmt.Errorf("error signing manifest: %v", err)
	}

	return string(signedManifest.Canonical), signedManifest, nil
}

// MakeSchema2Manifest constructs a schema 2 manifest from a given list of digests and returns
// the digest of the manifest
func MakeSchema2Manifest(config distribution.Descriptor, layers []distribution.Descriptor) (string, distribution.Manifest, error) {
	m := schema2.Manifest{
		Versioned: schema2.SchemaVersion,
		Config:    config,
		Layers:    make([]distribution.Descriptor, len(layers)),
	}
	m.Config.MediaType = schema2.MediaTypeConfig

	for _, layer := range layers {
		layer.MediaType = schema2.MediaTypeLayer
		m.Layers = append(m.Layers, layer)
	}

	manifest, err := schema2.FromStruct(m)
	if err != nil {
		return "", nil, err
	}

	_, payload, err := manifest.Payload()
	if err != nil {
		return "", nil, err
	}

	return string(payload), manifest, nil
}

func MakeRandomLayers(layerCount int) ([]distribution.Descriptor, []LayerPayload, error) {
	var (
		layers   []distribution.Descriptor
		payloads []LayerPayload
	)

	for i := 0; i < layerCount; i++ {
		rs, ds, err := CreateRandomTarFile()
		if err != nil {
			return layers, payloads, fmt.Errorf("unexpected error generating test layer file: %v", err)
		}
		dgst := digest.Digest(ds)

		content, err := ioutil.ReadAll(rs)
		if err != nil {
			return layers, payloads, fmt.Errorf("unexpected error reading layer data: %v", err)
		}

		layers = append(layers, distribution.Descriptor{Digest: dgst, Size: int64(len(content))})
		payloads = append(payloads, LayerPayload(content))
	}

	return layers, payloads, nil
}

func MakeManifestConfig() (ConfigPayload, distribution.Descriptor, error) {
	cfg := imageapi.DockerImageConfig{}
	cfgDesc := distribution.Descriptor{}

	jsonBytes, err := json.Marshal(&cfg)
	if err != nil {
		return nil, cfgDesc, err
	}

	cfgDesc.Digest = digest.FromBytes(jsonBytes)
	cfgDesc.Size = int64(len(jsonBytes))

	return jsonBytes, cfgDesc, nil
}

func CreateRandomManifest(schemaVersion ManifestSchemaVesion, layerCount int) (string, distribution.Manifest, *Payload, error) {
	var (
		rawManifest string
		manifest    distribution.Manifest
		cfgDesc     distribution.Descriptor
		err         error
	)

	layersDescs, layerPayloads, err := MakeRandomLayers(layerCount)
	if err != nil {
		return "", nil, nil, fmt.Errorf("cannot generate layers: %v", err)
	}

	payload := &Payload{
		Layers: layerPayloads,
	}

	switch schemaVersion {
	case ManifestSchema1:
		rawManifest, manifest, err = MakeSchema1Manifest(layersDescs)
	case ManifestSchema2:
		payload.Config, cfgDesc, err = MakeManifestConfig()
		if err != nil {
			return "", nil, nil, err
		}
		rawManifest, manifest, err = MakeSchema2Manifest(cfgDesc, layersDescs)
	default:
		return "", nil, nil, fmt.Errorf("unsupported manifest version %d", schemaVersion)
	}

	return rawManifest, manifest, payload, err
}
