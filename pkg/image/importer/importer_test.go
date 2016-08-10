package importer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/docker/distribution/manifest/schema1"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/image/api"
)

func TestImportNothing(t *testing.T) {
	ctx := NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(NoCredentials)
	isi := &api.ImageStreamImport{}
	i := NewImageStreamImporter(ctx, 5, nil)
	if err := i.Import(nil, isi); err != nil {
		t.Fatal(err)
	}
}

func expectStatusError(status unversioned.Status, message string) bool {
	if status.Status != unversioned.StatusFailure || status.Message != message {
		return false
	}
	return true
}

func TestImport(t *testing.T) {
	m := &schema1.SignedManifest{}
	if err := json.Unmarshal([]byte(etcdManifest), m); err != nil {
		t.Fatal(err)
	}
	insecureRetriever := &mockRetriever{
		repo: &mockRepository{
			getTagErr:   fmt.Errorf("no such tag"),
			getByTagErr: fmt.Errorf("no such manifest tag"),
			getErr:      fmt.Errorf("no such digest"),
		},
	}
	testCases := []struct {
		retriever RepositoryRetriever
		isi       api.ImageStreamImport
		expect    func(*api.ImageStreamImport, *testing.T)
	}{
		{
			retriever: insecureRetriever,
			isi: api.ImageStreamImport{
				Spec: api.ImageStreamImportSpec{
					Images: []api.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"}, ImportPolicy: api.TagImportPolicy{Insecure: true}},
					},
				},
			},
			expect: func(isi *api.ImageStreamImport, t *testing.T) {
				if !insecureRetriever.insecure {
					t.Errorf("expected retriever to beset insecure: %#v", insecureRetriever)
				}
			},
		},
		{
			retriever: &mockRetriever{
				repo: &mockRepository{
					getTagErr:   fmt.Errorf("no such tag"),
					getByTagErr: fmt.Errorf("no such manifest tag"),
					getErr:      fmt.Errorf("no such digest"),
				},
			},
			isi: api.ImageStreamImport{
				Spec: api.ImageStreamImportSpec{
					Images: []api.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test/un/parse/able/image"}},
						{From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"}},
					},
				},
			},
			expect: func(isi *api.ImageStreamImport, t *testing.T) {
				if !expectStatusError(isi.Status.Images[0].Status, "Internal error occurred: no such manifest tag") {
					t.Errorf("unexpected status: %#v", isi.Status.Images[0].Status)
				}
				if !expectStatusError(isi.Status.Images[1].Status, "Internal error occurred: no such digest") {
					t.Errorf("unexpected status: %#v", isi.Status.Images[1].Status)
				}
				if !expectStatusError(isi.Status.Images[2].Status, " \"\" is invalid: from.name: Invalid value: \"test/un/parse/able/image\": invalid name: the docker pull spec \"test/un/parse/able/image\" must be two or three segments separated by slashes") {
					t.Errorf("unexpected status: %#v", isi.Status.Images[2].Status)
				}
				// non DockerImage refs are no-ops
				if status := isi.Status.Images[3].Status; status.Status != "" {
					t.Errorf("unexpected status: %#v", isi.Status.Images[3].Status)
				}
				expectedTags := []string{"latest", "", "", ""}
				for i, image := range isi.Status.Images {
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
		{
			retriever: &mockRetriever{err: fmt.Errorf("error")},
			isi: api.ImageStreamImport{
				Spec: api.ImageStreamImportSpec{
					Repository: &api.RepositoryImportSpec{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"},
					},
				},
			},
			expect: func(isi *api.ImageStreamImport, t *testing.T) {
				if !reflect.DeepEqual(isi.Status.Repository.AdditionalTags, []string(nil)) {
					t.Errorf("unexpected additional tags: %#v", isi.Status.Repository)
				}
				if len(isi.Status.Repository.Images) != 0 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				if isi.Status.Repository.Status.Status != unversioned.StatusFailure || isi.Status.Repository.Status.Message != "Internal error occurred: error" {
					t.Errorf("unexpected status: %#v", isi.Status.Repository.Status)
				}
			},
		},
		{
			retriever: &mockRetriever{repo: &mockRepository{manifest: m}},
			isi: api.ImageStreamImport{
				Spec: api.ImageStreamImportSpec{
					Images: []api.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test@sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test:tag"}},
					},
				},
			},
			expect: func(isi *api.ImageStreamImport, t *testing.T) {
				if len(isi.Status.Images) != 2 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				expectedTags := []string{"", "tag"}
				for i, image := range isi.Status.Images {
					if image.Status.Status != unversioned.StatusSuccess {
						t.Errorf("unexpected status %d: %#v", i, image.Status)
					}
					// the image name is always the sha256, and size is calculated
					if image.Image == nil || image.Image.Name != "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238" || image.Image.DockerImageMetadata.Size != 28643712 {
						t.Errorf("unexpected image %d: %#v", i, image.Image.Name)
					}
					// the most specific reference is returned
					if image.Image.DockerImageReference != "test@sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238" {
						t.Errorf("unexpected ref %d: %#v", i, image.Image.DockerImageReference)
					}
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
		{
			retriever: &mockRetriever{
				repo: &mockRepository{
					manifest: m,
					tags: map[string]string{
						"v1":    "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"other": "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"v2":    "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"3":     "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"3.1":   "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"abc":   "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
					},
					getTagErr:   fmt.Errorf("no such tag"),
					getByTagErr: fmt.Errorf("no such manifest tag"),
				},
			},
			isi: api.ImageStreamImport{
				Spec: api.ImageStreamImportSpec{
					Repository: &api.RepositoryImportSpec{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"},
					},
				},
			},
			expect: func(isi *api.ImageStreamImport, t *testing.T) {
				if !reflect.DeepEqual(isi.Status.Repository.AdditionalTags, []string{"other"}) {
					t.Errorf("unexpected additional tags: %#v", isi.Status.Repository)
				}
				if len(isi.Status.Repository.Images) != 5 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				expectedTags := []string{"3", "v2", "v1", "3.1", "abc"}
				for i, image := range isi.Status.Repository.Images {
					if image.Status.Status != unversioned.StatusFailure || image.Status.Message != "Internal error occurred: no such manifest tag" {
						t.Errorf("unexpected status %d: %#v", i, isi.Status.Repository.Images)
					}
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
	}
	for i, test := range testCases {
		im := NewImageStreamImporter(test.retriever, 5, nil)
		if err := im.Import(nil, &test.isi); err != nil {
			t.Errorf("%d: %v", i, err)
		}
		if test.expect != nil {
			test.expect(&test.isi, t)
		}
	}
}

const etcdManifest = `
{
   "schemaVersion": 1, 
   "tag": "latest", 
   "name": "coreos/etcd", 
   "architecture": "amd64", 
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }, 
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }, 
      {
         "blobSum": "sha256:2560187847cadddef806eaf244b7755af247a9dbabb90ca953dd2703cf423766"
      }, 
      {
         "blobSum": "sha256:744b46d0ac8636c45870a03830d8d82c20b75fbfb9bc937d5e61005d23ad4cfe"
      }
   ], 
   "history": [
      {
         "v1Compatibility": "{\"id\":\"fe50ac14986497fa6b5d2cc24feb4a561d01767bc64413752c0988cb70b0b8b9\",\"parent\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"created\":\"2015-12-30T22:29:13.967754365Z\",\"container\":\"c8d0f1a274b5f52fa5beb280775ef07cf18ec0f95e5ae42fbad01157e2614d42\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT \\u0026{[\\\"/etcd\\\"]}\"],\"Image\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":[\"/etcd\"],\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":[\"/etcd\"],\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"parent\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"created\":\"2015-12-30T22:29:13.504159783Z\",\"container\":\"080708d544f85052a46fab72e701b4358c1b96cb4b805a5b2d66276fc2aaf85d\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) EXPOSE 2379/tcp 2380/tcp 4001/tcp 7001/tcp\"],\"Image\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"parent\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"created\":\"2015-12-30T22:29:12.912813629Z\",\"container\":\"f28be899c9b8680d4cf8585e663ad20b35019db062526844e7cfef117ce9037f\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:e330b1da49d993059975e46560b3bd360691498b0f2f6e00f39fc160cf8d4ec3 in /\"],\"Image\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":13502144}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"created\":\"2015-12-30T22:29:12.346834862Z\",\"container\":\"1b97abade59e4b5b935aede236980a54fb500cd9ee5bd4323c832c6d7b3ffc6e\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:74912593c6783292c4520514f5cc9313acbd1da0f46edee0fdbed2a24a264d6f in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":15141568}"
      }
   ], 
   "signatures": [
      {
         "header": {
            "alg": "RS256", 
            "jwk": {
               "e": "AQAB", 
               "kty": "RSA", 
               "n": "yB40ou1GMvIxYs1jhxWaeoDiw3oa0_Q2UJThUPtArvO0tRzaun9FnSphhOEHIGcezfq95jy-3MN-FIjmsWgbPHY8lVDS38fF75aCw6qkholwqjmMtUIgPNYoMrg0rLUE5RRyJ84-hKf9Fk7V3fItp1mvCTGKaS3ze-y5dTTrfbNGE7qG638Dla2Fuz-9CNgRQj0JH54o547WkKJC-pG-j0jTDr8lzsXhrZC7lJas4yc-vpt3D60iG4cW_mkdtIj52ZFEgHZ56sUj7AhnNVly0ZP9W1hmw4xEHDn9WLjlt7ivwARVeb2qzsNdguUitcI5hUQNwpOVZ_O3f1rUIL_kRw"
            }
         }, 
         "protected": "eyJmb3JtYXRUYWlsIjogIkNuMCIsICJmb3JtYXRMZW5ndGgiOiA1OTI2LCAidGltZSI6ICIyMDE2LTAxLTAyVDAyOjAxOjMzWiJ9", 
         "signature": "DrQ43UWeit-thDoRGTCP0Gd2wL5K2ecyPhHo_au0FoXwuKODja0tfwHexB9ypvFWngk-ijXuwO02x3aRIZqkWpvKLxxzxwkrZnPSje4o_VrFU4z5zwmN8sJw52ODkQlW38PURIVksOxCrb0zRl87yTAAsUAJ_4UUPNltZSLnhwy-qPb2NQ8ghgsONcBxRQrhPFiWNkxDKZ3kjvzYyrXDxTcvwK3Kk_YagZ4rCOhH1B7mAdVSiSHIvvNV5grPshw_ipAoqL2iNMsxWxLjYZl9xSJQI2asaq3fvh8G8cZ7T-OahDUos_GyhnIj39C-9ouqdJqMUYFETqbzRCR6d36CpQ"
      }
   ]
}`
