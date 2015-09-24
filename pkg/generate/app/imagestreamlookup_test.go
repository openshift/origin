package app

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func testImageStreamClient(imageStreams *imageapi.ImageStreamList, images map[string]*imageapi.ImageStreamImage) client.Interface {
	fake := &testclient.Fake{}

	fake.AddReactor("list", "imagestreams", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, imageStreams, nil
	})
	fake.AddReactor("get", "imagestreamimages", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, images[action.(ktestclient.GetAction).GetName()], nil
	})

	return fake
}

func TestImageStreamByAnnotationSearcherAndResolver(t *testing.T) {
	streams, images := fakeImageStreams(
		&fakeImageStreamDesc{
			name: "ruby",
			supports: map[string]string{
				"ruby20": "ruby:2.0,ruby:2.1,ruby",
				"ruby19": "ruby:1.9,ruby:1.9.4,ruby",
			},
		},
		&fakeImageStreamDesc{
			name: "wildfly",
			supports: map[string]string{
				"v8": "wildfly:8.0,java,jee",
				"v7": "wildfly:7.0,java",
			},
		},
	)
	client := testImageStreamClient(streams, images)
	searcher := NewImageStreamByAnnotationSearcher(client, client, []string{"default"})
	resolver := UniqueExactOrInexactMatchResolver{Searcher: searcher}
	tests := []struct {
		value       string
		expectMatch bool
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
		searchResults, _ := searcher.Search(test.value)
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
	stream, images := fakeImageStream("builder", map[string]string{
		"ruby":    "ruby,ruby:2.0,ruby:1.9",
		"java":    "java,jee,wildfly",
		"wildfly": "wildfly:8.0",
	})
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
			expectedScore := test.expectScores[match.Image.ID]
			if match.Score != expectedScore {
				t.Errorf("%s: unexpected score for match %s. Got: %f, Expected: %f\n", test.name, match.Image.ID, match.Score, expectedScore)
			}
		}
	}
}

type fakeImageStreamDesc struct {
	name     string
	supports map[string]string
}

func fakeImageStreams(descs ...*fakeImageStreamDesc) (*imageapi.ImageStreamList, map[string]*imageapi.ImageStreamImage) {
	streams := &imageapi.ImageStreamList{
		Items: []imageapi.ImageStream{},
	}
	allImages := map[string]*imageapi.ImageStreamImage{}
	for _, desc := range descs {
		stream, images := fakeImageStream(desc.name, desc.supports)
		streams.Items = append(streams.Items, *stream)
		for k, v := range images {
			allImages[k] = v
		}
	}
	return streams, allImages
}

func fakeImageStream(name string, supports map[string]string) (*imageapi.ImageStream, map[string]*imageapi.ImageStreamImage) {
	stream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{},
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{},
		},
	}
	images := map[string]*imageapi.ImageStreamImage{}
	for tag, value := range supports {
		stream.Spec.Tags[tag] = imageapi.TagReference{
			Annotations: map[string]string{
				"supports": value,
			},
		}
		stream.Status.Tags[tag] = imageapi.TagEventList{
			Items: []imageapi.TagEvent{
				{
					Image: tag + "-image",
				},
			},
		}
		images[name+"@"+tag+"-image"] = &imageapi.ImageStreamImage{
			Image: imageapi.Image{
				DockerImageReference: "example/" + tag,
				DockerImageMetadata:  imageapi.DockerImage{ID: tag},
			},
		}

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
				ImageStream: &imageapi.ImageStream{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "testimage",
						Namespace: "myns",
					},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: "test/imagename",
					},
				},
			},
			expectedRef: "test/imagename:latest",
		},
		{
			name: "image stream with tag",
			match: &ComponentMatch{
				ImageStream: &imageapi.ImageStream{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "testimage",
						Namespace: "myns",
					},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: "test/imagename",
					},
				},
				ImageTag: "v2",
			},
			expectedRef: "test/imagename:v2",
		},
		{
			name: "docker image",
			match: &ComponentMatch{
				Image: &imageapi.DockerImage{},
				Value: "test/dockerimage",
			},
			expectedRef: "test/dockerimage",
		},
		{
			name: "docker image with tag",
			match: &ComponentMatch{
				Image: &imageapi.DockerImage{},
				Value: "test/dockerimage:tag",
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
		expectedRef, _ := imageapi.ParseDockerImageReference(test.expectedRef)
		if !reflect.DeepEqual(imgRef.DockerImageReference, expectedRef) {
			t.Errorf("%s: unexpected resulting reference: %#v", test.name, imgRef.DockerImageReference)
		}
	}

}
