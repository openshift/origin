package image

import (
	"testing"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
)

func TestLatestTaggedImage(t *testing.T) {
	tests := []struct {
		tag            string
		tags           []imagev1.NamedTagEventList
		expected       string
		expectNotFound bool
	}{
		{
			tag:            "foo",
			tags:           []imagev1.NamedTagEventList{},
			expectNotFound: true,
		},
		{
			tag: "foo",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expectNotFound: true,
		},
		{
			tag: "",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},
		{
			tag: "foo",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
				{
					Tag: "foo",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "foo-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "foo-ref",
		},
	}

	for i, test := range tests {
		stream := &imagev1.ImageStream{}
		stream.Status.Tags = test.tags

		actual := imageutil.LatestTaggedImage(stream, test.tag)
		if actual == nil {
			if !test.expectNotFound {
				t.Errorf("%d: unexpected nil result", i)
			}
			continue
		}
		if e, a := test.expected, actual.DockerImageReference; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestResolveLatestTaggedImage(t *testing.T) {
	tests := []struct {
		tag            string
		statusRef      string
		refs           []imagev1.TagReference
		tags           []imagev1.NamedTagEventList
		expected       string
		expectNotFound bool
	}{
		{
			tag:            "foo",
			tags:           []imagev1.NamedTagEventList{},
			expectNotFound: true,
		},
		{
			tag: "foo",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expectNotFound: true,
		},
		{
			tag: "",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},
		{
			tag: "foo",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
				{
					Tag: "foo",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "foo-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "foo-ref",
		},

		// the default reference policy does nothing
		{
			refs: []imagev1.TagReference{
				{
					Name:            "latest",
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.SourceTagReferencePolicy},
				},
			},
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy does nothing unless reference is set
		{
			refs: []imagev1.TagReference{
				{
					Name:            "latest",
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
				},
			},
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy does nothing unless the image id is set
		{
			statusRef: "test.server/a/b",
			refs: []imagev1.TagReference{
				{
					Name:            "latest",
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
				},
			},
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy uses the output status reference and the image id
		// and returns a pullthrough spec
		{
			statusRef: "test.server/a/b",
			refs: []imagev1.TagReference{
				{
					Name:            "latest",
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
				},
			},
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "test.server/a/b@sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
		},
	}

	for i, test := range tests {
		stream := &imagev1.ImageStream{}
		stream.Status.DockerImageRepository = test.statusRef
		stream.Status.Tags = test.tags
		stream.Spec.Tags = test.refs

		actual, ok := imageutil.ResolveLatestTaggedImage(stream, test.tag)
		if !ok {
			if !test.expectNotFound {
				t.Errorf("%d: unexpected nil result", i)
			}
			continue
		}
		if e, a := test.expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}
