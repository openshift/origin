package image

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseNamedDockerImageReference(t *testing.T) {
	testCases := []struct {
		From                               string
		Registry, Namespace, Name, Tag, ID string
		Err                                bool
	}{
		{
			From: "foo",
			Name: "foo",
		},
		{
			From: "foo:tag",
			Name: "foo",
			Tag:  "tag",
		},
		{
			From: "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Name: "sha256",
			Tag:  "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From: "foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Name: "foo",
			ID:   "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From:      "bar/foo",
			Namespace: "bar",
			Name:      "foo",
		},
		{
			From:      "bar/foo:tag",
			Namespace: "bar",
			Name:      "foo",
			Tag:       "tag",
		},
		{
			From:      "bar/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From:      "bar/foo/baz",
			Namespace: "bar",
			Name:      "foo/baz",
		},
		{
			From:      "bar/library/baz",
			Namespace: "bar",
			Name:      "library/baz",
		},
		{
			From:      "bar/foo/baz:tag",
			Namespace: "bar",
			Name:      "foo/baz",
			Tag:       "tag",
		},
		{
			From:      "bar/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Namespace: "bar",
			Name:      "foo/baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From:      "bar:5000/foo/baz",
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
		},
		{
			From:      "bar:5000/library/baz",
			Registry:  "bar:5000",
			Namespace: "library",
			Name:      "baz",
		},
		{
			From:     "bar:5000/baz",
			Registry: "bar:5000",
			Name:     "baz",
		},
		{
			From:      "bar:5000/foo/baz:tag",
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
		},
		{
			From:      "bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From:     "myregistry.io/foo",
			Registry: "myregistry.io",
			Name:     "foo",
		},
		{
			From:     "localhost/bar",
			Registry: "localhost",
			Name:     "bar",
		},
		{
			From:      "docker.io/library/myapp",
			Registry:  "docker.io",
			Namespace: "library",
			Name:      "myapp",
		},
		{
			From:     "docker.io/myapp",
			Registry: "docker.io",
			Name:     "myapp",
		},
		{
			From:      "docker.io/user/myapp",
			Registry:  "docker.io",
			Namespace: "user",
			Name:      "myapp",
		},
		{
			From:      "docker.io/user/project/myapp",
			Registry:  "docker.io",
			Namespace: "user",
			Name:      "project/myapp",
		},
		{
			From:     "index.docker.io/bar",
			Registry: "index.docker.io",
			Name:     "bar",
		},
		{
			// registry/namespace/name == 255 chars
			From:      fmt.Sprintf("bar:5000/%s/%s:tag", strings.Repeat("a", 63), strings.Repeat("b", 182)),
			Registry:  "bar:5000",
			Namespace: strings.Repeat("a", 63),
			Name:      strings.Repeat("b", 182),
			Tag:       "tag",
		},
		{
			// docker.io/namespace/name == 255 chars with explicit namespace
			From:      fmt.Sprintf("docker.io/library/%s:tag", strings.Repeat("b", 231)),
			Registry:  "docker.io",
			Namespace: "library",
			Name:      strings.Repeat("b", 231),
			Tag:       "tag",
		},
		{
			// docker.io/namespace/name == 255 chars with implicit namespace
			From:     fmt.Sprintf("docker.io/%s:tag", strings.Repeat("b", 231)),
			Registry: "docker.io",
			Name:     strings.Repeat("b", 231),
			Tag:      "tag",
		},
		{
			// registry/namespace/name > 255 chars
			From: fmt.Sprintf("bar:5000/%s/%s:tag", strings.Repeat("a", 63), strings.Repeat("b", 183)),
			Err:  true,
		},
		{
			// docker.io/name > 255 chars with implicit namespace
			From: fmt.Sprintf("docker.io/%s:tag", strings.Repeat("b", 246)),
			Err:  true,
		},
		{
			From: "registry.io/foo/bar/:Tag",
			Err:  true,
		},
		{
			From: "https://bar:5000/foo/baz",
			Err:  true,
		},
		{
			From: "http://bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Err:  true,
		},
		{
			From: "registry:3000/integration/imageStream:success",
			Err:  true,
		},
		{
			From: "registry:5000/integration/test-image-stream@sha256:00000000000000000000000000000001",
			Err:  true,
		},
		{
			From: "abc@badid",
			Err:  true,
		},
		{
			From: "index.docker.io/mysql@sha256:bad",
			Err:  true,
		},
		{
			From: "@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Err:  true,
		},
		{
			From: ":tag",
			Err:  true,
		},
		{
			From:      "bar/foo/baz/biz",
			Namespace: "bar",
			Name:      "foo/baz/biz",
		},
		{
			From: "bar/foo/baz////biz",
			Err:  true,
		},
		{
			From: "//foo/baz/biz",
			Err:  true,
		},
		{
			From: "ftp://baz/baz/biz",
			Err:  true,
		},
		{
			From: "",
			Err:  true,
		},
	}

	for _, testCase := range testCases {
		ref, err := parseNamedDockerImageReference(testCase.From)
		switch {
		case err != nil && !testCase.Err:
			t.Errorf("%s: unexpected error: %v", testCase.From, err)
			continue
		case err == nil && testCase.Err:
			t.Errorf("%s: unexpected non-error: %#+v", testCase.From, ref)
			continue
		case err != nil && testCase.Err:
			continue
		}
		if e, a := testCase.Registry, ref.Registry; e != a {
			t.Errorf("%s: registry: expected %q, got %q", testCase.From, e, a)
		}
		if e, a := testCase.Namespace, ref.Namespace; e != a {
			t.Errorf("%s: namespace: expected %q, got %q", testCase.From, e, a)
		}
		if e, a := testCase.Name, ref.Name; e != a {
			t.Errorf("%s: name: expected %q, got %q", testCase.From, e, a)
		}
		if e, a := testCase.Tag, ref.Tag; e != a {
			t.Errorf("%s: tag: expected %q, got %q", testCase.From, e, a)
		}
		if e, a := testCase.ID, ref.ID; e != a {
			t.Errorf("%s: id: expected %q, got %q", testCase.From, e, a)
		}
	}
}
