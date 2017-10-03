package server

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"
)

func TestUnmarshalManifestSchema2(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		manifestString         string
		expectedConfig         distribution.Descriptor
		expectedReferences     []distribution.Descriptor
		expectedErrorSubstring string
	}{
		{
			name:           "valid nginx image with sizes in manifest",
			manifestString: manifestSchema2,
			expectedConfig: manifestSchema2ConfigDescriptor,
			expectedReferences: []distribution.Descriptor{
				manifestSchema2ConfigDescriptor,
				manifestSchema2LayerDescriptors[0],
				manifestSchema2LayerDescriptors[1],
				manifestSchema2LayerDescriptors[2],
			},
		},

		{
			name:                   "invalid schema2 image",
			manifestString:         manifestSchema2Invalid,
			expectedErrorSubstring: "invalid character",
		},

		{
			name:                   "manifest schema1 image",
			manifestString:         manifestSchema1,
			expectedErrorSubstring: "unexpected manifest schema version",
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			manifest, err := unmarshalManifestSchema2([]byte(tc.manifestString))
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

			dm, ok := manifest.(*schema2.DeserializedManifest)
			if !ok {
				t.Fatalf("got unexpected manifest schema: %T", manifest)
			}

			if !reflect.DeepEqual(dm.Config, tc.expectedConfig) {
				t.Errorf("got unexpected image config descriptor: %s", diff.ObjectGoPrintDiff(dm.Config, tc.expectedConfig))
			}

			refs := dm.References()
			if !reflect.DeepEqual(refs, tc.expectedReferences) {
				t.Errorf("got unexpected image references: %s", diff.ObjectGoPrintDiff(refs, tc.expectedReferences))
			}
		})
	}
}

var manifestSchema2LayerDescriptors = []distribution.Descriptor{
	{
		MediaType: schema2.MediaTypeLayer,
		Digest:    digest.Digest("sha256:afeb2bfd31c0760573e7262de6ae67a84da0e0a1c3e8157bbddd41a501b18a5c"),
		Size:      22488057,
	},
	{
		MediaType: schema2.MediaTypeLayer,
		Digest:    digest.Digest("sha256:7ff5d10493db2cdfc1b7238434c503cc0664d48d0f7154ea9472e734b28a72dd"),
		Size:      21869700,
	},
	{
		MediaType: schema2.MediaTypeLayer,
		Digest:    digest.Digest("sha256:d2562f1ae1d0593a26c54006ad0a6211c35fdc8b4067485d7208000d83477de2"),
		Size:      201,
	},
}

const manifestSchema2ConfigDigest = `sha256:da5939581ac835614e3cf6c765e7489e6d0fc602a44e98c07013f1c938f49675`

var manifestSchema2ConfigDescriptor = distribution.Descriptor{
	Digest:    digest.Digest(manifestSchema2ConfigDigest),
	Size:      5838,
	MediaType: schema2.MediaTypeConfig,
}

const manifestSchema2 = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "size": 5838,
      "digest": "sha256:da5939581ac835614e3cf6c765e7489e6d0fc602a44e98c07013f1c938f49675"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 22488057,
         "digest": "sha256:afeb2bfd31c0760573e7262de6ae67a84da0e0a1c3e8157bbddd41a501b18a5c"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 21869700,
         "digest": "sha256:7ff5d10493db2cdfc1b7238434c503cc0664d48d0f7154ea9472e734b28a72dd"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 201,
         "digest": "sha256:d2562f1ae1d0593a26c54006ad0a6211c35fdc8b4067485d7208000d83477de2"
      }
   ]
}`

const manifestSchema2Invalid = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {},
   []
}`
