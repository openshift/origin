package image

import (
	"testing"
)

func TestDockerImageReferenceAsRepository(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Namespace: "bar",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "bar/foo",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.AsRepository().String()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}

}

func TestDockerImageReferenceDaemonMinimal(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Namespace: "library",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "library/foo:tag",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
		{
			Registry:  "localhost:5000",
			Namespace: "library",
			Name:      "bar",
			Tag:       "latest",
			Expected:  "localhost:5000/library/bar",
		},
		{
			Registry:  "index.docker.io",
			Namespace: "foo",
			Name:      "bar",
			Tag:       "latest",
			Expected:  "docker.io/foo/bar",
		},
		{
			Registry:  "registry-1.docker.io",
			Namespace: "library",
			Name:      "foo",
			Tag:       "bar",
			Expected:  "docker.io/foo:bar",
		},
		{
			Registry:  "docker.io",
			Namespace: "foo",
			Name:      "library",
			Expected:  "docker.io/foo/library",
		},
		{
			Registry: "registry-1.docker.io",
			Name:     "library",
			Tag:      "foo",
			Expected: "docker.io/library:foo",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.DaemonMinimal().Exact()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestDockerImageReferenceString(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Name:     "foo",
			Expected: "foo",
		},
		{
			Name:     "foo",
			Tag:      "tag",
			Expected: "foo:tag",
		},
		{
			Name:     "foo",
			ID:       "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Name:     "foo",
			ID:       "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "foo:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			Expected:  "bar/foo",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "bar/foo:tag",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar/foo/baz:tag",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar:5000/foo/baz",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar:5000/foo/baz:tag",
		},
		{
			Registry:  "bar:5000",
			Namespace: "library",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar:5000/library/baz:tag",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "docker.io",
			Namespace: "user",
			Name:      "app",
			Expected:  "docker.io/user/app",
		},
		{
			Registry: "index.docker.io",
			Name:     "foo",
			Expected: "index.docker.io/library/foo",
		},
		{
			Registry:  "index.docker.io",
			Namespace: "library",
			Name:      "bar",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "index.docker.io/library/bar@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.String()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestDockerImageReferenceEquality(t *testing.T) {
	equalityTests := []struct {
		a, b    DockerImageReference
		isEqual bool
	}{
		{
			a:       DockerImageReference{},
			b:       DockerImageReference{},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Name: "openshift",
			},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Name: "openshift3",
			},
			isEqual: false,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       "latest",
			},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       "v1.0",
			},
			isEqual: false,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       "latest",
				ID:        "d0a28ab59a",
			},
			isEqual: false,
		},
	}
	for i, test := range equalityTests {
		if isEqual := test.a.Equal(test.b); isEqual != test.isEqual {
			t.Errorf("test %d: %#v.Equal(%#v) = %t; want %t",
				i, test.a, test.b, isEqual, test.isEqual)
		}
		// commutativeness sanity check
		if x, y := test.a.Equal(test.b), test.b.Equal(test.a); x != y {
			t.Errorf("test %[1]d: %[2]q.Equal(%[3]q) = %[4]t != %[3]q.Equal(%[2]q) = %[5]t",
				i, test.a, test.b, x, y)
		}
	}
}
