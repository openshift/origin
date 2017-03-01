package testutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	distclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/libtrust"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/diff"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ManifestSchemaVersion int
type LayerPayload []byte
type ConfigPayload []byte

type Payload struct {
	Config ConfigPayload
	Layers []LayerPayload
}

const (
	ManifestSchema1 ManifestSchemaVersion = 1
	ManifestSchema2 ManifestSchemaVersion = 2
)

// MakeSchema1Manifest constructs a schema 1 manifest from a given list of digests and returns
// the digest of the manifest
// github.com/docker/distribution/testutil
func MakeSchema1Manifest(name, tag string, layers []distribution.Descriptor) (string, distribution.Manifest, error) {
	m := schema1.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
		FSLayers: make([]schema1.FSLayer, 0, len(layers)),
		History:  make([]schema1.History, 0, len(layers)),
		Name:     name,
		Tag:      tag,
	}

	for _, layer := range layers {
		m.FSLayers = append(m.FSLayers, schema1.FSLayer{BlobSum: layer.Digest})
		m.History = append(m.History, schema1.History{V1Compatibility: "{}"})
	}

	pk, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return "", nil, fmt.Errorf("unexpected error generating private key: %v", err)
	}

	signedManifest, err := schema1.Sign(&m, pk)
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
		Layers:    make([]distribution.Descriptor, 0, len(layers)),
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

func CreateRandomManifest(schemaVersion ManifestSchemaVersion, layerCount int) (string, distribution.Manifest, *Payload, error) {
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
		rawManifest, manifest, err = MakeSchema1Manifest("who", "cares", layersDescs)
	case ManifestSchema2:
		_, cfgDesc, err = MakeManifestConfig()
		if err != nil {
			return "", nil, nil, err
		}
		rawManifest, manifest, err = MakeSchema2Manifest(cfgDesc, layersDescs)
	default:
		return "", nil, nil, fmt.Errorf("unsupported manifest version %d", schemaVersion)
	}

	return rawManifest, manifest, payload, err
}

// CreateUploadTestManifest generates a random manifest blob and uploads it to the given repository. For this
// purpose, a given number of layers will be created and uploaded.
func CreateAndUploadTestManifest(
	schemaVersion ManifestSchemaVersion,
	layerCount int,
	serverURL *url.URL,
	creds auth.CredentialStore,
	repoName, tag string,
) (dgst digest.Digest, canonical, manifestConfig string, manifest distribution.Manifest, err error) {
	var (
		layerDescriptors = make([]distribution.Descriptor, 0, layerCount)
	)

	for i := 0; i < layerCount; i++ {
		ds, _, err := UploadRandomTestBlob(serverURL, creds, repoName)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("unexpected error generating test blob layer: %v", err)
		}
		layerDescriptors = append(layerDescriptors, ds)
	}

	switch schemaVersion {
	case ManifestSchema1:
		canonical, manifest, err = MakeSchema1Manifest(repoName, tag, layerDescriptors)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to make manifest of schema 1: %v", err)
		}
	case ManifestSchema2:
		cfgPayload, cfgDesc, err := MakeManifestConfig()
		if err != nil {
			return "", "", "", nil, err
		}
		_, err = UploadPayloadAsBlob(cfgPayload, serverURL, creds, repoName)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to upload manifest config of schema 2: %v", err)
		}
		canonical, manifest, err = MakeSchema2Manifest(cfgDesc, layerDescriptors)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to make manifest schema 2: %v", err)
		}
		manifestConfig = string(cfgPayload)
	default:
		return "", "", "", nil, fmt.Errorf("unsupported manifest version %d", schemaVersion)
	}

	expectedDgst := digest.FromBytes([]byte(canonical))

	ctx := context.Background()
	ref, err := reference.ParseNamed(repoName)
	if err != nil {
		return "", "", "", nil, err
	}

	var rt http.RoundTripper
	if creds != nil {
		challengeManager := auth.NewSimpleChallengeManager()
		_, err := ping(challengeManager, serverURL.String()+"/v2/", "")
		if err != nil {
			return "", "", "", nil, err
		}
		rt = transport.NewTransport(
			nil,
			auth.NewAuthorizer(
				challengeManager,
				auth.NewTokenHandler(nil, creds, repoName, "pull", "push"),
				auth.NewBasicHandler(creds)))
	}
	repo, err := distclient.NewRepository(ctx, ref, serverURL.String(), rt)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to get repository %q: %v", repoName, err)
	}

	ms, err := repo.Manifests(ctx)
	if err != nil {
		return "", "", "", nil, err
	}
	dgst, err = ms.Put(ctx, manifest)
	if err != nil {
		return "", "", "", nil, err
	}

	if expectedDgst != dgst {
		return "", "", "", nil, fmt.Errorf("registry server computed different digest for uploaded manifest than expected: %q != %q", dgst, expectedDgst)
	}

	return dgst, canonical, manifestConfig, manifest, nil
}

// AssertManifestsEqual compares two manifests and returns if they are equal. Signatures of manifest schema 1
// are not taken into account.
func AssertManifestsEqual(t *testing.T, description string, ma distribution.Manifest, mb distribution.Manifest) {
	if ma == mb {
		return
	}

	if (ma == nil) != (mb == nil) {
		t.Fatalf("[%s] only one of the manifests is nil", description)
	}

	_, pa, err := ma.Payload()
	if err != nil {
		t.Fatalf("[%s] failed to get payload for first manifest: %v", description, err)
	}
	_, pb, err := mb.Payload()
	if err != nil {
		t.Fatalf("[%s] failed to get payload for second manifest: %v", description, err)
	}

	var va, vb manifest.Versioned
	if err := json.Unmarshal([]byte(pa), &va); err != nil {
		t.Fatalf("[%s] failed to unmarshal payload of the first manifest: %v", description, err)
	}
	if err := json.Unmarshal([]byte(pb), &vb); err != nil {
		t.Fatalf("[%s] failed to unmarshal payload of the second manifest: %v", description, err)
	}

	if !reflect.DeepEqual(va, vb) {
		t.Fatalf("[%s] manifests are of different version: %s", description, diff.ObjectGoPrintDiff(va, vb))
	}

	switch va.SchemaVersion {
	case 1:
		ms1a, ok := ma.(*schema1.SignedManifest)
		if !ok {
			t.Fatalf("[%s] failed to convert first manifest (%T) to schema1.SignedManifest", description, ma)
		}
		ms1b, ok := mb.(*schema1.SignedManifest)
		if !ok {
			t.Fatalf("[%s] failed to convert first manifest (%T) to schema1.SignedManifest", description, mb)
		}
		if !reflect.DeepEqual(ms1a.Manifest, ms1b.Manifest) {
			t.Fatalf("[%s] manifests don't match: %s", description, diff.ObjectGoPrintDiff(ms1a.Manifest, ms1b.Manifest))
		}

	case 2:
		if !reflect.DeepEqual(ma, mb) {
			t.Fatalf("[%s] manifests don't match: %s", description, diff.ObjectGoPrintDiff(ma, mb))
		}

	default:
		t.Fatalf("[%s] unrecognized manifest schema version: %d", description, va.SchemaVersion)
	}
}

// NewImageManifest creates a new Image object for the given manifest string. Note that the manifest must
// contain signatures if it is of schema 1.
func NewImageForManifest(repoName string, rawManifest string, manifestConfig string, managedByOpenShift bool) (*imageapi.Image, error) {
	var versioned manifest.Versioned
	if err := json.Unmarshal([]byte(rawManifest), &versioned); err != nil {
		return nil, err
	}

	_, desc, err := distribution.UnmarshalManifest(versioned.MediaType, []byte(rawManifest))
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)
	if managedByOpenShift {
		annotations[imageapi.ManagedByOpenShiftAnnotation] = "true"
	}

	img := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:        desc.Digest.String(),
			Annotations: annotations,
		},
		DockerImageReference: fmt.Sprintf("localhost:5000/%s@%s", repoName, desc.Digest.String()),
		DockerImageManifest:  rawManifest,
		DockerImageConfig:    manifestConfig,
	}

	if err := imageapi.ImageWithMetadata(img); err != nil {
		return nil, fmt.Errorf("failed to fill image with metadata: %v", err)
	}

	return img, nil
}
