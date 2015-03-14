package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type expectedImage struct {
	Tag   string
	ID    string
	Image *docker.Image
	Err   error
}

type fakeDockerRegistryClient struct {
	Registry                 string
	Namespace, Name, Tag, ID string

	Tags map[string]string
	Err  error

	Images []expectedImage
}

func (f *fakeDockerRegistryClient) Connect(registry string) (dockerregistry.Connection, error) {
	f.Registry = registry
	return f, nil
}

func (f *fakeDockerRegistryClient) ImageTags(namespace, name string) (map[string]string, error) {
	f.Namespace, f.Name = namespace, name
	return f.Tags, f.Err
}

func (f *fakeDockerRegistryClient) ImageByTag(namespace, name, tag string) (*docker.Image, error) {
	if len(tag) == 0 {
		tag = "latest"
	}
	f.Namespace, f.Name, f.Tag = namespace, name, tag
	for _, t := range f.Images {
		if t.Tag == tag {
			return t.Image, t.Err
		}
	}
	return nil, dockerregistry.NewImageNotFoundError(fmt.Sprintf("%s/%s", namespace, name), tag, tag)
}

func (f *fakeDockerRegistryClient) ImageByID(namespace, name, id string) (*docker.Image, error) {
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
	c := ImportController{client: cli, repositories: fake, mappings: fake}

	repo := api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
	}
	other := repo
	if err := c.Next(&repo); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !kapi.Semantic.DeepEqual(repo, other) {
		t.Errorf("did not expect change to repo")
	}
}

func TestControllerRepoHandled(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}

	repo := api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "other",
		},
		DockerImageRepository: "foo/bar",
	}
	if err := c.Next(&repo); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) == 0 {
		t.Errorf("did not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 1 {
		t.Error("expected an update action: %#v", fake.Actions)
	}
}

func TestControllerTagRetrievalFails(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{Err: fmt.Errorf("test error")}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}

	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
	}
	if err := c.Next(&repo); err != cli.Err {
		t.Errorf("unexpected error: %v", err)
	}
	if len(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) != 0 {
		t.Errorf("should not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 0 {
		t.Error("expected no actions on fake client")
	}
}

func TestControllerRepoTagsAlreadySet(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}

	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
		Tags: map[string]string{
			"test": "",
		},
	}
	if err := c.Next(&repo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) == 0 {
		t.Errorf("did not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 1 {
		t.Error("expected an update action: %#v", fake.Actions)
	}
}

func TestControllerImageNotFoundError(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{Tags: map[string]string{"latest": "not_found"}}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}
	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
	}
	if err := c.Next(&repo); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) == 0 {
		t.Errorf("did not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 1 {
		t.Error("expected an update action: %#v", fake.Actions)
	}
}

func TestControllerImageWithGenericError(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{"latest": "found"},
		Images: []expectedImage{
			{
				ID:  "found",
				Err: fmt.Errorf("test error"),
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}
	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
	}
	if err := c.Next(&repo); err != cli.Images[0].Err {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) != 0 {
		t.Errorf("did not expect annotation: %#v", repo)
	}
	if len(fake.Actions) != 0 {
		t.Error("expected no update action: %#v", fake.Actions)
	}
}

func TestControllerWithImage(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{"latest": "found"},
		Images: []expectedImage{
			{
				ID: "found",
				Image: &docker.Image{
					Comment: "foo",
					Config:  &docker.Config{},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}
	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
	}
	if err := c.Next(&repo); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !isRFC3339(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) {
		t.Fatalf("did not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 2 {
		t.Error("expected an update action: %#v", fake.Actions)
	}
}

func TestControllerWithEmptyTag(t *testing.T) {
	cli, fake := &fakeDockerRegistryClient{
		Tags: map[string]string{"latest": "found"},
		Images: []expectedImage{
			{
				ID: "found",
				Image: &docker.Image{
					Comment: "foo",
					Config:  &docker.Config{},
				},
			},
		},
	}, &client.Fake{}
	c := ImportController{client: cli, repositories: fake, mappings: fake}
	repo := api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test", Namespace: "other"},
		DockerImageRepository: "foo/bar",
		Tags: map[string]string{
			"latest": "",
		},
	}
	if err := c.Next(&repo); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !isRFC3339(repo.Annotations["openshift.io/image.dockerRepositoryCheck"]) {
		t.Fatalf("did not set annotation: %#v", repo)
	}
	if len(fake.Actions) != 2 {
		t.Error("expected an update action: %#v", fake.Actions)
	}
}

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
