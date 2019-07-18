package image

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
)

func TestSortStatusTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []imagev1.NamedTagEventList
		expected []string
	}{
		{
			name: "all timestamps here",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "other",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "other-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
							Image:                "other-image",
						},
					},
				},
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "latest-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 53, 0, 0, time.UTC),
							Image:                "latest-image",
						},
					},
				},
				{
					Tag: "third",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "third-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 54, 0, 0, time.UTC),
							Image:                "third-image",
						},
					},
				},
			},
			expected: []string{"third", "latest", "other"},
		},
		{
			name: "two equal timestamps",
			tags: []imagev1.NamedTagEventList{
				{
					Tag: "other",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "other-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
							Image:                "other-image",
						},
					},
				},
				{
					Tag: "latest",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "latest-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 52, 0, 0, time.UTC),
							Image:                "latest-image",
						},
					},
				},
				{
					Tag: "third",
					Items: []imagev1.TagEvent{
						{
							DockerImageReference: "third-ref",
							Created:              metav1.Date(2015, 9, 4, 13, 53, 0, 0, time.UTC),
							Image:                "third-image",
						},
					},
				},
			},
			expected: []string{"third", "latest", "other"},
		},
	}

	for _, test := range tests {
		got := SortStatusTags(test.tags)
		if !reflect.DeepEqual(test.expected, got) {
			t.Errorf("%s: tags mismatch: expected %v, got %v", test.name, test.expected, got)
		}
	}
}
