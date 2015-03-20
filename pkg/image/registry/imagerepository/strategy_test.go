package imagerepository

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)
import "github.com/openshift/origin/pkg/image/api"
import "testing"

type fakeDefaultRegistry struct {
	registry string
}

func (f *fakeDefaultRegistry) DefaultRegistry() (string, bool) {
	return f.registry, len(f.registry) > 0
}

func TestDockerImageRepository(t *testing.T) {
	tests := map[string]struct {
		repo            *api.ImageRepository
		expected        string
		defaultRegistry string
	}{
		"DockerImageRepository set on repo": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Name: "somerepo",
				},
				DockerImageRepository: "a/b",
			},
			expected: "a/b",
		},
		"default namespace": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Name: "somerepo",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/default/somerepo",
		},
		"nondefault namespace": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/somens/somerepo",
		},
		"missing default registry": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "",
			expected:        "",
		},
	}

	for testName, test := range tests {
		strategy := NewStrategy(&fakeDefaultRegistry{test.defaultRegistry})
		value := strategy.dockerImageRepository(test.repo)
		if e, a := test.expected, value; e != a {
			t.Errorf("%s: expected %q, got %q", testName, e, a)
		}
	}
}

func TestTagsChanged(t *testing.T) {
	tests := map[string]struct {
		tags               map[string]string
		existingTagHistory map[string]api.TagEventList
		expectedTagHistory map[string]api.TagEventList
		repo               string
		previous           map[string]string
	}{
		"no tags, no history": {
			repo:               "registry:5000/ns/repo",
			tags:               make(map[string]string),
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: make(map[string]api.TagEventList),
		},
		"single tag update, preserves history": {
			repo:     "registry:5000/ns/repo",
			previous: map[string]string{},
			tags:     map[string]string{"t1": "t1"},
			existingTagHistory: map[string]api.TagEventList{
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"empty tag ignored on create": {
			repo:               "registry:5000/ns/repo",
			tags:               map[string]string{"t1": ""},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{},
		},
		"tag to missing ignored on create": {
			repo:               "registry:5000/ns/repo",
			tags:               map[string]string{"t1": "t2"},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{},
		},
		"new tags, no history": {
			repo:               "registry:5000/ns/repo",
			tags:               map[string]string{"t1": "t1", "t2": "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						//TODO use the line below when we're on Docker 1.6 with true pull by digest
						//DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						DockerImageReference: "registry:5000/ns/repo:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"no-op": {
			repo:     "registry:5000/ns/repo",
			previous: map[string]string{"t1": "v1image1", "t2": "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			tags:     map[string]string{"t1": "v1image1", "t2": "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			existingTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"new tag copies existing history": {
			repo:     "registry:5000/ns/repo",
			previous: map[string]string{"t1": "t1", "t3": "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			tags:     map[string]string{"t1": "t1", "t2": "t1", "t3": "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			existingTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:v1image1",
						Image:                "v1image1",
					},
				}},
				"t3": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:v1image1",
						Image:                "v1image1",
					},
				}},
				// tag copies existing history
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo:v1image1",
						Image:                "v1image1",
					},
				}},
				"t3": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
	}

	for testName, test := range tests {
		fmt.Println(testName)
		repo := &api.ImageRepository{
			Tags: test.tags,
			Status: api.ImageRepositoryStatus{
				DockerImageRepository: test.repo,
				Tags: test.existingTagHistory,
			},
		}
		previousRepo := &api.ImageRepository{
			Tags: test.previous,
			Status: api.ImageRepositoryStatus{
				DockerImageRepository: test.repo,
				Tags: test.existingTagHistory,
			},
		}
		if test.previous == nil {
			previousRepo = nil
		}

		tagsChanged(previousRepo, repo)

		if !reflect.DeepEqual(test.tags, repo.Tags) {
			t.Errorf("%s: repo.Tags was unexpectedly updated: %#v", testName, repo.Tags)
			continue
		}

		for expectedTag, expectedTagHistory := range test.expectedTagHistory {
			updatedTagHistory, ok := repo.Status.Tags[expectedTag]
			if !ok {
				t.Errorf("%s: missing history for tag %q", testName, expectedTag)
				continue
			}
			if e, a := len(expectedTagHistory.Items), len(updatedTagHistory.Items); e != a {
				t.Errorf("%s: tag %q: expected %d in history, got %d: %#v", testName, expectedTag, e, a, updatedTagHistory)
				continue
			}
			for i, expectedTagEvent := range expectedTagHistory.Items {
				if e, a := expectedTagEvent.Image, updatedTagHistory.Items[i].Image; e != a {
					t.Errorf("%s: tag %q: docker image id: expected %q, got %q", testName, expectedTag, e, a)
					continue
				}
				if e, a := expectedTagEvent.DockerImageReference, updatedTagHistory.Items[i].DockerImageReference; e != a {
					t.Errorf("%s: tag %q: docker image reference: expected %q, got %q", testName, expectedTag, e, a)
				}
			}
		}
	}
}
