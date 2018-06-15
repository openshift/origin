package app

import (
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	apptest "github.com/openshift/origin/pkg/oc/generate/app/test"
)

type fakeRegistrySearcher struct {
	matches ComponentMatches
	errs    []error
}

func (f fakeRegistrySearcher) Type() string {
	return ""
}

func (f fakeRegistrySearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	return f.matches, f.errs
}

func TestDockerImageLookup(t *testing.T) {

	images := []docker.APIImages{
		{
			ID:       "testid",
			RepoTags: []string{"test/myimage:testtag"},
		},
	}

	image := &docker.Image{
		ID: "testid",
	}

	matches := ComponentMatches{
		&ComponentMatch{
			Value: "matchvalue",
			Score: 0.0,
		},
	}

	// found in remote registry, local docker client defined
	searcher := DockerClientSearcher{
		Client: &apptest.FakeDockerClient{
			Images: images,
			Image:  image,
		},
		RegistrySearcher: fakeRegistrySearcher{
			matches: matches,
		},
	}

	results, _ := searcher.Search(true, "testid")
	if len(results) != 1 {
		t.Fatalf("Expected a single result, got %d %#v", len(results), results)
	}
	if results[0].Value != matches[0].Value {
		t.Fatalf("Unexpected result %#v", *results[0])
	}

	// found in remote registry, local docker client undefined
	searcher = DockerClientSearcher{
		Client: nil,
		RegistrySearcher: fakeRegistrySearcher{
			matches: matches,
		},
	}

	results, _ = searcher.Search(true, "testid")
	if len(results) != 1 {
		t.Fatalf("Expected a single result, got %d %#v", len(results), results)
	}
	if results[0].Value != matches[0].Value {
		t.Fatalf("Unexpected result %#v", *results[0])
	}

	// found in local docker client
	searcher = DockerClientSearcher{
		Client: &apptest.FakeDockerClient{
			Images: images,
			Image:  image,
		},
		RegistrySearcher: fakeRegistrySearcher{
			matches: ComponentMatches{},
		},
	}

	results, _ = searcher.Search(true, "test/myimage:testtag")
	if len(results) != 1 {
		t.Fatalf("Expected a single result, got %d %#v", len(results), results)
	}
	if results[0].Value != "test/myimage:testtag" {
		t.Fatalf("Unexpected result %#v", *results[0])
	}
}
