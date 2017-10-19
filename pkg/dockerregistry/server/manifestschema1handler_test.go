package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/libtrust"

	registryclient "github.com/openshift/origin/pkg/dockerregistry/server/client"
	"github.com/openshift/origin/pkg/dockerregistry/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

func TestUnmarshalManifestSchema1(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		manifestString         string
		signatures             [][]byte
		expectedName           string
		expectedTag            string
		expectedReferences     []distribution.Descriptor
		expectedSignatures     [][]byte
		expectedErrorSubstring string
	}{
		{
			name:           "valid manifest with sizes",
			manifestString: manifestSchema1,
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[1])},
			},
			expectedSignatures: [][]byte{[]byte(manifestSchema1Signature)},
		},

		{
			name:           "valid manifest with missing sizes",
			manifestString: manifestSchema1WithoutSize,
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[1])},
			},
			expectedSignatures: [][]byte{[]byte(manifestSchema1WithoutSizeSignature)},
		},

		{
			name:           "having shorter history",
			manifestString: manifestSchema1ShortHistory,
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[1])},
			},
			expectedSignatures: [][]byte{[]byte(manifestSchema1ShortHistorySignature)},
		},

		{
			name:           "having shorter fs layers",
			manifestString: manifestSchema1ShortFSLayers,
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
			},
			expectedSignatures: [][]byte{[]byte(manifestSchema1ShortFSLayersSignature)},
		},

		{
			name:           "additional signatures",
			manifestString: manifestSchema1,
			signatures:     [][]byte{[]byte("my signature")},
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[1])},
			},
			// the additional signature is ignored
			expectedSignatures: [][]byte{[]byte(manifestSchema1Signature)},
		},

		{
			name:                   "manifest missing signatures",
			manifestString:         manifestSchema1MissingSignatures,
			expectedErrorSubstring: "no signatures",
		},

		{
			name:           "just external signatures",
			manifestString: manifestSchema1MissingSignatures,
			signatures:     manifestSchema1ExternalSignatures,
			expectedName:   "library/busybox",
			expectedTag:    "1.23",
			expectedReferences: []distribution.Descriptor{
				{MediaType: schema1.MediaTypeManifestLayer, Digest: digest.Digest(manifestSchema1Layers[0])},
			},
			expectedSignatures: manifestSchema1ExternalSignaturesCompact,
		},

		{
			name:                   "invalid manifest",
			manifestString:         manifestSchema1Invalid,
			expectedErrorSubstring: "invalid character",
		},

		{
			name:           "manifest schema 2",
			manifestString: manifestSchema2,
			// FIXME: this could report some better error
			expectedErrorSubstring: "no signatures",
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			manifest, err := unmarshalManifestSchema1([]byte(tc.manifestString), tc.signatures)
			if err != nil {
				if len(tc.expectedErrorSubstring) == 0 {
					t.Fatalf("got unexpected error: (%T) %v", err, err)
				}
				if !strings.Contains(err.Error(), tc.expectedErrorSubstring) {
					t.Fatalf("expected error with string %q, instead got: %v", tc.expectedErrorSubstring, err)
				}
				return
			}
			if err == nil && len(tc.expectedErrorSubstring) > 0 {
				t.Fatalf("got non-error while expecting: %s", tc.expectedErrorSubstring)
			}

			sm, ok := manifest.(*schema1.SignedManifest)
			if !ok {
				t.Fatalf("got unexpected manifest schema: %T", sm)
			}

			if sm.Name != tc.expectedName {
				t.Errorf("got unexpected image name: %s", diff.ObjectGoPrintDiff(sm.Name, tc.expectedName))
			}
			if sm.Tag != tc.expectedTag {
				t.Errorf("got unexpected image tag: %s", diff.ObjectGoPrintDiff(sm.Tag, tc.expectedTag))
			}

			refs := manifest.References()
			if !reflect.DeepEqual(refs, tc.expectedReferences) {
				t.Errorf("got unexpected image references: %s", diff.ObjectGoPrintDiff(refs, tc.expectedReferences))
			}

			signatures, err := sm.Signatures()
			if err != nil {
				t.Fatalf("failed to get manifest signatures: %v", err)
			}
			if !reflect.DeepEqual(signatures, tc.expectedSignatures) {
				t.Errorf("got unexpected image signatures: %s", diff.ObjectGoPrintDiff(signatures, tc.expectedSignatures))
				for i, sig := range signatures {
					t.Logf("signature #%d: %#v", i, string(sig))

				}
				for i, sig := range tc.expectedSignatures {
					t.Logf("expected signature #%d: %#v", i, string(sig))
				}
			}
		})
	}
}

func TestManifestSchema1Handler_Layers(t *testing.T) {
	for _, tc := range []struct {
		name            string
		manifestString  string
		blobDescriptors blobDescriptors
		expectedLayers  []imageapiv1.ImageLayer
		expectedError   error
	}{
		{
			name:           "valid manifest with sizes",
			manifestString: manifestSchema1,
			expectedLayers: []imageapiv1.ImageLayer{
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      manifestSchema1Layers[1],
					LayerSize: 1095501,
				},
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      digestSHA256GzippedEmptyTar.String(),
					LayerSize: 0,
				},
			},
		},

		{
			name:           "valid manifest with missing sizes",
			manifestString: manifestSchema1WithoutSize,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   100,
				},
			},
			expectedLayers: []imageapiv1.ImageLayer{
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      manifestSchema1Layers[1],
					LayerSize: 100,
				},
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      digestSHA256GzippedEmptyTar.String(),
					LayerSize: 0,
				},
			},
		},

		{
			name:           "manifest sizes take precedence",
			manifestString: manifestSchema1,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   5,
				},
			},
			expectedLayers: []imageapiv1.ImageLayer{
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      manifestSchema1Layers[1],
					LayerSize: 1095501,
				},
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      digestSHA256GzippedEmptyTar.String(),
					LayerSize: 0,
				},
			},
		},

		{
			name:           "shorter history",
			manifestString: manifestSchema1ShortHistory,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   100,
				},
			},
			expectedLayers: []imageapiv1.ImageLayer{
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      manifestSchema1Layers[1],
					LayerSize: 100,
				},
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      digestSHA256GzippedEmptyTar.String(),
					LayerSize: 0,
				},
			},
		},

		{
			name:           "shorter fs layers",
			manifestString: manifestSchema1ShortFSLayers,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[0]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[0]),
					Size:   100,
				},
			},
			expectedLayers: []imageapiv1.ImageLayer{
				{
					MediaType: schema1.MediaTypeManifestLayer,
					Name:      digestSHA256GzippedEmptyTar.String(),
					LayerSize: 5,
				},
			},
		},

		{
			name:           "manifest with no layers",
			manifestString: manifestSchema1NoLayers,
			expectedLayers: []imageapiv1.ImageLayer{},
		},

		{
			name:           "blob unknown",
			manifestString: manifestSchema1WithoutSize,
			expectedError:  distribution.ErrBlobUnknown,
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			manifest := tryUnmarshalManifestSchema1OrGenerateSignatures(t, tc.manifestString)

			bds := blobDescriptors{
				digestSHA256GzippedEmptyTar: distribution.Descriptor{
					Digest: digestSHA256GzippedEmptyTar,
					Size:   0,
				},
			}
			for d, desc := range tc.blobDescriptors {
				bds[d] = desc
			}

			bs := newTestBlobStore(bds, nil)
			_, imageClient := testutil.NewFakeOpenShiftWithClient()
			repo := newTestRepository(t, "nm", "repo", testRepositoryOptions{
				client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
				blobs:  bs,
			})
			h, err := NewManifestHandler(repo, manifest)
			if err != nil {
				t.Fatal(err)
			}

			ctx := withAuthPerformed(context.Background())
			layers, err := h.Layers(ctx)
			if !assertErrorAndContinue(t, err, tc.expectedError) {
				return
			}

			if !reflect.DeepEqual(layers, tc.expectedLayers) {
				t.Fatalf("got unexpected docker image layers: %s", diff.ObjectGoPrintDiff(layers, tc.expectedLayers))
			}
		})
	}
}

func TestManifestSchema1Handler_Metadata(t *testing.T) {
	for _, tc := range []struct {
		name              string
		manifestString    string
		metadataString    string
		blobDescriptors   blobDescriptors
		expectedImageSize int64
		expectedError     error
	}{
		{
			name:              "sizes in manifest",
			manifestString:    manifestSchema1,
			metadataString:    manifestSchema1Metadata,
			expectedImageSize: 1095501,
		},

		{
			name:           "manifest without layer sizes",
			manifestString: manifestSchema1WithoutSize,
			metadataString: manifestSchema1Metadata,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   100,
				},
			},
			expectedImageSize: 100,
		},

		{
			name:           "manifest sizes take precedence",
			manifestString: manifestSchema1,
			metadataString: manifestSchema1Metadata,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   5,
				},
			},
			expectedImageSize: 1095501,
		},

		{
			name:           "manifest with shorter history",
			manifestString: manifestSchema1ShortHistory,
			metadataString: manifestSchema1Metadata,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[1]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[1]),
					Size:   100,
				},
			},
			expectedImageSize: 100,
		},

		{
			name:           "manifest with shorter fs layers",
			manifestString: manifestSchema1ShortFSLayers,
			metadataString: manifestSchema1Metadata,
			blobDescriptors: blobDescriptors{
				digest.Digest(manifestSchema1Layers[0]): distribution.Descriptor{
					Digest: digest.Digest(manifestSchema1Layers[0]),
					Size:   100,
				},
			},
			expectedImageSize: 5,
		},

		{
			name:           "manifest with no layers",
			manifestString: manifestSchema1NoLayers,
			expectedError:  ErrNoManifestMetadata,
		},

		{
			name:           "blob unknown",
			manifestString: manifestSchema1WithoutSize,
			expectedError:  distribution.ErrBlobUnknown,
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			manifest := tryUnmarshalManifestSchema1OrGenerateSignatures(t, tc.manifestString)

			bds := blobDescriptors{
				digestSHA256GzippedEmptyTar: distribution.Descriptor{
					Digest: digestSHA256GzippedEmptyTar,
					Size:   0,
				},
			}
			for d, desc := range tc.blobDescriptors {
				bds[d] = desc
			}

			bs := newTestBlobStore(bds, nil)
			_, imageClient := testutil.NewFakeOpenShiftWithClient()
			repo := newTestRepository(t, "nm", "repo", testRepositoryOptions{
				client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
				blobs:  bs,
			})
			h, err := NewManifestHandler(repo, manifest)
			if err != nil {
				t.Fatal(err)
			}

			ctx := withAuthPerformed(context.Background())
			meta, err := h.Metadata(ctx)
			if !assertErrorAndContinue(t, err, tc.expectedError) {
				return
			}

			if meta.Created.IsZero() {
				t.Errorf("unexpected non-zero Created value")
			}

			expMeta := imageapi.DockerImage{}
			if err := json.Unmarshal([]byte(tc.metadataString), &expMeta); err != nil {
				t.Fatal(err)
			}
			expMeta.Size = tc.expectedImageSize
			expMeta.TypeMeta = meta.TypeMeta

			if !reflect.DeepEqual(meta, &expMeta) {
				t.Fatalf("got unexpected image metadata: %s", diff.ObjectGoPrintDiff(meta, &expMeta))
			}
		})
	}
}

var _signingKey libtrust.PrivateKey

func getSigningKey(t *testing.T) libtrust.PrivateKey {
	if _signingKey == nil {
		sk, err := libtrust.GenerateECP256PrivateKey()
		if err != nil {
			t.Fatalf("failed to generate signing key: %v", err)
		}
		_signingKey = sk
	}
	return _signingKey
}

func assertErrorAndContinue(t *testing.T, e, exp error) bool {
	if e != nil {
		if exp == nil {
			t.Fatalf("got unexpected error: (%T) %v", e, e)
		}
		if e.Error() != exp.Error() {
			t.Fatalf("got unexpected error: %s", diff.ObjectGoPrintDiff(e, exp))
		}
		return false
	}
	if e == nil && exp != nil {
		t.Fatalf("got non-error while expecting: %v", exp)
	}

	return true
}

func tryUnmarshalManifestSchema1OrGenerateSignatures(t *testing.T, manifestString string) *schema1.SignedManifest {
	manifest, err := unmarshalManifestSchema1([]byte(manifestString), [][]byte{})
	if err != nil {
		t.Logf("failed to unmarshal manifest because of: %v\ntrying to generate signatures...", err)
		ms1 := schema1.Manifest{}
		if err := json.Unmarshal([]byte(manifestString), &ms1); err != nil {
			t.Fatalf("failed to unmarshal manifest json: %v", err)
		}
		signedManifest, err := schema1.Sign(&ms1, getSigningKey(t))
		if err != nil {
			t.Fatalf("failed to sign manifest: %v", err)
		}
		payload, err := signedManifest.MarshalJSON()
		if err != nil {
			t.Fatalf("failed to serialize manifest: %v", err)
		}
		t.Logf("signed manifest:\n%s\n", string(payload))
		t.Fatalf("rewrite the manifest string according to the output")
	}

	sm, ok := manifest.(*schema1.SignedManifest)
	if !ok {
		t.Fatalf("got unexpected manifest schema: %T", sm)
	}

	return sm
}

// imported from docker.io/busybox:1.23
const manifestSchema1Digest = `sha256:2780635f864cc66c7a5c74aca8047970b95cb91b6d5c135964d984ffe07a2024`
const manifestSchema1Metadata = "{\"Id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"Parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Created\":\"2015-09-21T20:15:47.866196515Z\",\"Container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"ContainerConfig\":{\"Hostname\":\"5f8e0e129ff1\",\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\"},\"DockerVersion\":\"1.8.2\",\"Config\":{\"Hostname\":\"5f8e0e129ff1\",\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\"},\"Architecture\":\"amd64\",\"Size\":1095501}\n"
const manifestSchema1Signature = "{\"header\":{\"jwk\":{\"crv\":\"P-256\",\"kid\":\"QKEZ:N7ZA:BUSY:KPSH:PARP:NU4K:POHK:VLWF:EW22:4JFB:MKYJ:ZYSE\",\"kty\":\"EC\",\"x\":\"ppU7aXPngzHYJUswWcpDDL50hYkHWanmcrs_0X8L8Pc\",\"y\":\"dRpAggds8FfHRZsOms_g13XBOMnuqkP1fEWisGwvXso\"},\"alg\":\"ES256\"},\"signature\":\"KixitWkKYsVqNL0mkSxVSZMXQ61tzgXTlTlyeLHz4I2dZNXdDwHJZmYeoMGnYKM_HQKDcQHQeYSoxlu8AMTLOQ\",\"protected\":\"eyJmb3JtYXRMZW5ndGgiOjMyMTAsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xNVQwOTo0MzowNFoifQ\"}"

var manifestSchema1Layers = []string{
	digestSHA256GzippedEmptyTar.String(),
	"sha256:9d7588d3c0635b53bd9a7dcd40bdf5d2d32cd3fb919c3a29ec2febbc2449eb19",
}

// imported from docker.io/busybox:1.23
const manifestSchema1 = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:9d7588d3c0635b53bd9a7dcd40bdf5d2d32cd3fb919c3a29ec2febbc2449eb19"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.866196515Z\",\"container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.433616227Z\",\"container\":\"5f8e0e129ff1e03bbb50a8b6ba7636fa5503c695125b1c392490d8aa113e8cf6\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:6cccb5f0a3b3947116a0c0f55d071980d94427ba0d6dad17bc68ead832cc0a8f in /\"],\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":1095501}\n"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "QKEZ:N7ZA:BUSY:KPSH:PARP:NU4K:POHK:VLWF:EW22:4JFB:MKYJ:ZYSE",
               "kty": "EC",
               "x": "ppU7aXPngzHYJUswWcpDDL50hYkHWanmcrs_0X8L8Pc",
               "y": "dRpAggds8FfHRZsOms_g13XBOMnuqkP1fEWisGwvXso"
            },
            "alg": "ES256"
         },
         "signature": "KixitWkKYsVqNL0mkSxVSZMXQ61tzgXTlTlyeLHz4I2dZNXdDwHJZmYeoMGnYKM_HQKDcQHQeYSoxlu8AMTLOQ",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjMyMTAsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xNVQwOTo0MzowNFoifQ"
      }
   ]
}`

const manifestSchema1WithoutSizeSignature = `{"header":{"jwk":{"crv":"P-256","kid":"IA3H:ZTL6:ZE5F:YBJU:TV2M:NSYK:W7ON:3D2K:5R3T:B7ZR:7J6X:IY4F","kty":"EC","x":"hM0pR9f7IIqWoKsD62bL_9tMmi1l04YRsVcCV_Q8ePw","y":"Lw1BZJLmNnII5Zt0Uk3nfqbDSDvqbZ5_ay4CM89AUTc"},"alg":"ES256"},"signature":"xlqhy7h3GLoiG_Z4sTwuvjA7t7pv9Jmc74kKkv8cozxvEPGvNOVgpnFDXtRkcfPIUNZAB8LJ6zMQWGkB5akSZA","protected":"eyJmb3JtYXRMZW5ndGgiOjMxOTMsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxMzozNDowNFoifQ"}`
const manifestSchema1WithoutSize = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:9d7588d3c0635b53bd9a7dcd40bdf5d2d32cd3fb919c3a29ec2febbc2449eb19"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.866196515Z\",\"container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.433616227Z\",\"container\":\"5f8e0e129ff1e03bbb50a8b6ba7636fa5503c695125b1c392490d8aa113e8cf6\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:6cccb5f0a3b3947116a0c0f55d071980d94427ba0d6dad17bc68ead832cc0a8f in /\"],\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}\n"
      }
   ],
   "signatures": [
     {
        "header": {
           "jwk": {
              "crv": "P-256",
              "kid": "IA3H:ZTL6:ZE5F:YBJU:TV2M:NSYK:W7ON:3D2K:5R3T:B7ZR:7J6X:IY4F",
              "kty": "EC",
              "x": "hM0pR9f7IIqWoKsD62bL_9tMmi1l04YRsVcCV_Q8ePw",
              "y": "Lw1BZJLmNnII5Zt0Uk3nfqbDSDvqbZ5_ay4CM89AUTc"
           },
           "alg": "ES256"
        },
        "signature": "xlqhy7h3GLoiG_Z4sTwuvjA7t7pv9Jmc74kKkv8cozxvEPGvNOVgpnFDXtRkcfPIUNZAB8LJ6zMQWGkB5akSZA",
        "protected": "eyJmb3JtYXRMZW5ndGgiOjMxOTMsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxMzozNDowNFoifQ"
     }
  ]
}`

const manifestSchema1ShortHistorySignature = `{"header":{"jwk":{"crv":"P-256","kid":"BMQ5:5OIV:TJXC:IJQE:BYCE:7UBD:SWFQ:HFBN:STVV:XDNE:VJRG:KUUA","kty":"EC","x":"rZo1KLwKH0ZfiTzGFxTTQxbarJZ7gE4fWuPrucpZwjo","y":"QkoUQ3HauBjMythY94qevDCKzMEiLYJse3cVSqrSO4k"},"alg":"ES256"},"signature":"Fn_Diinka9s_cYTBvHoSklrBm3oM8rYe7PNZwEg_hAB-g0SOvTmiCqFjC9QahvhFtUZYT3cpZpJLFzRVAU32Tg","protected":"eyJmb3JtYXRMZW5ndGgiOjE4NTksImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxNDozMzo0MFoifQ"}`
const manifestSchema1ShortHistory = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:9d7588d3c0635b53bd9a7dcd40bdf5d2d32cd3fb919c3a29ec2febbc2449eb19"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.866196515Z\",\"container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "BMQ5:5OIV:TJXC:IJQE:BYCE:7UBD:SWFQ:HFBN:STVV:XDNE:VJRG:KUUA",
               "kty": "EC",
               "x": "rZo1KLwKH0ZfiTzGFxTTQxbarJZ7gE4fWuPrucpZwjo",
               "y": "QkoUQ3HauBjMythY94qevDCKzMEiLYJse3cVSqrSO4k"
            },
            "alg": "ES256"
         },
         "signature": "Fn_Diinka9s_cYTBvHoSklrBm3oM8rYe7PNZwEg_hAB-g0SOvTmiCqFjC9QahvhFtUZYT3cpZpJLFzRVAU32Tg",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjE4NTksImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxNDozMzo0MFoifQ"
      }
   ]
}`

const manifestSchema1ShortFSLayersSignature = `{"header":{"jwk":{"crv":"P-256","kid":"JV5N:BVLF:L6WC:TVCF:7QJS:FB63:DGAS:IVJV:QQ2U:P77G:SVUF:TJPL","kty":"EC","x":"6cbmNJxXJi09n1hM1Yw5_vWeueCDjHGKXTyzQkH6KkM","y":"XSoPqwZ9pL8mQZkKAJb_SuUhtHsBN1_MP0sB6Bz4RN4"},"alg":"ES256"},"signature":"sdJzNKAlPrIeV4ftAwoSGBO3SP0p3ciqsSaj19Q-zDpgrU6R5L4uGp2OiP7yt5gz8w5kQScbjACrrfS-hcZTkA","protected":"eyJmb3JtYXRMZW5ndGgiOjMwODIsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxNTowNDowMFoifQ"}`
const manifestSchema1ShortFSLayers = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.866196515Z\",\"container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":5}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.433616227Z\",\"container\":\"5f8e0e129ff1e03bbb50a8b6ba7636fa5503c695125b1c392490d8aa113e8cf6\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:6cccb5f0a3b3947116a0c0f55d071980d94427ba0d6dad17bc68ead832cc0a8f in /\"],\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}\n"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "JV5N:BVLF:L6WC:TVCF:7QJS:FB63:DGAS:IVJV:QQ2U:P77G:SVUF:TJPL",
               "kty": "EC",
               "x": "6cbmNJxXJi09n1hM1Yw5_vWeueCDjHGKXTyzQkH6KkM",
               "y": "XSoPqwZ9pL8mQZkKAJb_SuUhtHsBN1_MP0sB6Bz4RN4"
            },
            "alg": "ES256"
         },
         "signature": "sdJzNKAlPrIeV4ftAwoSGBO3SP0p3ciqsSaj19Q-zDpgrU6R5L4uGp2OiP7yt5gz8w5kQScbjACrrfS-hcZTkA",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjMwODIsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0xOFQxNTowNDowMFoifQ"
      }
   ]
}`

const manifestSchema1NoLayers = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [],
   "history": [],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "4ZEJ:RG7V:AYDT:YJDG:E4QU:3PDO:KZBH:REE3:VMB5:2MBZ:BW7L:3HUF",
               "kty": "EC",
               "x": "mEFmDF5f4rVaJSNwLH7dyaaYPPi--L3V6Oqq5bvtZTA",
               "y": "RqymHTBZ7UQenhOsqKhzwDDNjmMHSEuVujYZxwoJVjw"
            },
            "alg": "ES256"
         },
         "signature": "OK7YO7yFRTBcipZ7qgx7K5SHSEzqV99D9EkKM5oLBYbKl2ouQDv-wORH3QNARynRGqPbQ1Dyjpi-4z2kSvc74w",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjEzNiwiZm9ybWF0VGFpbCI6IkNuMCIsInRpbWUiOiIyMDE3LTA5LTE5VDEzOjA0OjA4WiJ9"
      }
   ]
}`

var manifestSchema1ExternalSignatures = [][]byte{[]byte(`{
   "header": {
  	"jwk": {
  	   "crv": "P-256",
  	   "kid": "QGG7:JZ2V:PFXZ:NKUP:XDPM:V3GS:KRRG:NB27:D4RF:2FQY:ISZV:TYUB",
  	   "kty": "EC",
  	   "x": "9itRpQlCqD-vlbSvGH9laJIuM9PfDOU7-mJ42zkFu7E",
  	   "y": "zGP4n85_A2XgzZ770E3IWAijI0W5kbmv0FrgDPEcFMM"
  	},
  	"alg": "ES256"
   },
   "signature": "HbWKBd8wRh20G0HAO7qfFgviW2AI8a5woKM48fhTcPuJXr0qA9CyMoEdfrHFk_vwplv4w8CZImizfHbZ3UxzoQ",
   "protected": "eyJmb3JtYXRMZW5ndGgiOjE3NDgsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0yMFQxMzoxNDozOVoifQ"
}`)}
var manifestSchema1ExternalSignaturesCompact = [][]byte{[]byte("{\"header\":{\"jwk\":{\"crv\":\"P-256\",\"kid\":\"QGG7:JZ2V:PFXZ:NKUP:XDPM:V3GS:KRRG:NB27:D4RF:2FQY:ISZV:TYUB\",\"kty\":\"EC\",\"x\":\"9itRpQlCqD-vlbSvGH9laJIuM9PfDOU7-mJ42zkFu7E\",\"y\":\"zGP4n85_A2XgzZ770E3IWAijI0W5kbmv0FrgDPEcFMM\"},\"alg\":\"ES256\"},\"signature\":\"HbWKBd8wRh20G0HAO7qfFgviW2AI8a5woKM48fhTcPuJXr0qA9CyMoEdfrHFk_vwplv4w8CZImizfHbZ3UxzoQ\",\"protected\":\"eyJmb3JtYXRMZW5ndGgiOjE3NDgsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wOS0yMFQxMzoxNDozOVoifQ\"}")}

const manifestSchema1MissingSignatures = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"d7057cb020844f245031d27b76cb18af05db1cc3a96a29fa7777af75f5ac91a3\",\"parent\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"created\":\"2015-09-21T20:15:47.866196515Z\",\"container\":\"7f652467f9e6d1b3bf51172868b9b0c2fa1c711b112f4e987029b1624dd6295f\",\"container_config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2\",\"config\":{\"Hostname\":\"5f8e0e129ff1\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"cfa753dfea5e68a24366dfba16e6edf573daa447abf65bc11619c1a98a3aff54\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":5}\n"
      }
   ]
}`

const manifestSchema1Invalid = `{
   "schemaVersion": 1,
   "name": "library/busybox",
   "tag": "1.23",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
   ],
   "history": [],
   "signatures": [],
}`
