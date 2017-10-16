package testutil

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/libtrust"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	"github.com/openshift/origin/pkg/image/util"
)

type ManifestSchemaVersion int

type ConfigPayload []byte

const (
	ManifestSchema1 ManifestSchemaVersion = 1
	ManifestSchema2 ManifestSchemaVersion = 2
)

// MakeSchema1Manifest constructs a schema 1 manifest from a given list of digests and returns
// the digest of the manifest
func MakeSchema1Manifest(name, tag string, layers []distribution.Descriptor) (distribution.Manifest, error) {
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
		return nil, fmt.Errorf("unexpected error generating private key: %v", err)
	}

	signedManifest, err := schema1.Sign(&m, pk)
	if err != nil {
		return nil, fmt.Errorf("error signing manifest: %v", err)
	}

	return signedManifest, nil
}

// MakeSchema2Manifest constructs a schema 2 manifest from a given list of digests and returns
// the digest of the manifest
func MakeSchema2Manifest(config distribution.Descriptor, layers []distribution.Descriptor) (distribution.Manifest, error) {
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
		return nil, err
	}

	return manifest, nil
}

// CanonicalManifest returns m in its canonical representation.
func CanonicalManifest(m distribution.Manifest) ([]byte, error) {
	switch m := m.(type) {
	case *schema1.SignedManifest:
		return m.Canonical, nil
	case *schema2.DeserializedManifest:
		_, payload, err := m.Payload()
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	return nil, fmt.Errorf("no canonical representation of %T: %#+v", m, m)
}

func MakeRandomLayer() ([]byte, distribution.Descriptor, error) {
	content, err := CreateRandomTarFile()
	if err != nil {
		return nil, distribution.Descriptor{}, fmt.Errorf("failed to generate data for a random layer: %v", err)
	}

	return content, distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      int64(len(content)),
		Digest:    digest.FromBytes(content),
	}, nil
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

// CreateUploadTestManifest generates a random manifest blob and uploads it to the given repository. For this
// purpose, a given number of layers will be created and uploaded.
func CreateAndUploadTestManifest(
	ctx context.Context,
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
		ds, _, err := UploadRandomTestBlob(ctx, serverURL, creds, repoName)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("unexpected error generating test blob layer: %v", err)
		}
		layerDescriptors = append(layerDescriptors, ds)
	}

	rt, err := NewTransport(serverURL.String(), repoName, creds)
	if err != nil {
		return "", "", "", nil, err
	}

	repo, err := NewRepository(ctx, repoName, serverURL.String(), rt)
	if err != nil {
		return "", "", "", nil, err
	}

	switch schemaVersion {
	case ManifestSchema1:
		manifest, err = MakeSchema1Manifest(repoName, tag, layerDescriptors)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to make manifest of schema 1: %v", err)
		}
	case ManifestSchema2:
		cfgPayload, cfgDesc, err := MakeManifestConfig()
		if err != nil {
			return "", "", "", nil, err
		}
		err = UploadBlob(ctx, repo, cfgDesc, cfgPayload)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to upload manifest config of schema 2: %v", err)
		}
		manifest, err = MakeSchema2Manifest(cfgDesc, layerDescriptors)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed to make manifest schema 2: %v", err)
		}
		manifestConfig = string(cfgPayload)
	default:
		return "", "", "", nil, fmt.Errorf("unsupported manifest version %d", schemaVersion)
	}

	canonicalBytes, err := CanonicalManifest(manifest)
	if err != nil {
		return "", "", "", nil, err
	}

	dgst = digest.FromBytes(canonicalBytes)

	if err := UploadManifest(ctx, repo, tag, manifest); err != nil {
		return "", "", "", nil, err
	}

	return dgst, string(canonicalBytes), manifestConfig, manifest, nil
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
func NewImageForManifest(repoName string, rawManifest string, manifestConfig string, managedByOpenShift bool) (*imageapiv1.Image, error) {
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

	img := imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:        desc.Digest.String(),
			Annotations: annotations,
		},
		DockerImageReference: fmt.Sprintf("localhost:5000/%s@%s", repoName, desc.Digest.String()),
		DockerImageManifest:  rawManifest,
		DockerImageConfig:    manifestConfig,
	}
	if err := util.ImageWithMetadata(&img); err != nil {
		return nil, err
	}
	newImage := imageapiv1.Image{}
	if err := kapi.Scheme.Converter().Convert(&img, &newImage, 0, nil); err != nil {
		return nil, err
	}

	if err := imageapiv1.ImageWithMetadata(&newImage); err != nil {
		return nil, fmt.Errorf("failed to fill image with metadata: %v", err)
	}

	return &newImage, nil
}
