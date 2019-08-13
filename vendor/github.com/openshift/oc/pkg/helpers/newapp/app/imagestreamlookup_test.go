package app

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imagefake "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/reference"
)

func testImageStreamClient(imageStreams *imagev1.ImageStreamList, images map[string]*imagev1.ImageStreamImage) imagev1client.ImageV1Interface {
	fake := &imagefake.Clientset{}

	fake.AddReactor("list", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, imageStreams, nil
	})
	fake.AddReactor("get", "imagestreamimages", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, images[action.(clientgotesting.GetAction).GetName()], nil
	})

	return fake.ImageV1()
}

func TestImageStreamByAnnotationSearcherAndResolver(t *testing.T) {
	streams, images := fakeImageStreams(
		&fakeImageStreamDesc{
			name: "ruby",
			tags: []imagev1.TagReference{
				{
					Name: "ruby20",
					Annotations: map[string]string{
						"supports": "ruby:2.0,ruby:2.1,ruby",
					},
				},
				{
					Name: "ruby19",
					Annotations: map[string]string{
						"supports": "ruby:1.9,ruby:1.9.4,ruby",
					},
				},
			},
		},
		&fakeImageStreamDesc{
			name: "wildfly",
			tags: []imagev1.TagReference{
				{
					Name: "v8",
					Annotations: map[string]string{
						"supports": "wildfly:8.0,java,jee",
					},
				},
				{
					Name: "v7",
					Annotations: map[string]string{
						"supports": "wildfly:7.0,java",
					},
				},
			},
		},
	)
	client := testImageStreamClient(streams, images)
	searcher := NewImageStreamByAnnotationSearcher(client, client, []string{"default"})
	resolver := UniqueExactOrInexactMatchResolver{Searcher: searcher}
	tests := []struct {
		value       string
		expectMatch bool
		precise     bool
	}{
		{
			value:       "ruby:2.0",
			expectMatch: true,
		},
		{
			value:       "python",
			expectMatch: false,
		},
		{
			value:       "jee:1.6",
			expectMatch: true,
		},
		{
			value:       "java",
			expectMatch: false,
		},
	}

	for _, test := range tests {
		searchResults, errs := searcher.Search(test.precise, test.value)
		if errs != nil {
			t.Errorf("unexpected errors: %v", errs)
		}
		if len(searchResults) == 0 && test.expectMatch {
			t.Errorf("Expected a search match for %s. Got none", test.value)
		}
		if len(searchResults) == 1 && !test.expectMatch {
			t.Errorf("Did not expect search a match for %s. Got a match", test.value)
		}

		_, err := resolver.Resolve(test.value)
		if err != nil && test.expectMatch {
			t.Errorf("Expected a resolve match for %s. Got none", test.value)
		}
		if err == nil && !test.expectMatch {
			t.Errorf("Did not expect resolve a match for %s. Got a match", test.value)
		}
	}
}

func TestImageStreamSearcher(t *testing.T) {
	streams, images := fakeImageStreams(
		&fakeImageStreamDesc{
			name: "nodejs1",
			tags: []imagev1.TagReference{
				{
					Name: "0.10",
					Annotations: map[string]string{
						"supports": "nodejs1:0.10,nodejs1:0.1,nodejs1",
						"tags":     "hidden",
					},
				},
				{
					Name: "4",
					Annotations: map[string]string{
						"supports": "nodejs1:4,nodejs1",
					},
				},
			},
		},
		&fakeImageStreamDesc{
			name: "nodejs2",
			tags: []imagev1.TagReference{
				{
					Name: "0.10",
					Annotations: map[string]string{
						"supports": "nodejs2:0.10,nodejs2:0.1,nodejs2",
						"tags":     "hidden",
					},
				},
			},
		},
		&fakeImageStreamDesc{
			name: "nodejs3",
			tags: []imagev1.TagReference{
				{
					Name: "4",
					Annotations: map[string]string{
						"supports": "nodejs3:4,nodejs3",
						"tags":     "hidden",
					},
				},
			},
			latest: "4",
		},
		&fakeImageStreamDesc{
			name: "nodejs4",
			tags: []imagev1.TagReference{
				{
					Name: "0.10",
					Annotations: map[string]string{
						"supports": "nodejs4:0.10,nodejs4:0.1,nodejs4",
					},
				},
				{
					Name: "4",
					Annotations: map[string]string{
						"supports": "nodejs4:4,nodejs4",
						"tags":     "hidden",
					},
				},
			},
			latest: "4",
			latestannotations: map[string]string{
				"tags": "hidden",
			},
		},
		&fakeImageStreamDesc{
			name: "ruby20",
			tags: []imagev1.TagReference{
				{
					Name: "stable",
					Annotations: map[string]string{
						"supports": "ruby:1.9,ruby:1.9.4",
					},
				},
			},
		},
		&fakeImageStreamDesc{
			name: "wildfly",
			tags: []imagev1.TagReference{
				{
					Name: "v8",
					Annotations: map[string]string{
						"supports": "java,jee",
					},
				},
				{
					Name: "v7",
					Annotations: map[string]string{
						"supports": "java",
					},
				},
			},
			latest: "v8",
		},
	)
	client := testImageStreamClient(streams, images)
	searcher := ImageStreamSearcher{Client: client, Namespaces: []string{"default"}}
	resolver := UniqueExactOrInexactMatchResolver{Searcher: searcher}
	tests := []struct {
		value       string
		precise     bool
		expectMatch bool
		expectTag   string
	}{
		{
			value:       "ruby20",
			expectMatch: true,
		},
		{
			value:       "ruby2.0",
			expectMatch: false,
		},
		{
			value:       "wildfly",
			expectMatch: true,
			expectTag:   "v8",
		},
		{
			value:       "nodejs1",
			expectMatch: true,
			expectTag:   "4",
		},
		{
			value:       "nodejs2",
			expectMatch: false,
		},
		{
			value:       "nodejs3",
			expectMatch: true,
			expectTag:   "latest",
		},
		{
			value:       "nodejs4",
			expectMatch: true,
			expectTag:   "0.10",
		},
	}

	for _, test := range tests {
		searchResults, errs := searcher.Search(test.precise, test.value)
		if len(searchResults) == 0 && test.expectMatch {
			t.Errorf("Expected a search match for %s. Got none: %v", test.value, errs)
		}
		if len(searchResults) == 1 && !test.expectMatch {
			t.Errorf("Did not expect a search match for %s. Got a match: %#v", test.value, searchResults[0])
		}

		result, err := resolver.Resolve(test.value)
		if err != nil && test.expectMatch {
			t.Errorf("Expected a resolve match for %s. Got none: %v", test.value, err)
		}
		if err == nil && !test.expectMatch {
			t.Errorf("Did not expect a resolve match for %s. Got a match: %#v", test.value, result)
		}
		if err != nil {
			continue
		}
		if len(test.expectTag) > 0 && result.ImageTag != test.expectTag {
			t.Errorf("Did not expect match for %s to result in tag %s: %#v", test.value, result.ImageTag, result)
		}
	}
}

func TestMatchSupportsAnnotation(t *testing.T) {
	tests := []struct {
		name, value, annotation string
		expectedMatch           bool
		expectedScore           float32
	}{
		{
			name:          "exact match",
			value:         "wildfly:8.0",
			annotation:    "java,wildfly,wildfly:7.0,wildfly:8.0,jee",
			expectedMatch: true,
			expectedScore: 0.0,
		},
		{
			name:          "partial match (version specified)",
			value:         "ruby:2.0",
			annotation:    "ruby:1.9,ruby:1.8",
			expectedMatch: true,
			expectedScore: 0.5,
		},
		{
			name:          "partial match (no version specified)",
			value:         "ruby",
			annotation:    "ruby:2.0,ruby:2.1",
			expectedMatch: true,
			expectedScore: 0.5,
		},
		{
			name:          "partial match (no version in annotation)",
			value:         "ruby:2.1",
			annotation:    "rails,ruby",
			expectedMatch: true,
			expectedScore: 0.5,
		},
		{
			name:          "empty annotation",
			value:         "jee",
			annotation:    "",
			expectedMatch: false,
		},
		{
			name:          "no match",
			value:         "java",
			annotation:    "jee,wildfly:8.0,wildfly:7.0",
			expectedMatch: false,
		},
	}

	for _, test := range tests {
		score, matches := matchSupportsAnnotation(test.value, test.annotation)
		if matches != test.expectedMatch {
			t.Errorf("%s: unexpected match result; got: %v; expected: %v", test.name, matches, test.expectedMatch)
			continue
		}
		if matches && score != test.expectedScore {
			t.Errorf("%s: unexpected score: got: %f; expected: %f", test.name, score, test.expectedScore)
		}
	}
}

func TestAnnotationMatches(t *testing.T) {
	stream, images := fakeImageStream(&fakeImageStreamDesc{
		name: "builder",
		tags: []imagev1.TagReference{
			{
				Name: "ruby",
				Annotations: map[string]string{
					"supports": "ruby,ruby:2.0,ruby:1.9",
				},
			},
			{
				Name: "java",
				Annotations: map[string]string{
					"supports": "java,jee,wildfly",
				},
			},
			{
				Name: "wildfly",
				Annotations: map[string]string{
					"supports": "wildfly:8.0",
				},
			},
		},
		latest: ""})
	client := testImageStreamClient(nil, images)
	searcher := NewImageStreamByAnnotationSearcher(client, client, []string{"default"}).(*ImageStreamByAnnotationSearcher)
	tests := []struct {
		name         string
		value        string
		expectCount  int
		expectScores map[string]float32
	}{
		{
			name:        "exact match",
			value:       "ruby:2.0",
			expectCount: 1,
			expectScores: map[string]float32{
				"ruby": 0.0,
			},
		},
		{
			name:        "exact and partial match",
			value:       "wildfly:8.0",
			expectCount: 2,
			expectScores: map[string]float32{
				"java":    0.5,
				"wildfly": 0.0,
			},
		},
		{
			name:        "partial match",
			value:       "jee:1.5",
			expectCount: 1,
			expectScores: map[string]float32{
				"java": 0.5,
			},
		},
		{
			name:        "no match",
			value:       "php:5.0",
			expectCount: 0,
		},
	}

	for _, test := range tests {
		matches := searcher.annotationMatches(stream, test.value)
		if len(matches) != test.expectCount {
			t.Errorf("%s: unexpected number of matches. Got: %d. Expected: %d\n", test.name, len(matches), test.expectCount)
			continue
		}
		for _, match := range matches {
			expectedScore := test.expectScores[match.DockerImage.ID]
			if match.Score != expectedScore {
				t.Errorf("%s: unexpected score for match %s. Got: %f, Expected: %f\n", test.name, match.DockerImage.ID, match.Score, expectedScore)
			}
		}
	}
}

type fakeImageStreamDesc struct {
	name              string
	tags              []imagev1.TagReference
	latest            string
	latestannotations map[string]string
}

func fakeImageStreams(descs ...*fakeImageStreamDesc) (*imagev1.ImageStreamList, map[string]*imagev1.ImageStreamImage) {
	streams := &imagev1.ImageStreamList{
		Items: []imagev1.ImageStream{},
	}
	allImages := map[string]*imagev1.ImageStreamImage{}
	for _, desc := range descs {
		stream, images := fakeImageStream(desc)
		streams.Items = append(streams.Items, *stream)
		for k, v := range images {
			allImages[k] = v
		}
	}
	return streams, allImages
}

func fakeImageStream(desc *fakeImageStreamDesc) (*imagev1.ImageStream, map[string]*imagev1.ImageStreamImage) {
	stream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desc.name,
			Namespace: "namespace",
		},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{},
		},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{},
		},
	}
	images := map[string]*imagev1.ImageStreamImage{}
	for _, value := range desc.tags {
		stream.Spec.Tags = append(stream.Spec.Tags, value)
		stream.Status.Tags = append(stream.Status.Tags, imagev1.NamedTagEventList{
			Tag: value.Name,
			Items: []imagev1.TagEvent{
				{
					Image: value.Name + "-image",
				},
			},
		})

		imageMeta := runtime.RawExtension{
			Object: &dockerv10.DockerImage{
				ID: value.Name,
			},
		}
		imageMeta.Raw, _ = json.Marshal(imageMeta.Object)

		images[desc.name+"@"+value.Name+"-image"] = &imagev1.ImageStreamImage{
			Image: imagev1.Image{
				DockerImageReference: "example/" + value.Name,
				DockerImageMetadata:  imageMeta,
			},
		}
	}
	if len(desc.latest) > 0 {
		stream.Spec.Tags = append(stream.Spec.Tags, imagev1.TagReference{
			Name: "latest",
			From: &corev1.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      desc.latest,
				Namespace: "namespace",
			},
			Annotations: desc.latestannotations,
		})

		stream.Status.Tags = append(stream.Status.Tags, imagev1.NamedTagEventList{
			Tag: "latest",
			Items: []imagev1.TagEvent{
				{
					Image: desc.latest + "-image",
				},
			},
		})
	}
	return stream, images
}

func TestInputImageFromMatch(t *testing.T) {
	tests := []struct {
		name        string
		match       *ComponentMatch
		expectedTag string
		expectedRef string
	}{
		{
			name: "image stream",
			match: &ComponentMatch{
				ImageStream: &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testimage",
						Namespace: "myns",
					},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: "test/imagename",
					},
				},
			},
			expectedRef: "test/imagename:latest",
		},
		{
			name: "image stream with tag",
			match: &ComponentMatch{
				ImageStream: &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testimage",
						Namespace: "myns",
					},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: "test/imagename",
					},
				},
				ImageTag: "v2",
			},
			expectedRef: "test/imagename:v2",
		},
		{
			name: "container image",
			match: &ComponentMatch{
				DockerImage: &dockerv10.DockerImage{},
				Value:       "test/dockerimage",
			},
			expectedRef: "test/dockerimage",
		},
		{
			name: "container image with tag",
			match: &ComponentMatch{
				DockerImage: &dockerv10.DockerImage{},
				Value:       "test/dockerimage:tag",
			},
			expectedRef: "test/dockerimage:tag",
		},
	}
	for _, test := range tests {
		imgRef, err := InputImageFromMatch(test.match)
		if err != nil {
			t.Errorf("%s: unexpected error: %v\n", test.name, err)
			continue
		}
		expectedRef, _ := reference.Parse(test.expectedRef)
		if !reflect.DeepEqual(imgRef.Reference, expectedRef) {
			t.Errorf("%s: unexpected resulting reference: %#v", test.name, imgRef.Reference)
		}
	}

}
