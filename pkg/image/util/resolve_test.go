package util

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func image(pullSpec string) *imageapi.Image {
	return &imageapi.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"},
		DockerImageReference: pullSpec,
	}
}

func isimage(name, pullSpec string) *imageapi.ImageStreamImage {
	i := image(pullSpec)
	return &imageapi.ImageStreamImage{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: "default"},
		Image:      *i,
	}
}

func istag(name, pullSpec string) *imageapi.ImageStreamTag {
	i := image(pullSpec)
	return &imageapi.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: "default"},
		Image:      *i,
	}
}

func TestResolveImagePullSpec(t *testing.T) {
	testCases := []struct {
		client    *testclient.Fake
		source    string
		input     string
		expect    string
		expectErr bool
	}{
		{
			client: testclient.NewSimpleFake(isimage("test@sha256:foo", "registry.url/image/test:latest")),
			source: "isimage",
			input:  "test@sha256:foo",
			expect: "registry.url/image/test:latest",
		},
		{
			client: testclient.NewSimpleFake(istag("test:1.1", "registry.url/image/test:latest")),
			source: "istag",
			input:  "test:1.1",
			expect: "registry.url/image/test:latest",
		},
		{
			client: testclient.NewSimpleFake(istag("test:1.1", "registry.url/image/test:latest")),
			source: "istag",
			input:  "user/test:1.1",
			expect: "registry.url/image/test:latest",
		},
		{
			client: testclient.NewSimpleFake(),
			source: "docker",
			input:  "test:latest",
			expect: "test:latest",
		},
		{
			client:    testclient.NewSimpleFake(),
			source:    "istag",
			input:     "test:1.2",
			expectErr: true,
		},
		{
			client:    testclient.NewSimpleFake(),
			source:    "istag",
			input:     "test:1.2",
			expectErr: true,
		},
		{
			client:    testclient.NewSimpleFake(),
			source:    "unknown",
			input:     "",
			expectErr: true,
		},
	}

	for i, test := range testCases {
		t.Logf("[%d] trying to resolve %q %s and expecting %q (expectErr=%t)", i, test.source, test.input, test.expect, test.expectErr)
		result, err := ResolveImagePullSpec(test.client, test.client, test.source, test.input, "default")
		if err != nil && !test.expectErr {
			t.Errorf("[%d] unexpected error: %v", i, err)
		} else if err == nil && test.expectErr {
			t.Errorf("[%d] expected error but got none and result %q", i, result)
		}
		if test.expect != result {
			t.Errorf("[%d] expected %q, but got %q", i, test.expect, result)
		}
	}
}
