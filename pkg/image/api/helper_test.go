package api

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/diff"
)

func TestParseImageStreamImageName(t *testing.T) {
	tests := map[string]struct {
		input        string
		expectedRepo string
		expectedId   string
		expectError  bool
	}{
		"empty string": {
			input:       "",
			expectError: true,
		},
		"one part": {
			input:       "a",
			expectError: true,
		},
		"more than 2 parts": {
			input:       "a@b@c",
			expectError: true,
		},
		"empty name part": {
			input:       "@id",
			expectError: true,
		},
		"empty id part": {
			input:       "name@",
			expectError: true,
		},
		"valid input": {
			input:        "repo@id",
			expectedRepo: "repo",
			expectedId:   "id",
			expectError:  false,
		},
	}

	for name, test := range tests {
		repo, id, err := ParseImageStreamImageName(test.input)
		didError := err != nil
		if e, a := test.expectError, didError; e != a {
			t.Errorf("%s: expected error=%t, got=%t: %s", name, e, a, err)
			continue
		}
		if test.expectError {
			continue
		}
		if e, a := test.expectedRepo, repo; e != a {
			t.Errorf("%s: repo: expected %q, got %q", name, e, a)
			continue
		}
		if e, a := test.expectedId, id; e != a {
			t.Errorf("%s: id: expected %q, got %q", name, e, a)
			continue
		}
	}
}

func TestParseImageStreamTagName(t *testing.T) {
	tests := map[string]struct {
		id           string
		expectedName string
		expectedTag  string
		expectError  bool
	}{
		"empty id": {
			id:          "",
			expectError: true,
		},
		"missing semicolon": {
			id:          "hello",
			expectError: true,
		},
		"too many semicolons": {
			id:          "a:b:c",
			expectError: true,
		},
		"empty name": {
			id:          ":tag",
			expectError: true,
		},
		"empty tag": {
			id:          "name",
			expectError: true,
		},
		"happy path": {
			id:           "name:tag",
			expectError:  false,
			expectedName: "name",
			expectedTag:  "tag",
		},
	}

	for description, testCase := range tests {
		name, tag, err := ParseImageStreamTagName(testCase.id)
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expected err: %t, got: %t: %s", description, e, a, err)
		}
		if err != nil {
			continue
		}
		if e, a := testCase.expectedName, name; e != a {
			t.Errorf("%s: name: expected %q, got %q", description, e, a)
		}
		if e, a := testCase.expectedTag, tag; e != a {
			t.Errorf("%s: tag: expected %q, got %q", description, e, a)
		}
	}
}

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

func validImageWithManifestData() Image {
	return Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "id",
		},
		DockerImageManifest: `{
   "name": "library/ubuntu",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      },
      {
         "blobSum": "tarsum.dev+sha256:b194de3772ebbcdc8f244f663669799ac1cb141834b7cb8b69100285d357a2b0"
      },
      {
         "blobSum": "tarsum.dev+sha256:c937c4bb1c1a21cc6d94340812262c6472092028972ae69b551b1a70d4276171"
      },
      {
         "blobSum": "tarsum.dev+sha256:2aaacc362ac6be2b9e9ae8c6029f6f616bb50aec63746521858e47841b90fabd"
      },
      {
         "blobSum": "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"2d24f826cb16146e2016ff349a8a33ed5830f3b938d45c0f82943f4ab8c097e7\",\"parent\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"created\":\"2015-02-21T02:11:06.735146646Z\",\"container\":\"c9a3eda5951d28aa8dbe5933be94c523790721e4f80886d0a8e7a710132a38ec\",\"container_config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [/bin/bash]\"],\"Image\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"architecture\":\"amd64\",\"os\":\"linux\",\"checksum\":\"tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\",\"Size\":0}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"parent\":\"1c8294cc516082dfbb731f062806b76b82679ce38864dd87635f08869c993e45\",\"created\":\"2015-02-21T02:11:02.473113442Z\",\"container\":\"b4d4c42c196081cdb8e032446e9db92c0ce8ddeeeb1ef4e582b0275ad62b9af2\",\"container_config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"sed -i 's/^#\\\\s*\\\\(deb.*universe\\\\)$/\\\\1/g' /etc/apt/sources.list\"],\"Image\":\"1c8294cc516082dfbb731f062806b76b82679ce38864dd87635f08869c993e45\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"1c8294cc516082dfbb731f062806b76b82679ce38864dd87635f08869c993e45\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"architecture\":\"amd64\",\"os\":\"linux\",\"checksum\":\"tarsum.dev+sha256:b194de3772ebbcdc8f244f663669799ac1cb141834b7cb8b69100285d357a2b0\",\"Size\":1895}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"1c8294cc516082dfbb731f062806b76b82679ce38864dd87635f08869c993e45\",\"parent\":\"fa4fd76b09ce9b87bfdc96515f9a5dd5121c01cc996cf5379050d8e13d4a864b\",\"created\":\"2015-02-21T02:10:56.648624065Z\",\"container\":\"d6323d4f92bf13dd2a7d2957e8970c8dc8ba3c2df08ffebdf4a04b4b658c83fb\",\"container_config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"echo '#!/bin/sh' \\u003e /usr/sbin/policy-rc.d \\u0009\\u0026\\u0026 echo 'exit 101' \\u003e\\u003e /usr/sbin/policy-rc.d \\u0009\\u0026\\u0026 chmod +x /usr/sbin/policy-rc.d \\u0009\\u0009\\u0026\\u0026 dpkg-divert --local --rename --add /sbin/initctl \\u0009\\u0026\\u0026 cp -a /usr/sbin/policy-rc.d /sbin/initctl \\u0009\\u0026\\u0026 sed -i 's/^exit.*/exit 0/' /sbin/initctl \\u0009\\u0009\\u0026\\u0026 echo 'force-unsafe-io' \\u003e /etc/dpkg/dpkg.cfg.d/docker-apt-speedup \\u0009\\u0009\\u0026\\u0026 echo 'DPkg::Post-Invoke { \\\"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\\\"; };' \\u003e /etc/apt/apt.conf.d/docker-clean \\u0009\\u0026\\u0026 echo 'APT::Update::Post-Invoke { \\\"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\\\"; };' \\u003e\\u003e /etc/apt/apt.conf.d/docker-clean \\u0009\\u0026\\u0026 echo 'Dir::Cache::pkgcache \\\"\\\"; Dir::Cache::srcpkgcache \\\"\\\";' \\u003e\\u003e /etc/apt/apt.conf.d/docker-clean \\u0009\\u0009\\u0026\\u0026 echo 'Acquire::Languages \\\"none\\\";' \\u003e /etc/apt/apt.conf.d/docker-no-languages \\u0009\\u0009\\u0026\\u0026 echo 'Acquire::GzipIndexes \\\"true\\\"; Acquire::CompressionTypes::Order:: \\\"gz\\\";' \\u003e /etc/apt/apt.conf.d/docker-gzip-indexes\"],\"Image\":\"fa4fd76b09ce9b87bfdc96515f9a5dd5121c01cc996cf5379050d8e13d4a864b\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"fa4fd76b09ce9b87bfdc96515f9a5dd5121c01cc996cf5379050d8e13d4a864b\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"architecture\":\"amd64\",\"os\":\"linux\",\"checksum\":\"tarsum.dev+sha256:c937c4bb1c1a21cc6d94340812262c6472092028972ae69b551b1a70d4276171\",\"Size\":194533}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"fa4fd76b09ce9b87bfdc96515f9a5dd5121c01cc996cf5379050d8e13d4a864b\",\"parent\":\"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158\",\"created\":\"2015-02-21T02:10:48.627646094Z\",\"container\":\"43bd710ec89aa1d61685c5ddb8737b315f2152d04c9e127ea2a9cf79f950fa5d\",\"container_config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:0018ff77d038472f52512fd66ae2466e6325e5d91289e10a6b324f40582298df in /\"],\"Image\":\"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"architecture\":\"amd64\",\"os\":\"linux\",\"checksum\":\"tarsum.dev+sha256:2aaacc362ac6be2b9e9ae8c6029f6f616bb50aec63746521858e47841b90fabd\",\"Size\":188097705}\n"
      },
      {
         "v1Compatibility": "{\"id\":\"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158\",\"comment\":\"Imported from -\",\"created\":\"2013-06-13T14:03:50.821769-07:00\",\"container_config\":{\"Hostname\":\"\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null},\"docker_version\":\"0.4.0\",\"architecture\":\"x86_64\",\"checksum\":\"tarsum.dev+sha256:324d4cf44ee7daa46266c1df830c61a7df615c0632176a339e7310e34723d67a\",\"Size\":0}\n"
      }
   ],
   "schemaVersion": 1,
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "OIH7:HQFS:44FK:45VB:3B53:OIAG:TPL4:ATF5:6PNE:MGHN:NHQX:2GE4",
               "kty": "EC",
               "x": "Cu_UyxwLgHzE9rvlYSmvVdqYCXY42E9eNhBb0xNv0SQ",
               "y": "zUsjWJkeKQ5tv7S-hl1Tg71cd-CqnrtiiLxSi6N_yc8"
            },
            "alg": "ES256"
         },
         "signature": "JafVC022gLJbK8fEp9UrW-GWAi53nD4Xf0-fmb766nV6zGTq7g9RutSWHcpv3OqhV8t5b-4j-bXhvnGBjXA7Yg",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjEwMDc2LCJmb3JtYXRUYWlsIjoiQ24wIiwidGltZSI6IjIwMTUtMDMtMDdUMTI6Mzk6MjJaIn0"
      }
   ]
}`,
	}
}

func validImageWithManifestV2Data() Image {
	return Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "id",
		},
		DockerImageConfig: `{
    "architecture": "amd64",
    "config": {
        "AttachStderr": false,
        "AttachStdin": false,
        "AttachStdout": false,
        "Cmd": [
            "/bin/sh",
            "-c",
            "echo hi"
        ],
        "Domainname": "",
        "Entrypoint": null,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "derived=true",
            "asdf=true"
        ],
        "Hostname": "23304fc829f9",
        "Image": "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
        "Labels": {},
        "OnBuild": [],
        "OpenStdin": false,
        "StdinOnce": false,
        "Tty": false,
        "User": "",
        "Volumes": null,
        "WorkingDir": ""
    },
    "container": "e91032eb0403a61bfe085ff5a5a48e3659e5a6deae9f4d678daa2ae399d5a001",
    "container_config": {
        "AttachStderr": false,
        "AttachStdin": false,
        "AttachStdout": false,
        "Cmd": [
            "/bin/sh",
            "-c",
            "#(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]"
        ],
        "Domainname": "",
        "Entrypoint": null,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "derived=true",
            "asdf=true"
        ],
        "Hostname": "23304fc829f9",
        "Image": "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
        "Labels": {},
        "OnBuild": [],
        "OpenStdin": false,
        "StdinOnce": false,
        "Tty": false,
        "User": "",
        "Volumes": null,
        "WorkingDir": ""
    },
    "created": "2015-02-21T02:11:06.735146646Z",
    "docker_version": "1.9.0-dev",
    "history": [
        {
            "created": "2015-10-31T22:22:54.690851953Z",
            "created_by": "/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /"
        },
        {
            "created": "2015-10-31T22:22:55.613815829Z",
            "created_by": "/bin/sh -c #(nop) CMD [\"sh\"]"
        },
        {
            "created": "2015-11-04T23:06:30.934316144Z",
            "created_by": "/bin/sh -c #(nop) ENV derived=true",
            "empty_layer": true
        },
        {
            "created": "2015-11-04T23:06:31.192097572Z",
            "created_by": "/bin/sh -c #(nop) ENV asdf=true",
            "empty_layer": true
        },
        {
            "created": "2015-11-04T23:06:32.083868454Z",
            "created_by": "/bin/sh -c dd if=/dev/zero of=/file bs=1024 count=1024"
        },
        {
            "created": "2015-11-04T23:06:32.365666163Z",
            "created_by": "/bin/sh -c #(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]",
            "empty_layer": true
        }
    ],
    "os": "linux",
    "rootfs": {
        "diff_ids": [
            "sha256:c6f988f4874bb0add23a778f753c65efe992244e148a1d2ec2a8b664fb66bbd1",
            "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
            "sha256:13f53e08df5a220ab6d13c58b2bf83a59cbdc2e04d0a3f041ddf4b0ba4112d49"
        ],
        "type": "layers"
    }
}`,
		DockerImageManifest: `{
    "schemaVersion": 2,
    "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "config": {
        "mediaType": "application/vnd.docker.container.image.v1+json",
        "size": 7023,
        "digest": "sha256:815d06b56f4138afacd0009b8e3799fcdce79f0507bf8d0588e219b93ab6fd4d"
    },
    "layers": [
        {
            "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
            "size": 5312,
            "digest": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
        },
        {
            "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
            "size": 235231,
            "digest": "sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa"
        },
        {
            "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
            "size": 235231,
            "digest": "sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa"
        },
        {
            "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
            "size": 639152,
            "digest": "sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
        }
    ]
}`,
	}
}

func TestImageWithMetadata(t *testing.T) {
	tests := map[string]struct {
		image         Image
		expectedImage Image
		expectError   bool
	}{
		"no manifest data": {
			image:         Image{},
			expectedImage: Image{},
		},
		"error unmarshalling manifest data": {
			image: Image{
				DockerImageManifest: "{ no {{{ json here!!!",
			},
			expectedImage: Image{},
			expectError:   true,
		},
		"no history": {
			image: Image{
				DockerImageManifest: `{"name": "library/ubuntu", "tag": "latest"}`,
			},
			expectedImage: Image{
				DockerImageManifest: `{"name": "library/ubuntu", "tag": "latest"}`,
			},
		},
		"error unmarshalling v1 compat": {
			image: Image{
				DockerImageManifest: "{\"name\": \"library/ubuntu\", \"tag\": \"latest\", \"history\": [\"v1Compatibility\": \"{ not valid {{ json\" }",
			},
			expectError: true,
		},
		"happy path": {
			image: validImageWithManifestData(),
			expectedImage: Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "id",
				},
				DockerImageManifest: validImageWithManifestData().DockerImageManifest,
				DockerImageLayers: []ImageLayer{
					{Name: "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", MediaType: "application/vnd.docker.container.image.rootfs.diff+x-gtar", LayerSize: 0},
					{Name: "tarsum.dev+sha256:2aaacc362ac6be2b9e9ae8c6029f6f616bb50aec63746521858e47841b90fabd", MediaType: "application/vnd.docker.container.image.rootfs.diff+x-gtar", LayerSize: 188097705},
					{Name: "tarsum.dev+sha256:c937c4bb1c1a21cc6d94340812262c6472092028972ae69b551b1a70d4276171", MediaType: "application/vnd.docker.container.image.rootfs.diff+x-gtar", LayerSize: 194533},
					{Name: "tarsum.dev+sha256:b194de3772ebbcdc8f244f663669799ac1cb141834b7cb8b69100285d357a2b0", MediaType: "application/vnd.docker.container.image.rootfs.diff+x-gtar", LayerSize: 1895},
					{Name: "tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", MediaType: "application/vnd.docker.container.image.rootfs.diff+x-gtar", LayerSize: 0},
				},
				DockerImageManifestMediaType: "application/vnd.docker.distribution.manifest.v1+json",
				DockerImageMetadata: DockerImage{
					ID:        "2d24f826cb16146e2016ff349a8a33ed5830f3b938d45c0f82943f4ab8c097e7",
					Parent:    "117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c",
					Comment:   "",
					Created:   unversioned.Date(2015, 2, 21, 2, 11, 6, 735146646, time.UTC),
					Container: "c9a3eda5951d28aa8dbe5933be94c523790721e4f80886d0a8e7a710132a38ec",
					ContainerConfig: DockerConfig{
						Hostname:        "43bd710ec89a",
						Domainname:      "",
						User:            "",
						Memory:          0,
						MemorySwap:      0,
						CPUShares:       0,
						CPUSet:          "",
						AttachStdin:     false,
						AttachStdout:    false,
						AttachStderr:    false,
						PortSpecs:       nil,
						ExposedPorts:    nil,
						Tty:             false,
						OpenStdin:       false,
						StdinOnce:       false,
						Env:             []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
						Cmd:             []string{"/bin/sh", "-c", "#(nop) CMD [/bin/bash]"},
						Image:           "117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c",
						Volumes:         nil,
						WorkingDir:      "",
						Entrypoint:      nil,
						NetworkDisabled: false,
						SecurityOpts:    nil,
						OnBuild:         []string{},
					},
					DockerVersion: "1.4.1",
					Author:        "",
					Config: &DockerConfig{
						Hostname:        "43bd710ec89a",
						Domainname:      "",
						User:            "",
						Memory:          0,
						MemorySwap:      0,
						CPUShares:       0,
						CPUSet:          "",
						AttachStdin:     false,
						AttachStdout:    false,
						AttachStderr:    false,
						PortSpecs:       nil,
						ExposedPorts:    nil,
						Tty:             false,
						OpenStdin:       false,
						StdinOnce:       false,
						Env:             []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
						Cmd:             []string{"/bin/bash"},
						Image:           "117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c",
						Volumes:         nil,
						WorkingDir:      "",
						Entrypoint:      nil,
						NetworkDisabled: false,
						OnBuild:         []string{},
					},
					Architecture: "amd64",
					Size:         188294133,
				},
			},
		},
		"valid metadata size": {
			image: validImageWithManifestV2Data(),
			expectedImage: Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "id",
				},
				DockerImageConfig:            validImageWithManifestV2Data().DockerImageConfig,
				DockerImageManifest:          validImageWithManifestV2Data().DockerImageManifest,
				DockerImageManifestMediaType: "application/vnd.docker.distribution.manifest.v2+json",
				DockerImageLayers: []ImageLayer{
					{Name: "sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", LayerSize: 639152},
					{Name: "sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", LayerSize: 235231},
					{Name: "sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", LayerSize: 235231},
					{Name: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip", LayerSize: 5312},
				},
				DockerImageMetadata: DockerImage{
					ID:            "sha256:815d06b56f4138afacd0009b8e3799fcdce79f0507bf8d0588e219b93ab6fd4d",
					Parent:        "",
					Comment:       "",
					Created:       unversioned.Date(2015, 2, 21, 2, 11, 6, 735146646, time.UTC),
					Container:     "e91032eb0403a61bfe085ff5a5a48e3659e5a6deae9f4d678daa2ae399d5a001",
					DockerVersion: "1.9.0-dev",
					Author:        "",
					Architecture:  "amd64",
					Size:          882848,
					ContainerConfig: DockerConfig{
						Hostname:        "23304fc829f9",
						Domainname:      "",
						User:            "",
						Memory:          0,
						MemorySwap:      0,
						CPUShares:       0,
						CPUSet:          "",
						AttachStdin:     false,
						AttachStdout:    false,
						AttachStderr:    false,
						PortSpecs:       nil,
						ExposedPorts:    nil,
						Tty:             false,
						OpenStdin:       false,
						StdinOnce:       false,
						Env:             []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "derived=true", "asdf=true"},
						Cmd:             []string{"/bin/sh", "-c", "#(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]"},
						Image:           "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
						Volumes:         nil,
						WorkingDir:      "",
						Entrypoint:      nil,
						NetworkDisabled: false,
						SecurityOpts:    nil,
						OnBuild:         []string{},
					},
					Config: &DockerConfig{
						Hostname:        "23304fc829f9",
						Domainname:      "",
						User:            "",
						Memory:          0,
						MemorySwap:      0,
						CPUShares:       0,
						CPUSet:          "",
						AttachStdin:     false,
						AttachStdout:    false,
						AttachStderr:    false,
						PortSpecs:       nil,
						ExposedPorts:    nil,
						Tty:             false,
						OpenStdin:       false,
						StdinOnce:       false,
						Env:             []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "derived=true", "asdf=true"},
						Cmd:             []string{"/bin/sh", "-c", "echo hi"},
						Image:           "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
						Volumes:         nil,
						WorkingDir:      "",
						Entrypoint:      nil,
						NetworkDisabled: false,
						OnBuild:         []string{},
					},
				},
			},
		},
	}

	for name, test := range tests {
		imageWithMetadata := test.image
		err := ImageWithMetadata(&imageWithMetadata)
		gotError := err != nil
		if e, a := test.expectError, gotError; e != a {
			t.Fatalf("%s: expectError=%t, gotError=%t: %s", name, e, a, err)
		}
		if test.expectError {
			continue
		}
		if e, a := test.expectedImage, imageWithMetadata; !kapi.Semantic.DeepEqual(e, a) {
			t.Errorf("%s: image: %s", name, diff.ObjectDiff(e, a))
		}
	}
}

func TestLatestTaggedImage(t *testing.T) {
	tests := []struct {
		tag            string
		tags           map[string]TagEventList
		expected       string
		expectNotFound bool
	}{
		{
			tag:            "foo",
			tags:           map[string]TagEventList{},
			expectNotFound: true,
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expectNotFound: true,
		},
		{
			tag: "",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
				"foo": {
					Items: []TagEvent{
						{DockerImageReference: "foo-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "foo-ref",
		},
	}

	for i, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags

		actual := LatestTaggedImage(stream, test.tag)
		if actual == nil {
			if !test.expectNotFound {
				t.Errorf("%d: unexpected nil result", i)
			}
			continue
		}
		if e, a := test.expected, actual.DockerImageReference; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestAddTagEventToImageStream(t *testing.T) {
	tests := map[string]struct {
		tags           map[string]TagEventList
		nextRef        string
		nextImage      string
		expectedTags   map[string]TagEventList
		expectedUpdate bool
	}{
		"nil entry for tag": {
			tags:      map[string]TagEventList{},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"empty items for tag": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{},
				},
			},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"same ref and image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: false,
		},
		"same ref, different image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "ref",
			nextImage: "newimage",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "newimage",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"different ref, same image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "newref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "newref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"different ref, different image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "newref",
			nextImage: "newimage",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "newref",
							Image:                "newimage",
						},
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
	}

	for name, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags
		updated := AddTagEventToImageStream(stream, "latest", TagEvent{DockerImageReference: test.nextRef, Image: test.nextImage})
		if e, a := test.expectedUpdate, updated; e != a {
			t.Errorf("%s: expected updated=%t, got %t", name, e, a)
		}
		if e, a := test.expectedTags, stream.Status.Tags; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected\ntags=%#v\ngot=%#v", name, e, a)
		}
	}
}

func TestUpdateTrackingTags(t *testing.T) {
	tests := map[string]struct {
		fromNil               bool
		fromKind              string
		fromNamespace         string
		fromName              string
		trackingTags          []string
		nonTrackingTags       []string
		statusTags            []string
		updatedImageReference string
		updatedImage          string
		expectedUpdates       []string
	}{
		"nil from": {
			fromNil: true,
		},
		"from kind not ImageStreamTag": {
			fromKind: "ImageStreamImage",
		},
		"from namespace different": {
			fromNamespace: "other",
		},
		"from name different": {
			trackingTags: []string{"otherstream:2.0"},
		},
		"no tracking": {
			trackingTags: []string{},
			statusTags:   []string{"2.0", "3.0"},
		},
		"stream name in from name": {
			trackingTags:    []string{"latest"},
			fromName:        "ruby:2.0",
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest"},
		},
		"1 tracking, 1 not": {
			trackingTags:    []string{"latest"},
			nonTrackingTags: []string{"other"},
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest"},
		},
		"multiple tracking, multiple not": {
			trackingTags:    []string{"latest1", "latest2"},
			nonTrackingTags: []string{"other1", "other2"},
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest1", "latest2"},
		},
		"no change to tracked tag": {
			trackingTags:          []string{"latest"},
			statusTags:            []string{"2.0", "3.0"},
			updatedImageReference: "ns/ruby@id",
			updatedImage:          "id",
		},
	}

	for name, test := range tests {
		stream := &ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "ns",
				Name:      "ruby",
			},
			Spec: ImageStreamSpec{
				Tags: map[string]TagReference{},
			},
			Status: ImageStreamStatus{
				Tags: map[string]TagEventList{},
			},
		}

		if len(test.fromNamespace) > 0 {
			stream.Namespace = test.fromNamespace
		}

		fromName := test.fromName
		if len(fromName) == 0 {
			fromName = "2.0"
		}

		for _, tag := range test.trackingTags {
			stream.Spec.Tags[tag] = TagReference{
				From: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: fromName,
				},
			}
		}

		for _, tag := range test.nonTrackingTags {
			stream.Spec.Tags[tag] = TagReference{}
		}

		for _, tag := range test.statusTags {
			stream.Status.Tags[tag] = TagEventList{
				Items: []TagEvent{
					{
						DockerImageReference: "ns/ruby@id",
						Image:                "id",
					},
				},
			}
		}

		if test.fromNil {
			stream.Spec.Tags = map[string]TagReference{
				"latest": {},
			}
		}

		if len(test.fromKind) > 0 {
			stream.Spec.Tags = map[string]TagReference{
				"latest": {
					From: &kapi.ObjectReference{
						Kind: test.fromKind,
						Name: "asdf",
					},
				},
			}
		}

		updatedImageReference := test.updatedImageReference
		if len(updatedImageReference) == 0 {
			updatedImageReference = "ns/ruby@newid"
		}

		updatedImage := test.updatedImage
		if len(updatedImage) == 0 {
			updatedImage = "newid"
		}

		newTagEvent := TagEvent{
			DockerImageReference: updatedImageReference,
			Image:                updatedImage,
		}

		UpdateTrackingTags(stream, "2.0", newTagEvent)
		for _, tag := range test.expectedUpdates {
			tagEventList, ok := stream.Status.Tags[tag]
			if !ok {
				t.Errorf("%s: expected update for tag %q", name, tag)
				continue
			}
			if e, a := updatedImageReference, tagEventList.Items[0].DockerImageReference; e != a {
				t.Errorf("%s: dockerImageReference: expected %q, got %q", name, e, a)
			}
			if e, a := updatedImage, tagEventList.Items[0].Image; e != a {
				t.Errorf("%s: image: expected %q, got %q", name, e, a)
			}
		}
	}
}

func TestJoinImageStreamTag(t *testing.T) {
	if e, a := "foo:bar", JoinImageStreamTag("foo", "bar"); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
	if e, a := "foo:"+DefaultImageTag, JoinImageStreamTag("foo", ""); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
}

func TestResolveImageID(t *testing.T) {
	tests := map[string]struct {
		tags     map[string]TagEventList
		imageID  string
		expErr   string
		expEvent TagEvent
	}{
		"single tag, match ID prefix": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
							Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
						},
					},
				},
			},
			imageID: "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:  "",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
				Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			},
		},
		"single tag, match string prefix": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo:mytag",
							Image:                "mytag",
						},
					},
				},
			},
			imageID: "mytag",
			expErr:  "",
			expEvent: TagEvent{
				DockerImageReference: "repo:mytag",
				Image:                "mytag",
			},
		},
		"single tag, ID error": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b2",
							Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b2",
						},
					},
				},
			},
			imageID:  "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:   "not found",
			expEvent: TagEvent{},
		},
		"no tag": {
			tags:     map[string]TagEventList{},
			imageID:  "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:   "not found",
			expEvent: TagEvent{},
		},
		"multiple match": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@mytag",
							Image:                "mytag",
						},
						{
							DockerImageReference: "repo@mytag",
							Image:                "mytag2",
						},
					},
				},
			},
			imageID:  "mytag",
			expErr:   "multiple images match the prefix",
			expEvent: TagEvent{},
		},
		"find match out of multiple tags in first position": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000002",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000002",
						},
					},
				},
				"tag2": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000003",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000003",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
						},
					},
				},
			},
			imageID: "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
				Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		"find match out of multiple tags in last position": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000002",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000002",
						},
					},
				},
				"tag2": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000003",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000003",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
						},
					},
				},
			},
			imageID: "sha256:0000000000000000000000000000000000000000000000000000000000000004",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
				Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
			},
		},
	}

	for name, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags
		event, err := ResolveImageID(stream, test.imageID)
		if len(test.expErr) > 0 {
			if err == nil || !strings.Contains(err.Error(), test.expErr) {
				t.Errorf("%s: unexpected error, expected %v, got %v", name, test.expErr, err)
			}
			continue
		} else if err != nil {
			t.Errorf("%s: unexpected error, got %v", name, err)
			continue
		}
		if test.expEvent.Image != event.Image || test.expEvent.DockerImageReference != event.DockerImageReference {
			t.Errorf("%s: unexpected tag, expected %#v, got %#v", name, test.expEvent, event)
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
				Tag:       DefaultImageTag,
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
				Tag:       DefaultImageTag,
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

func TestPrioritizeTags(t *testing.T) {
	tags := []string{"5", "other", "latest", "v5.5", "v6", "5.2.3", "v5.3.6-bother", "5.3.6-abba", "5.6"}
	PrioritizeTags(tags)
	if !reflect.DeepEqual(tags, []string{"latest", "v6", "5", "5.6", "v5.5", "v5.3.6-bother", "5.3.6-abba", "5.2.3", "other"}) {
		t.Errorf("unexpected order: %v", tags)
	}
}

func TestTagsChanged(t *testing.T) {
	tests := map[string]struct {
		new     []TagEvent
		old     []TagEvent
		changed bool
		deleted bool
	}{
		"both empty": {
			new:     []TagEvent{},
			old:     []TagEvent{},
			changed: false,
			deleted: false,
		},
		"new image": {
			new:     []TagEvent{{Image: "newimage"}},
			old:     []TagEvent{},
			changed: true,
			deleted: false,
		},
		"image deleted": {
			new:     []TagEvent{},
			old:     []TagEvent{{Image: "oldimage"}},
			changed: true,
			deleted: true,
		},
		"image changed": {
			new:     []TagEvent{{Image: "newimage"}},
			old:     []TagEvent{{Image: "oldImage"}},
			changed: true,
			deleted: false,
		},
	}
	for name, test := range tests {
		changed, deleted := tagsChanged(test.new, test.old)
		if changed != test.changed || deleted != test.deleted {
			t.Errorf("%s: unexpected tagsChanged, expected (%v, %v) got (%v, %v)",
				name, test.changed, test.deleted, changed, deleted)
		}
	}
}

func TestIndexOfImageSignature(t *testing.T) {
	for _, tc := range []struct {
		name          string
		signatures    []ImageSignature
		matchType     string
		matchContent  []byte
		expectedIndex int
	}{
		{
			name:          "empty",
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "not present",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "first and only",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("binary"),
			expectedIndex: 0,
		},

		{
			name: "last",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 2,
		},

		{
			name: "many matches",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob2"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 1,
		},
	} {

		im := Image{
			Signatures: make([]ImageSignature, len(tc.signatures)),
		}
		for i, signature := range tc.signatures {
			signature.Name = fmt.Sprintf("%s:%s", signature.Type, signature.Content)
			im.Signatures[i] = signature
		}

		matchName := fmt.Sprintf("%s:%s", tc.matchType, tc.matchContent)

		index := IndexOfImageSignatureByName(im.Signatures, matchName)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}

		index = IndexOfImageSignature(im.Signatures, tc.matchType, tc.matchContent)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}
	}
}
