package controller

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

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
}

func (f *fakeDockerRegistryClient) Connect(registry string, insecure bool) (dockerregistry.Connection, error) {
	f.Registry = registry
	f.Insecure = insecure
	return f, f.ConnErr
}

func (f *fakeDockerRegistryClient) ImageTags(namespace, name string) (map[string]string, error) {
	f.Namespace, f.Name = namespace, name
	return f.Tags, f.Err
}

func (f *fakeDockerRegistryClient) ImageByTag(namespace, name, tag string) (*dockerregistry.Image, error) {
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
	f.Namespace, f.Name, f.ID = namespace, name, id
	for _, t := range f.Images {
		if t.ID == id {
			return t.Image, t.Err
		}
	}
	return nil, dockerregistry.NewImageNotFoundError(fmt.Sprintf("%s/%s", namespace, name), id, "")
}

func TestControllerNoOp(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: unversioned.Now().UTC().Format(time.RFC3339)},
			Name:        "test",
			Namespace:   "other",
		},
	}
	other, err := kapi.Scheme.DeepCopy(stream)
	if err != nil {
		t.Fatalf("unexpected deepcopy error: %v", err)
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !kapi.Semantic.DeepEqual(stream, other) {
		t.Errorf("did not expect change to stream")
	}
}

func TestControllerNoDockerRepo(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %#v", actions)
	}
	if !actions[0].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerExternalRepo(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Images: []expectedImage{
			{
				Tag: "mytag",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

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
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %#v", actions)
	}
	if !actions[0].Matches("create", "imagestreammappings") {
		t.Errorf("expected a create action: %#v", actions)
	}
	if !actions[1].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerExternalReferenceRepo(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Images: []expectedImage{
			{
				Tag: "mytag",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

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
					Reference: true,
				},
			},
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %#v", actions)
	}
	if !actions[0].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerExternalRepoFails(t *testing.T) {
	expectedError := fmt.Errorf("test error")
	cli, fake := &fakeDockerRegistryClient{
		Images: []expectedImage{
			{
				Tag: "mytag",
				Err: expectedError,
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

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
	if err := c.Next(&stream); !strings.Contains(err.Error(), expectedError.Error()) {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		t.Errorf("should not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 0 {
		t.Error("expected no actions on fake client")
	}
}

func TestControllerRepoHandled(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %#v", actions)
	}
	if !actions[0].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerTagRetrievalFails(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{Err: fmt.Errorf("test error")}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); err != cli.Err {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		t.Errorf("should not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 0 {
		t.Error("expected no actions on fake client")
	}
}

func TestControllerRetrievesInsecure(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{Err: fmt.Errorf("test error")}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
			Annotations: map[string]string{
				api.InsecureRepositoryAnnotation: "true",
			},
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); err != cli.Err {
		t.Errorf("unexpected error: %v", err)
	}
	if !cli.Insecure {
		t.Errorf("expected insecure call: %#v", cli)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		t.Errorf("should not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 0 {
		t.Error("expected no actions on fake client")
	}
}

func TestControllerImageNotFoundError(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{Tags: map[string]string{api.DefaultImageTag: "not_found"}}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}
	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %#v", actions)
	}
	if !actions[0].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerImageWithGenericError(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{api.DefaultImageTag: "found"},
		Images: []expectedImage{
			{
				ID:  "found",
				Err: fmt.Errorf("test error"),
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}
	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); !strings.Contains(err.Error(), cli.Images[0].Err.Error()) {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		t.Errorf("should not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 0 {
		t.Error("expected no actions on fake client")
	}
}

func TestControllerWithImage(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{api.DefaultImageTag: "found"},
		Images: []expectedImage{
			{
				ID: "found",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}
	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
		},
	}
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !isRFC3339(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) {
		t.Fatalf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %#v", actions)
	}
	if !actions[0].Matches("create", "imagestreammappings") {
		t.Errorf("expected a create action: %#v", actions)
	}
	if !actions[1].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions)
	}
}

func TestControllerWithExternalAndDefaultRegistry(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{"one": "1", "two": "2"},
		Images: []expectedImage{
			{
				ID: "1",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
			{
				ID: "2",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
			{
				Tag: "mytag",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
			Tags: map[string]api.TagReference{
				"ext": {
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
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %#v", actions)
	}
	for i := 0; i < 3; i++ {
		if !actions[i].Matches("create", "imagestreammappings") {
			t.Errorf("expected a create action: %d %#v", i, actions[i])
		}
	}
	if !actions[3].Matches("update", "imagestreams") {
		t.Errorf("expected an update action: %#v", actions[0])
	}
}

func TestControllerWithExternalAndDefaultRegistryErrorOnOneTag(t *testing.T) {
	expectedError := fmt.Errorf("test error")
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{"one": "1", "two": "2"},
		Images: []expectedImage{
			{
				ID: "1",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
			{
				ID:  "2",
				Err: expectedError,
			},
			{
				Tag: "mytag",
				Image: &dockerregistry.Image{
					Image: docker.Image{
						Comment: "foo",
						Config:  &docker.Config{},
					},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "foo/bar",
			Tags: map[string]api.TagReference{
				"ext": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "some/repo:mytag",
					},
				},
			},
		},
	}
	if err := c.Next(&stream); !strings.Contains(err.Error(), cli.Images[1].Err.Error()) {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) != 0 {
		t.Errorf("should not set annotation: %#v", stream)
	}
	actions := fake.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %#v", actions)
	}
	for i := 0; i < 1; i++ {
		if !actions[i].Matches("create", "imagestreammappings") {
			t.Errorf("expected a create action: %d %#v", i, actions[i])
		}
	}
}

func TestControllerWithSpecTags(t *testing.T) {
	tests := map[string]struct {
		dockerImageReference string
		from                 *kapi.ObjectReference
		expectUpdate         bool
	}{
		"no tracking": {
			expectUpdate: true,
		},
		"docker image": {
			from: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: "some/repo:tagX",
			},
			expectUpdate: true,
		},
		"from image stream tag": {
			from: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "2.0",
			},
			expectUpdate: false,
		},
		"from image stream image": {
			from: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "foo@sha256:1234",
			},
			expectUpdate: false,
		},
	}

	for name, test := range tests {
		cli, fake := &fakeDockerRegistryClient{
			Tags: map[string]string{api.DefaultImageTag: "found"},
			Images: []expectedImage{
				{
					ID: "found",
					Image: &dockerregistry.Image{
						Image: docker.Image{
							Comment: "foo",
							Config:  &docker.Config{},
						},
					},
				},
				{
					Tag: "tagX",
					Image: &dockerregistry.Image{
						Image: docker.Image{
							Comment: "foo",
							Config:  &docker.Config{},
						},
					},
				},
			},
		}, &client.Fake{}
		c := ImportController{client: cli, streams: fake, mappings: fake}
		stream := api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
			Spec: api.ImageStreamSpec{
				DockerImageRepository: "foo/bar",
				Tags: map[string]api.TagReference{
					api.DefaultImageTag: {
						From: test.from,
					},
				},
			},
		}
		if err := c.Next(&stream); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
		if !isRFC3339(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) {
			t.Errorf("%s: did not set annotation: %#v", name, stream)
		}
		actions := fake.Actions()
		if test.expectUpdate {
			if len(actions) != 2 {
				t.Errorf("%s: expected an update action: %#v", name, actions)
			}
			if !actions[0].Matches("create", "imagestreammappings") {
				t.Errorf("%s: expected %s, got %v", name, "create-imagestreammappings", actions[0])
			}
			if !actions[1].Matches("update", "imagestreams") {
				t.Errorf("%s: expected %s, got %v", name, "update-imagestreams", actions[1])
			}
		} else {
			if len(actions) != 1 {
				t.Errorf("%s: expected no update action: %#v", name, actions)
			}
			if !actions[0].Matches("update", "imagestreams") {
				t.Errorf("%s: expected %s, got %v", name, "update-imagestreams", actions[0])
			}
		}
	}
}

func TestControllerReturnsErrForRetries(t *testing.T) {
	expErr := fmt.Errorf("expected error")
	osClient := &client.Fake{}
	errISMClient := &client.Fake{}
	errISMClient.PrependReactor("create", "imagestreammappings", func(action kclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, ret, expErr
	})
	tests := map[string]struct {
		singleError bool
		expActions  int
		fakeClient  *client.Fake
		fakeDocker  *fakeDockerRegistryClient
		stream      *api.ImageStream
	}{
		"retry-able error no. 1": {
			singleError: true,
			expActions:  0,
			fakeClient:  osClient,
			fakeDocker: &fakeDockerRegistryClient{
				ConnErr: expErr,
			},
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "foo/bar",
				},
			},
		},
		"retry-able error no. 2": {
			singleError: true,
			expActions:  0,
			fakeClient:  osClient,
			fakeDocker: &fakeDockerRegistryClient{
				Err: expErr,
			},
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "foo/bar",
				},
			},
		},
		"retry-able error no. 3": {
			singleError: false,
			expActions:  0,
			fakeClient:  osClient,
			fakeDocker: &fakeDockerRegistryClient{
				ConnErr: expErr,
			},
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						api.DefaultImageTag: {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "foo/bar",
							},
						},
					},
				},
			},
		},
		"retry-able error no. 4": {
			singleError: false,
			expActions:  0,
			fakeClient:  osClient,
			fakeDocker: &fakeDockerRegistryClient{
				Images: []expectedImage{
					{
						Tag: api.DefaultImageTag,
						Image: &dockerregistry.Image{
							Image: docker.Image{
								Comment: "foo",
								Config:  &docker.Config{},
							},
						},
						Err: expErr,
					},
				},
			},
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						api.DefaultImageTag: {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "foo/bar",
							},
						},
					},
				},
			},
		},
		"retry-able error no. 5": {
			singleError: false,
			expActions:  1,
			fakeClient:  errISMClient,
			fakeDocker: &fakeDockerRegistryClient{
				Images: []expectedImage{
					{
						Tag: api.DefaultImageTag,
						Image: &dockerregistry.Image{
							Image: docker.Image{
								Comment: "foo",
								Config:  &docker.Config{},
							},
						},
					},
				},
			},
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						api.DefaultImageTag: {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "foo/bar",
							},
						},
					},
				},
			},
		},
	}

	for name, test := range tests {
		c := ImportController{client: test.fakeDocker, streams: test.fakeClient, mappings: test.fakeClient}

		err := c.Next(test.stream)
		if err == nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
		// The first condition checks error from the getTags method only,
		// iow. where the error returned is the exact error that happened.
		// The second condition checks error from the importTags method only,
		// iow. where the error is an aggregate.
		if test.singleError && err != expErr {
			t.Errorf("%s: unexpected error from getTags: %v", name, err)
		} else if !test.singleError && !strings.Contains(err.Error(), expErr.Error()) {
			t.Errorf("%s: unexpected error from importTags: %v", name, err)
		}
		if len(test.fakeClient.Actions()) != test.expActions {
			t.Errorf("%s: expected no actions: %#v", name, test.fakeClient.Actions())
		}
	}
}

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
