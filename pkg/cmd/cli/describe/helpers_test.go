package describe

import (
	"bytes"
	"testing"
	"text/tabwriter"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestFormatImageStreamTags(t *testing.T) {
	repo := imageapi.ImageStream{
		Spec: imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{
				"spec1": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "foo",
						Name:      "bar:latest",
					},
				},
				"spec2": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "mysql",
						Name:      "latest@sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
					},
				},
			},
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				imageapi.DefaultImageTag: {
					Items: []imageapi.TagEvent{
						{
							Created:              unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
							Image:                "sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
						},
						{
							Created:              unversioned.Date(2015, 3, 23, 7, 15, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:062b80555a5dd7f5d58e78b266785a399277ff8c3e402ce5fa5d8571788e6bad",
							Image:                "sha256:062b80555a5dd7f5d58e78b266785a399277ff8c3e402ce5fa5d8571788e6bad",
						},
					},
				},
				"spec1": {
					Items: []imageapi.TagEvent{
						{
							Created:              unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
							Image:                "sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
						},
					},
				},
				"spec2": {
					Items: []imageapi.TagEvent{
						{
							Created:              unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "mysql:latest",
							Image:                "sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
						},
					},
				},
			},
		},
	}

	out := new(tabwriter.Writer)
	b := make([]byte, 1024)
	buf := bytes.NewBuffer(b)
	out.Init(buf, 0, 8, 1, '\t', 0)

	formatImageStreamTags(out, &repo)
	out.Flush()
	actual := string(buf.String())
	t.Logf("\n%s", actual)
}

func TestFormatQuantity(t *testing.T) {
	testCases := []struct {
		value    int64
		scale    scale
		expected string
	}{
		{
			value:    1 << 30,
			scale:    giga,
			expected: "1GiB",
		},
		{
			value:    1 << 20,
			scale:    mega,
			expected: "1MiB",
		},
		{
			value:    10 * (1 << 20),
			scale:    giga,
			expected: "0.01GiB",
		},
		{
			value:    (1 << 30) + (1 << 20),
			scale:    giga,
			expected: "1GiB",
		},
		{
			value:    (1 << 30) + 10*(1<<20),
			scale:    giga,
			expected: "1.01GiB",
		},
	}

	for idx, test := range testCases {
		quantity := resource.NewQuantity(test.value, resource.BinarySI)
		actual := formatQuantity(quantity, test.scale)
		if actual != test.expected {
			t.Errorf("(%d) expected '%s', got '%s'", idx, test.expected, actual)
		}
	}
}
