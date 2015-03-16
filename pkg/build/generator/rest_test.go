package generator

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestCreateClone(t *testing.T) {
	rest := CloneREST{&BuildGenerator{Client: Client{
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}}

	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: kapi.ObjectMeta{Name: "name"}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateCloneObjectError(t *testing.T) {
	rest := CloneREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.Build{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

func TestCreateCloneValidationError(t *testing.T) {
	rest := CloneREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

func TestCreateInstantiate(t *testing.T) {
	rest := InstantiateREST{&BuildGenerator{Client: Client{
		GetBuildConfigFunc: func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
			return mockBuildConfig(mockSource(), mockSTIStrategyForImage(), mockOutput()), nil
		},
		UpdateBuildConfigFunc: func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		},
		CreateBuildFunc: func(ctx kapi.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx kapi.Context, name string) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}}

	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: kapi.ObjectMeta{Name: "name"}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateInstantiateObjectError(t *testing.T) {
	rest := InstantiateREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.Build{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

func TestCreateInstantiateValidationError(t *testing.T) {
	rest := InstantiateREST{&BuildGenerator{}}
	_, err := rest.Create(kapi.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}
