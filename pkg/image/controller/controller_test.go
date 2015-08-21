package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"

	kapi "k8s.io/kubernetes/pkg/api"

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

	Tags map[string]string
	Err  error

	Images []expectedImage
}

func (f *fakeDockerRegistryClient) Connect(registry string, insecure bool) (dockerregistry.Connection, error) {
	f.Registry = registry
	f.Insecure = insecure
	return f, nil
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

func TestControllerNoDockerRepo(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, streams: fake, mappings: fake}

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
	}
	other := stream
	if err := c.Next(&stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !kapi.Semantic.DeepEqual(stream, other) {
		t.Errorf("did not expect change to stream")
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
	if len(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 1 {
		t.Errorf("expected an update action: %#v", fake.Actions)
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
	if len(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) != 0 {
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
				"openshift.io/image.insecureRepository": "true",
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
	if len(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) != 0 {
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
	if len(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) == 0 {
		t.Errorf("did not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 1 {
		t.Errorf("expected an update action: %#v", fake.Actions())
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
	if err := c.Next(&stream); err != cli.Images[0].Err {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) != 0 {
		t.Errorf("did not expect annotation: %#v", stream)
	}
	if len(fake.Actions()) != 0 {
		t.Errorf("expected no update action: %#v", fake.Actions())
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
	if !isRFC3339(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) {
		t.Fatalf("did not set annotation: %#v", stream)
	}
	if len(fake.Actions()) != 2 {
		t.Errorf("expected an update action: %#v", fake.Actions())
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
				Name: "some/repo",
			},
			expectUpdate: false,
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
		if !isRFC3339(stream.Annotations["openshift.io/image.dockerRepositoryCheck"]) {
			t.Fatalf("%s: did not set annotation: %#v", name, stream)
		}
		if test.expectUpdate {
			if len(fake.Actions()) != 2 {
				t.Errorf("%s: expected an update action: %#v", name, fake.Actions())
			}
			if !fake.Actions()[0].Matches("create", "imagestreammappings") {
				t.Errorf("%s: expected %s, got %v", name, "create-imagestreammappings", fake.Actions()[0])
			}
			if !fake.Actions()[1].Matches("update", "imagestreams") {
				t.Errorf("%s: expected %s, got %v", name, "update-imagestreams", fake.Actions()[1])
			}
		} else {
			if len(fake.Actions()) != 1 {
				t.Errorf("%s: expected no update action: %#v", name, fake.Actions())
			}
			if !fake.Actions()[0].Matches("update", "imagestreams") {
				t.Errorf("%s: expected %s, got %v", name, "update-imagestreams", fake.Actions()[0])
			}
		}
	}
}

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
