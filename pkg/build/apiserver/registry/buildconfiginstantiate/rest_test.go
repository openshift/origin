package buildconfiginstantiate

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserverrest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/generator"
	mocks "github.com/openshift/origin/pkg/build/generator/test"
)

func TestCreateInstantiate(t *testing.T) {
	imageStream := mocks.MockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"test": "newImageID123"})
	image := mocks.MockImage("testImage@id", "registry.com/namespace/imagename@id")
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	rest := InstantiateREST{&generator.BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: generator.TestingClient{
			GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
			},
			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				return nil
			},
			CreateBuildFunc: func(ctx context.Context, build *buildv1.Build) error {
				return nil
			},
			GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
				return &buildv1.Build{}, nil
			},
			GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return imageStream, nil
			},
			GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
				return &imagev1.ImageStreamTag{Image: *image}, nil
			},
			GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
				return &imagev1.ImageStreamImage{Image: *image}, nil
			},
		}}}

	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: metav1.ObjectMeta{Name: "name"}}, apiserverrest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateInstantiateValidationError(t *testing.T) {
	rest := InstantiateREST{&generator.BuildGenerator{}}
	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{}, apiserverrest.ValidateAllObjectFunc, false)
	if err == nil {
		t.Error("Expected object got none!")
	}
}
