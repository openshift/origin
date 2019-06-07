package buildconfiginstantiate

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserverrest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
	"github.com/openshift/origin/pkg/build/apiserver/buildgenerator"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestCreateInstantiate(t *testing.T) {
	imageStream := MockImageStream("testImageStream", "registry.com/namespace/imagename", map[string]string{"test": "newImageID123"})
	image := MockImage("testImage@id", "registry.com/namespace/imagename@id")
	fakeSecrets := []runtime.Object{}
	for _, s := range MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	rest := InstantiateREST{&buildgenerator.BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).CoreV1(),
		ServiceAccounts: MockBuilderServiceAccount(MockBuilderSecrets()),
		Client: buildgenerator.TestingClient{
			GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return MockBuildConfig(MockSource(), MockSourceStrategyForImageRepository(), MockOutput()), nil
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

	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: metav1.ObjectMeta{Name: "name"}}, apiserverrest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateInstantiateValidationError(t *testing.T) {
	rest := InstantiateREST{&buildgenerator.BuildGenerator{}}
	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{}, apiserverrest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}

const (
	tagName = "test"

	imageRepoNamespace = "testns"
	imageRepoName      = "testRepo"
)

var (
	Encode = func(src string) []byte {
		return []byte(src)
	}

	SampleDockerConfigs = map[string][]byte{
		"hub":  Encode(`{"https://index.docker.io/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
		"ipv4": Encode(`{"https://1.1.1.1:5000/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
		"host": Encode(`{"https://registry.host/v1/":{"auth": "Zm9vOmJhcgo=", "email": ""}}`),
	}
)

func MockBuilderSecrets() []*corev1.Secret {
	var secrets []*corev1.Secret
	for name, conf := range SampleDockerConfigs {
		secrets = append(secrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Type: corev1.SecretTypeDockercfg,
			Data: map[string][]byte{".dockercfg": conf},
		})
	}
	return secrets
}

func MockBuilderServiceAccount(secrets []*corev1.Secret) corev1client.ServiceAccountsGetter {
	var (
		secretRefs  []corev1.ObjectReference
		fakeObjects []runtime.Object
	)
	for _, secret := range secrets {
		secretRefs = append(secretRefs, corev1.ObjectReference{
			Name: secret.Name,
			Kind: "Secret",
		})
		fakeObjects = append(fakeObjects, secret)
	}
	fakeObjects = append(fakeObjects, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrappolicy.BuilderServiceAccountName,
			Namespace: metav1.NamespaceDefault,
		},
		Secrets: secretRefs,
	})
	return fake.NewSimpleClientset(fakeObjects...).CoreV1()
}

func MockBuildConfig(source buildv1.BuildSource, strategy buildv1.BuildStrategy, output buildv1.BuildOutput) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"testbclabel": "testbcvalue",
			},
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
}

func MockSource() buildv1.BuildSource {
	return buildv1.BuildSource{
		Git: &buildv1.GitBuildSource{
			URI: "http://test.repository/namespace/name",
			Ref: "test-tag",
		},
	}
}

func MockSourceStrategyForImageRepository() buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		SourceStrategy: &buildv1.SourceBuildStrategy{
			From: corev1.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func MockOutput() buildv1.BuildOutput {
	return buildv1.BuildOutput{
		To: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: "localhost:5000/test/image-tag",
		},
	}
}

func MockImageStream(repoName, dockerImageRepo string, tags map[string]string) *imagev1.ImageStream {
	tagHistory := []imagev1.NamedTagEventList{}
	for tag, imageID := range tags {
		tagHistory = append(tagHistory, imagev1.NamedTagEventList{
			Tag: tag,
			Items: []imagev1.TagEvent{
				{
					Image:                imageID,
					DockerImageReference: fmt.Sprintf("%s:%s", dockerImageRepo, imageID),
				},
			},
		})
	}

	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
		Status: imagev1.ImageStreamStatus{
			DockerImageRepository: dockerImageRepo,
			Tags:                  tagHistory,
		},
	}
}

func MockImage(name, dockerSpec string) *imagev1.Image {
	return &imagev1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		DockerImageReference: dockerSpec,
	}
}
