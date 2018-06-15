package buildclone

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserverrest "k8s.io/apiserver/pkg/registry/rest"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/generator"
)

func TestCreateClone(t *testing.T) {
	rest := CloneREST{&generator.BuildGenerator{Client: generator.TestingClient{
		CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}}

	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: metav1.ObjectMeta{Name: "name"}}, apiserverrest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateCloneValidationError(t *testing.T) {
	rest := CloneREST{&generator.BuildGenerator{}}
	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{}, apiserverrest.ValidateAllObjectFunc, false)
	if err == nil {
		t.Error("Expected object got none!")
	}
}
