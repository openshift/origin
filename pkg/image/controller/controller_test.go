package controller

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util"

	client "github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type expectedImage struct {
	Tag   string
	ID    string
	Image *dockerregistry.Image
	Err   error
}

type fakeDockerRegistryClient struct {
	Registry                 string
	Namespace, Name, Tag, ID string
	Insecure                 bool

	Tags    map[string]string
	Err     error
	ConnErr error

	Images []expectedImage

	Called bool
}

func (f *fakeDockerRegistryClient) Connect(registry string, insecure bool) (dockerregistry.Connection, error) {
	f.Called = true
	f.Registry = registry
	f.Insecure = insecure
	return f, f.ConnErr
}

func (f *fakeDockerRegistryClient) ImageTags(namespace, name string) (map[string]string, error) {
	f.Called = true
	f.Namespace, f.Name = namespace, name
	return f.Tags, f.Err
}

func (f *fakeDockerRegistryClient) ImageByTag(namespace, name, tag string) (*dockerregistry.Image, error) {
	f.Called = true
	if len(tag) == 0 {
		tag = api.DefaultImageTag
	}
	f.Namespace, f.Name, f.Tag = namespace, name, tag
	for _, t := range f.Images {
		if t.Tag == tag {
			return t.Image, t.Err
		}
	}
	return nil, dockerregistry.NewImageNotFoundError(fmt.Sprintf("%s/%s", namespace, name), tag, tag)
}

func (f *fakeDockerRegistryClient) ImageByID(namespace, name, id string) (*dockerregistry.Image, error) {
	f.Called = true
	f.Namespace, f.Name, f.ID = namespace, name, id
	for _, t := range f.Images {
		if t.ID == id {
			return t.Image, t.Err
		}
	}
	return nil, dockerregistry.NewImageNotFoundError(fmt.Sprintf("%s/%s", namespace, name), id, "")
}

func TestControllerStart(t *testing.T) {
	two := int64(2)
	testCases := []struct {
		stream *api.ImageStream
		run    bool
	}{
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: "a random error"},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},

		// references are ignored
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:      &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:      &kapi.ObjectReference{Kind: "AnotherImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},

		// spec tag will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
						},
					},
				},
			},
		},
		// spec tag with generation with no pending status will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Items: []api.TagEvent{{Generation: 1}}}},
				},
			},
		},
		// spec tag with generation with status condition error and equal generation will not be imported
		{
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Conditions: []api.TagEventCondition{
						{
							Type:       api.ImportSuccess,
							Status:     kapi.ConditionFalse,
							Generation: 2,
						},
					}}},
				},
			},
		},
		// spec tag with generation with status condition error and older generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Conditions: []api.TagEventCondition{
						{
							Type:       api.ImportSuccess,
							Status:     kapi.ConditionFalse,
							Generation: 1,
						},
					}}},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Items: []api.TagEvent{{Generation: 1}}}},
				},
			},
		},
	}

	for i, test := range testCases {
		fake := &client.Fake{}
		c := ImportController{streams: fake}
		other, err := kapi.Scheme.DeepCopy(test.stream)
		if err != nil {
			t.Fatal(err)
		}

		if err := c.Next(test.stream); err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
		}
		if test.run {
			if len(fake.Actions()) == 0 {
				t.Errorf("%d: expected remote calls: %#v", i, fake)
			}
		} else {
			if !kapi.Semantic.DeepEqual(test.stream, other) {
				t.Errorf("%d: did not expect change to stream: %s", i, util.ObjectGoPrintDiff(test.stream, other))
			}
			if len(fake.Actions()) != 0 {
				t.Errorf("%d: did not expect remote calls", i)
			}
		}
	}
}

func TestControllerExternalRepo(t *testing.T) {
	fake := &client.Fake{}
	c := ImportController{streams: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
		Spec: api.ImageStreamSpec{
			Tags: map[string]api.TagReference{
				"1.1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "some/repo:mytag",
					},
				},
			},
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 actions, got %#v", actions)
	}
	if !actions[0].Matches("create", "imagestreamimports") {
		t.Errorf("expected a create action: %#v", actions)
	}
}
