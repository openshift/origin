package api

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

func TestParseDockerImageReference(t *testing.T) {
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
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
		},
		{
			From:      "bar/foo/baz:tag",
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
		},
		{
			From:      "bar/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			From:      "bar:5000/foo/baz",
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
		},
		{
			From:      "bar:5000/baz",
			Registry:  "bar:5000",
			Namespace: "library",
			Name:      "baz",
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
		// TODO: test cases if ParseDockerImageReference validates segment length and allowed chars
		//
		// {
		// 	// namespace/name == 255 chars
		// 	From:      fmt.Sprintf("bar:5000/%s/%s:tag", strings.Repeat("a", 63), strings.Repeat("b", 191)),
		// 	Registry:  "bar:5000",
		// 	Namespace: strings.Repeat("a", 63),
		// 	Name:      strings.Repeat("b", 191),
		// 	Tag:       "tag",
		// },
		// {
		// 	// namespace/name == 255 chars with implicit namespace
		// 	From:      fmt.Sprintf("bar:5000/%s:tag", strings.Repeat("b", 247)),
		// 	Registry:  "bar:5000",
		// 	Namespace: "library",
		// 	Name:      strings.Repeat("b", 247),
		// 	Tag:       "tag",
		// },
		// {
		// 	// namespace/name > 255 chars
		// 	From: fmt.Sprintf("bar:5000/%s/%s:tag", strings.Repeat("a", 63), strings.Repeat("b", 192)),
		// 	Err:  true,
		// },
		// {
		// 	// namespace/name > 255 chars with implicit namespace
		// 	From: fmt.Sprintf("bar:5000/%s:tag", strings.Repeat("b", 248)),
		// 	Err:  true,
		// },
		// {
		// 	// namespace < 2 chars
		// 	From: "bar:5000/a/b:tag",
		// 	Err:  true,
		// },
		{
			From: "https://bar:5000/foo/baz",
			Err:  true,
		},
		{
			From: "http://bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Err:  true,
		},
		{
			From: "bar/foo/baz/biz",
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
		ref, err := ParseDockerImageReference(testCase.From)
		switch {
		case err != nil && !testCase.Err:
			t.Errorf("%s: unexpected error: %v", testCase.From, err)
			continue
		case err == nil && testCase.Err:
			t.Errorf("%s: unexpected non-error", testCase.From)
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

func TestDockerImageReferenceString(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Name:     "foo",
			Expected: "library/foo",
		},
		{
			Name:     "foo",
			Tag:      "tag",
			Expected: "library/foo:tag",
		},
		{
			Name:     "foo",
			ID:       "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "library/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Name:     "foo",
			ID:       "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "library/foo:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
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
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
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
			expectedImage: Image{},
		},
		"error unmarshalling v1 compat": {
			image: Image{
				DockerImageManifest: `{"name": "library/ubuntu", "tag": "latest", "history": ["v1Compatibility": "{ not valid {{ json" }`,
			},
			expectError: true,
		},
		"happy path": {
			image: validImageWithManifestData(),
			expectedImage: Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "id",
				},
				DockerImageManifest: "",
				DockerImageMetadata: DockerImage{
					ID:        "2d24f826cb16146e2016ff349a8a33ed5830f3b938d45c0f82943f4ab8c097e7",
					Parent:    "117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c",
					Comment:   "",
					Created:   util.Date(2015, 2, 21, 2, 11, 6, 735146646, time.UTC),
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
					Size:         0,
				},
			},
		},
	}

	for name, test := range tests {
		imageWithMetadata, err := ImageWithMetadata(test.image)
		gotError := err != nil
		if e, a := test.expectError, gotError; e != a {
			t.Fatalf("%s: expectError=%t, gotError=%t: %s", name, e, a, err)
		}
		if test.expectError {
			continue
		}
		if e, a := test.expectedImage, *imageWithMetadata; !kapi.Semantic.DeepEqual(e, a) {
			stringE := fmt.Sprintf("%#v", e)
			stringA := fmt.Sprintf("%#v", a)
			t.Errorf("%s: image: %s", name, util.StringDiff(stringE, stringA))
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
			t.Errorf("%s: expected tags=%v, got %v", name, e, a)
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

func TestNameAndTag(t *testing.T) {
	if e, a := "foo:bar", NameAndTag("foo", "bar"); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
	if e, a := "foo:"+DefaultImageTag, NameAndTag("foo", ""); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
}
