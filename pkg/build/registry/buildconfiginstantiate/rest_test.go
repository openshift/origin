package buildconfiginstantiate

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserverrest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/generator"
	mocks "github.com/openshift/origin/pkg/build/generator/test"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
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
			GetBuildConfigFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
			},
			UpdateBuildConfigFunc: func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
				return nil
			},
			CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
				return nil
			},
			GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
				return &buildapi.Build{}, nil
			},
			GetImageStreamFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return imageStream, nil
			},
			GetImageStreamTagFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{Image: *image}, nil
			},
			GetImageStreamImageFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{Image: *image}, nil
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
