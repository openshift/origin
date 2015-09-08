package test

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

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

func MockBuilderSecrets() (secrets []*kapi.Secret) {
	i := 1
	for name, conf := range SampleDockerConfigs {
		secrets = append(secrets, &kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
			},
			Type: kapi.SecretTypeDockercfg,
			Data: map[string][]byte{".dockercfg": conf},
		})
		i++
	}
	return secrets
}

func MockBuilderServiceAccount(secrets []*kapi.Secret) kclient.ServiceAccountsNamespacer {
	var (
		secretRefs  []kapi.ObjectReference
		fakeObjects []runtime.Object
	)
	for _, secret := range secrets {
		secretRefs = append(secretRefs, kapi.ObjectReference{Name: secret.Name, Kind: "Secret"})
		fakeObjects = append(fakeObjects, secret)
	}
	fakeObjects = append(fakeObjects, &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.BuilderServiceAccountName},
		Secrets:    secretRefs,
	})
	return testclient.NewSimpleFake(fakeObjects...)
}

func MockBuildConfig(source buildapi.BuildSource, strategy buildapi.BuildStrategy, output buildapi.BuildOutput) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
}

func MockSource() buildapi.BuildSource {
	return buildapi.BuildSource{
		Type: buildapi.BuildSourceGit,
		Git: &buildapi.GitBuildSource{
			URI: "http://test.repository/namespace/name",
			Ref: "test-tag",
		},
	}
}

func MockSourceStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		Type: buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func MockOutput() buildapi.BuildOutput {
	return buildapi.BuildOutput{
		To: &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: "http://localhost:5000/test/image-tag",
		},
	}
}

func MockImageStream(repoName, dockerImageRepo string, tags map[string]string) *imageapi.ImageStream {
	tagHistory := make(map[string]imageapi.TagEventList)
	for tag, imageID := range tags {
		tagHistory[tag] = imageapi.TagEventList{
			Items: []imageapi.TagEvent{
				{
					Image:                imageID,
					DockerImageReference: fmt.Sprintf("%s:%s", dockerImageRepo, imageID),
				},
			},
		}
	}

	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: repoName,
		},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: dockerImageRepo,
			Tags: tagHistory,
		},
	}
}

func MockImage(name, dockerSpec string) *imageapi.Image {
	return &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		DockerImageReference: dockerSpec,
	}
}
