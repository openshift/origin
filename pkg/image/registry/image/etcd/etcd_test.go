package etcd

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/coreos/go-etcd/etcd"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
)

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestImageListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/images/" + namespace
	}
	return "/images"
}
func makeTestImageKey(namespace, id string) string {
	return "/images/" + namespace + "/" + id
}
func makeTestDefaultImageKey(id string) string {
	return makeTestImageKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultImageListKey() string {
	return makeTestImageListKey(kapi.NamespaceDefault)
}
func makeTestImageRepositoriesListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/imageRepositories/" + namespace
	}
	return "/imageRepositories"
}
func makeTestImageRepositoriesKey(namespace, id string) string {
	return "/imageRepositories/" + namespace + "/" + id
}
func makeTestDefaultImageRepositoriesKey(id string) string {
	return makeTestImageRepositoriesKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultImageRepositoriesListKey() string {
	return makeTestImageRepositoriesListKey(kapi.NamespaceDefault)
}

func newHelper(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeEtcdClient, latest.Codec, etcdtest.PathPrefix())
	return fakeEtcdClient, helper
}

func newStorage(t *testing.T) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "")
	return NewREST(etcdStorage), fakeClient
}

func TestStorage(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	image.NewRegistry(storage)
}

func validNewImage() *api.Image {
	return &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		DockerImageReference: "openshift/origin",
	}
}

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.store).ClusterScope()
	image := validNewImage()
	image.ObjectMeta = kapi.ObjectMeta{GenerateName: "foo"}
	test.TestCreate(
		// valid
		image,
		// invalid
		&api.Image{},
	)
}

func TestCreateRegistryError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("test error")
	storage := NewREST(helper)

	image := validNewImage()
	_, err := storage.Create(kapi.NewDefaultContext(), image)
	if err != fakeEtcdClient.Err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.TestIndex = true

	storage := NewREST(helper)

	existingImage := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "foo",
			ResourceVersion: "1",
		},
		DockerImageReference: "foo/bar:abcd1234",
	}

	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				CreatedIndex:  1,
				ModifiedIndex: 1,
			},
		},
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		DockerImageReference: "foo/bar:abcd1234",
	})
	if err == nil {
		t.Fatalf("Unexpected non error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected already exists error, got %s", err)
	}
}

func TestListError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("test error")
	storage := NewREST(helper)
	images, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != fakeEtcdClient.Err {
		t.Fatalf("Expected %#v, Got %#v", fakeEtcdClient.Err, err)
	}
	if images != nil {
		t.Errorf("Unexpected non-nil image list: %#v", images)
	}
}

func TestListEmptyList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images")] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}
	storage := NewREST(helper)
	images, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(images.(*api.ImageList).Items) != 0 {
		t.Errorf("Unexpected non-zero images list: %#v", images)
	}
	if images.(*api.ImageList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", images)
	}
}

func TestListPopulatedList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})},
				},
			},
		},
	}

	storage := NewREST(helper)

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	images := list.(*api.ImageList)

	if e, a := 2, len(images.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestListFiltered(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "bar",
								Labels: map[string]string{"env": "dev"},
							},
						}),
					},
				},
			},
		},
		E: nil,
	}
	storage := NewREST(helper)
	list, err := storage.List(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}), fields.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	images := list.(*api.ImageList)
	if len(images.Items) != 1 || images.Items[0].Name != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestCreateMissingID(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Image{})
	if obj != nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateOK(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	image, ok := obj.(*api.Image)
	if !ok {
		t.Errorf("Expected image type, got: %#v", obj)
	}
	if image.Name != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestGetError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("bad")
	storage := NewREST(helper)

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if image != nil {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %v, got %v", fakeEtcdClient.Err, err)
	}
}

func TestGetNotFound(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if image != nil {
		t.Errorf("Unexpected image: %#v", image)
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

func TestCreateSetsMetadata(t *testing.T) {
	testCases := []struct {
		image  *api.Image
		expect func(*api.Image) bool
	}{
		{
			image: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
				DockerImageReference: "openshift/ruby-19-centos",
			},
		},
		{
			expect: func(image *api.Image) bool {
				if image.DockerImageMetadata.Size != 28643712 {
					t.Errorf("image had size %d", image.DockerImageMetadata.Size)
					return false
				}
				if len(image.DockerImageLayers) != 4 || image.DockerImageLayers[0].Name != "sha256:744b46d0ac8636c45870a03830d8d82c20b75fbfb9bc937d5e61005d23ad4cfe" || image.DockerImageLayers[0].Size != 15141568 {
					t.Errorf("unexpected layers: %#v", image.DockerImageLayers)
					return false
				}
				return true
			},
			image: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
				DockerImageReference: "openshift/ruby-19-centos",
				DockerImageManifest:  etcdManifest,
			},
		},
	}

	for i, test := range testCases {
		_, helper := newHelper(t)
		storage := NewREST(helper)

		obj, err := storage.Create(kapi.NewDefaultContext(), test.image)
		if obj == nil {
			t.Errorf("%d: Expected nil obj, got %v", i, obj)
			continue
		}
		if err != nil {
			t.Errorf("%d: Unexpected non-nil error: %#v", i, err)
			continue
		}
		image, ok := obj.(*api.Image)
		if !ok {
			t.Errorf("%d: Expected image type, got: %#v", i, obj)
			continue
		}
		if test.expect != nil && !test.expect(image) {
			t.Errorf("%d: Unexpected image: %#v", i, obj)
		}
	}
}

func TestUpdateResetsMetadata(t *testing.T) {
	testCases := []struct {
		image    *api.Image
		existing *api.Image
		expect   func(*api.Image) bool
	}{
		// manifest changes are ignored
		{
			expect: func(image *api.Image) bool {
				if image.Labels["a"] != "b" {
					t.Errorf("unexpected labels: %s", image.Labels)
					return false
				}
				if image.DockerImageManifest != "" {
					t.Errorf("unexpected manifest: %s", image.DockerImageManifest)
					return false
				}
				if image.DockerImageMetadata.ID != "foo" {
					t.Errorf("unexpected docker image: %#v", image.DockerImageMetadata)
					return false
				}
				if image.DockerImageReference != "openshift/ruby-19-centos-2" {
					t.Errorf("image reference changed: %s", image.DockerImageReference)
					return false
				}
				if image.DockerImageMetadata.Size != 0 {
					t.Errorf("image had size %d", image.DockerImageMetadata.Size)
					return false
				}
				if len(image.DockerImageLayers) != 1 && image.DockerImageLayers[0].Size != 10 {
					t.Errorf("unexpected layers: %#v", image.DockerImageLayers)
					return false
				}
				return true
			},
			existing: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo", ResourceVersion: "1"},
				DockerImageReference: "openshift/ruby-19-centos-2",
				DockerImageLayers:    []api.ImageLayer{{Name: "test", Size: 10}},
				DockerImageMetadata:  api.DockerImage{ID: "foo"},
			},
			image: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo", ResourceVersion: "1", Labels: map[string]string{"a": "b"}},
				DockerImageReference: "openshift/ruby-19-centos",
				DockerImageManifest:  etcdManifest,
			},
		},
		// existing manifest is preserved, and unpacked
		{
			expect: func(image *api.Image) bool {
				if image.DockerImageManifest != etcdManifest {
					t.Errorf("unexpected manifest: %s", image.DockerImageManifest)
					return false
				}
				if image.DockerImageMetadata.ID != "fe50ac14986497fa6b5d2cc24feb4a561d01767bc64413752c0988cb70b0b8b9" {
					t.Errorf("unexpected docker image: %#v", image.DockerImageMetadata)
					return false
				}
				if image.DockerImageReference != "openshift/ruby-19-centos-2" {
					t.Errorf("image reference changed: %s", image.DockerImageReference)
					return false
				}
				if image.DockerImageMetadata.Size != 28643712 {
					t.Errorf("image had size %d", image.DockerImageMetadata.Size)
					return false
				}
				if len(image.DockerImageLayers) != 4 || image.DockerImageLayers[0].Name != "sha256:744b46d0ac8636c45870a03830d8d82c20b75fbfb9bc937d5e61005d23ad4cfe" || image.DockerImageLayers[0].Size != 15141568 {
					t.Errorf("unexpected layers: %#v", image.DockerImageLayers)
					return false
				}
				return true
			},
			existing: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo", ResourceVersion: "1"},
				DockerImageReference: "openshift/ruby-19-centos-2",
				DockerImageLayers:    []api.ImageLayer{},
				DockerImageManifest:  etcdManifest,
			},
			image: &api.Image{
				ObjectMeta:           kapi.ObjectMeta{Name: "foo", ResourceVersion: "1"},
				DockerImageReference: "openshift/ruby-19-centos",
				DockerImageMetadata:  api.DockerImage{ID: "foo"},
			},
		},
	}

	for i, test := range testCases {
		fakeEtcdClient, helper := newHelper(t)
		storage := NewREST(helper)

		fakeEtcdClient.Data[etcdtest.AddPrefix("/images/foo")] = tools.EtcdResponseWithError{
			R: &etcd.Response{
				Node: &etcd.Node{
					Value:         runtime.EncodeOrDie(latest.Codec, test.existing),
					CreatedIndex:  1,
					ModifiedIndex: 1,
				},
			},
		}

		obj, _, err := storage.Update(kapi.NewDefaultContext(), test.image)
		if err != nil {
			t.Errorf("%d: Unexpected non-nil error: %#v", i, err)
			continue
		}
		if obj == nil {
			t.Errorf("%d: Expected nil obj, got %v", i, obj)
			continue
		}
		image, ok := obj.(*api.Image)
		if !ok {
			t.Errorf("%d: Expected image type, got: %#v", i, obj)
			continue
		}
		if test.expect != nil && !test.expect(image) {
			t.Errorf("%d: Unexpected image: %#v", i, obj)
		}
	}
}

func TestGetOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	expectedImage := &api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	}
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, expectedImage),
			},
		},
	}
	storage := NewREST(helper)

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if image == nil {
		t.Fatal("Unexpected nil image")
	}
	if err != nil {
		t.Fatal("Unexpected non-nil error", err)
	}
	if image.(*api.Image).Name != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestDelete(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Image{}),
			},
		},
	}
	storage := NewREST(helper)

	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)

	if obj == nil {
		t.Error("Unexpected nil obj")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	status, ok := obj.(*unversioned.Status)
	if !ok {
		t.Fatalf("Expected status type, got: %#v", obj)
	}
	if status.Status != unversioned.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
	if len(fakeEtcdClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeEtcdClient.DeletedKeys)
	} else if key := etcdtest.AddPrefix("/images/foo"); fakeEtcdClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeEtcdClient.DeletedKeys[0], key)
	}
}

func TestDeleteNotFound(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = tools.EtcdErrorNotFound
	storage := NewREST(helper)
	_, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestDeleteImageError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("Some error")
	storage := NewREST(helper)
	_, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestWatchErrorWithFieldSet(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	_, err := storage.Watch(kapi.NewDefaultContext(), labels.Everything(), fields.SelectorFromSet(fields.Set{"foo": "bar"}), "1")
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	if err.Error() != "field selectors are not supported on images" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestWatchOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)

	var tests = []struct {
		label    labels.Selector
		images   []*api.Image
		expected []bool
	}{
		{
			labels.Everything(),
			[]*api.Image{
				{ObjectMeta: kapi.ObjectMeta{Name: "a"}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b"}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c"}, DockerImageMetadata: api.DockerImage{}},
			},
			[]bool{
				true,
				true,
				true,
			},
		},
		{
			labels.SelectorFromSet(labels.Set{"color": "blue"}),
			[]*api.Image{
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: map[string]string{"color": "blue"}}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b", Labels: map[string]string{"color": "green"}}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: map[string]string{"color": "blue"}}, DockerImageMetadata: api.DockerImage{}},
			},
			[]bool{
				true,
				false,
				true,
			},
		},
	}
	for _, tt := range tests {
		watching, err := storage.Watch(kapi.NewDefaultContext(), tt.label, fields.Everything(), "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fakeEtcdClient.WaitForWatchCompletion()

		for testIndex, image := range tt.images {
			imageBytes, _ := latest.Codec.Encode(image)
			fakeEtcdClient.WatchResponse <- &etcd.Response{
				Action: "set",
				Node: &etcd.Node{
					Value: string(imageBytes),
				},
			}

			select {
			case event, ok := <-watching.ResultChan():
				if !ok {
					t.Errorf("watching channel should be open")
				}
				if !tt.expected[testIndex] {
					t.Errorf("unexpected image returned from watch: %#v", event.Object)
				}
				if e, a := watch.Added, event.Type; e != a {
					t.Errorf("Expected %v, got %v", e, a)
				}
				image.DockerImageMetadataVersion = "1.0"
				if e, a := image, event.Object; !reflect.DeepEqual(e, a) {
					t.Errorf("Objects did not match: %s", util.ObjectDiff(e, a))
				}
			case <-time.After(50 * time.Millisecond):
				if tt.expected[testIndex] {
					t.Errorf("Expected image %#v to be returned from watch", image)
				}
			}
		}

		select {
		case _, ok := <-watching.ResultChan():
			if !ok {
				t.Errorf("watching channel should be open")
			}
		default:
		}

		fakeEtcdClient.WatchInjectError <- nil
		if _, ok := <-watching.ResultChan(); ok {
			t.Errorf("watching channel should be closed")
		}
		watching.Stop()
	}
}

type fakeStrategy struct {
	rest.RESTCreateStrategy
}

func (fakeStrategy) PrepareForCreate(obj runtime.Object) {
	img := obj.(*api.Image)
	img.Annotations = make(map[string]string, 1)
	img.Annotations["test"] = "PrepareForCreate"
}

func TestStrategyPrepareMethods(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	img := validNewImage()
	strategy := fakeStrategy{image.Strategy}

	storage.store.CreateStrategy = strategy

	obj, err := storage.Create(kapi.NewDefaultContext(), img)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	newImage := obj.(*api.Image)
	if newImage.Annotations["test"] != "PrepareForCreate" {
		t.Errorf("Expected PrepareForCreate annotation")
	}
}
