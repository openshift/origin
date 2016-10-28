package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/registry/api/errcode"
	gocontext "golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"github.com/davecgh/go-spew/spew"
)

func TestImageStreamImport(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// can't give invalid image specs, should be invalid
	isi, err := c.ImageStreams(testutil.Namespace()).Import(&api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "doesnotexist",
		},
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "///a/a/a/a/a/redis:latest"}, To: &kapi.LocalObjectReference{Name: "tag"}},
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis:latest"}},
			},
		},
	})
	if err == nil || isi != nil || !errors.IsInvalid(err) {
		t.Fatalf("unexpected responses: %#v %#v %#v", err, isi, isi.Status.Import)
	}
	// does not create stream
	if _, err := c.ImageStreams(testutil.Namespace()).Get("doesnotexist"); err == nil || !errors.IsNotFound(err) {
		t.Fatal(err)
	}

	// import without committing
	isi, err = c.ImageStreams(testutil.Namespace()).Import(&api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "doesnotexist",
		},
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis:latest"}, To: &kapi.LocalObjectReference{Name: "other"}},
			},
		},
	})
	if err != nil || isi == nil || isi.Status.Import != nil {
		t.Fatalf("unexpected responses: %v %#v %#v", err, isi, isi.Status.Import)
	}
	// does not create stream
	if _, err := c.ImageStreams(testutil.Namespace()).Get("doesnotexist"); err == nil || !errors.IsNotFound(err) {
		t.Fatal(err)
	}

	// import with commit
	isi, err = c.ImageStreams(testutil.Namespace()).Import(&api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "doesnotexist",
		},
		Spec: api.ImageStreamImportSpec{
			Import: true,
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis:latest"}, To: &kapi.LocalObjectReference{Name: "other"}},
			},
		},
	})
	if err != nil || isi == nil || isi.Status.Import == nil {
		t.Fatalf("unexpected responses: %v %#v %#v", err, isi, isi.Status.Import)
	}

	if isi.Status.Images[0].Image == nil || isi.Status.Images[0].Image.DockerImageMetadata.Size == 0 || len(isi.Status.Images[0].Image.DockerImageLayers) == 0 {
		t.Fatalf("unexpected image output: %#v", isi.Status.Images[0].Image)
	}

	stream := isi.Status.Import
	if _, ok := stream.Annotations[api.DockerImageRepositoryCheckAnnotation]; !ok {
		t.Fatalf("unexpected stream: %#v", stream)
	}
	if stream.Generation != 1 || len(stream.Spec.Tags) != 1 || len(stream.Status.Tags) != 1 {
		t.Fatalf("unexpected stream: %#v", stream)
	}
	for tag, ref := range stream.Spec.Tags {
		if ref.Generation == nil || *ref.Generation != stream.Generation || tag != "other" || ref.From == nil ||
			ref.From.Name != "redis:latest" || ref.From.Kind != "DockerImage" {
			t.Fatalf("unexpected stream: %#v", stream)
		}
		event := stream.Status.Tags[tag]
		if len(event.Conditions) > 0 || len(event.Items) != 1 || event.Items[0].Generation != stream.Generation || strings.HasPrefix(event.Items[0].DockerImageReference, "docker.io/library/redis@sha256:") {
			t.Fatalf("unexpected stream: %#v", stream)
		}
	}

	// stream should not have changed
	stream2, err := c.ImageStreams(testutil.Namespace()).Get("doesnotexist")
	if err != nil {
		t.Fatal(err)
	}
	if stream.Generation != stream2.Generation || !reflect.DeepEqual(stream.Spec, stream2.Spec) ||
		!reflect.DeepEqual(stream.Status, stream2.Status) || !reflect.DeepEqual(stream.Annotations, stream2.Annotations) {
		t.Errorf("streams changed: %#v %#v", stream, stream2)
	}
}

// mockRegistryHandler returns a registry mock handler with several repositories. requireAuth causes handler
// to return unauthorized and request basic authentication header if not given. count is increased each
// time the handler is invoked. There are three repositories:
//  - test/image with phpManifest
//  - test/image2 with etcdManifest
//  - test/image3 with tags: v1, v2 and latest
//    - the first points to etcdManifest
//    - the others cause handler to return unknown error
func mockRegistryHandler(t *testing.T, requireAuth bool, count *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*count)++
		t.Logf("%d got %s %s", *count, r.Method, r.URL.Path)

		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		if requireAuth {
			if len(r.Header.Get("Authorization")) == 0 {
				w.Header().Set("WWW-Authenticate", "BASIC")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		switch r.URL.Path {
		case "/v2/":
			w.Write([]byte(`{}`))
		case "/v2/test/image/manifests/" + phpDigest:
			w.Write([]byte(phpManifest))
		case "/v2/test/image2/manifests/" + etcdDigest:
			w.Write([]byte(etcdManifest))
		case "/v2/test/image3/tags/list":
			w.Write([]byte("{\"name\": \"test/image3\", \"tags\": [\"latest\", \"v1\", \"v2\"]}"))
		case "/v2/test/image3/manifests/latest", "/v2/test/image3/manifests/v2", "/v2/test/image3/manifests/" + danglingDigest:
			errcode.ServeJSON(w, errcode.ErrorCodeUnknown)
		case "/v2/test/image3/manifests/v1", "/v2/test/image3/manifests/" + etcdDigest:
			if r.Method == "HEAD" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(etcdManifest)))
				w.Header().Set("Docker-Content-Digest", etcdDigest)
				w.WriteHeader(http.StatusOK)
			} else {
				w.Write([]byte(etcdManifest))
			}
		default:
			t.Fatalf("unexpected request %s: %#v", r.URL.Path, r)
		}
	})
}

func testImageStreamImport(t *testing.T, c *client.Client, imageSize int64, imagestreamimport *api.ImageStreamImport) {
	imageStreams := c.ImageStreams(testutil.Namespace())

	isi, err := imageStreams.Import(imagestreamimport)
	if err != nil {
		t.Fatal(err)
	}

	if len(isi.Status.Images) != 1 {
		t.Errorf("imported unexpected number of images (%d != 1)", len(isi.Status.Images))
	}

	for i, image := range isi.Status.Images {
		if image.Status.Status != unversioned.StatusSuccess {
			t.Errorf("unexpected status %d: %#v", i, image.Status)
		}

		if image.Image == nil {
			t.Errorf("unexpected empty image %d", i)
		}

		// the image name is always the sha256, and size is calculated
		if image.Image.Name != convertedDigest {
			t.Errorf("unexpected image %d: %#v (expect %q)", i, image.Image.Name, convertedDigest)
		}

		// the image size is calculated
		if image.Image.DockerImageMetadata.Size == 0 {
			t.Errorf("unexpected image size %d: %#v", i, image.Image.DockerImageMetadata.Size)
		}

		if image.Image.DockerImageMetadata.Size != imageSize {
			t.Errorf("unexpected image size %d: %#v (expect %d)", i, image.Image.DockerImageMetadata.Size, imageSize)
		}
	}
}

func testImageStreamImportWithPath(t *testing.T, reponame string) {
	imageDigest := "sha256:815d06b56f4138afacd0009b8e3799fcdce79f0507bf8d0588e219b93ab6fd4d"
	descriptors := map[string]int64{
		"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4": 3000,
		"sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa": 200,
		"sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4": 10,
	}

	imageSize := int64(0)
	for _, size := range descriptors {
		imageSize += size
	}

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	// start regular HTTP servers
	requireAuth := false
	count := 0
	countStat := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		t.Logf("%d got %s %s", count, r.Method, r.URL.Path)

		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		if requireAuth {
			if len(r.Header.Get("Authorization")) == 0 {
				w.Header().Set("WWW-Authenticate", "BASIC")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		switch r.URL.Path {
		case "/v2/":
			w.Write([]byte(`{}`))
		case "/v2/" + reponame + "/tags/list":
			w.Write([]byte("{\"name\": \"" + reponame + "\", \"tags\": [\"testtag\"]}"))
		case "/v2/" + reponame + "/manifests/testtag", "/v2/" + reponame + "/manifests/" + imageDigest:
			if r.Method == "HEAD" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(convertedManifest)))
				w.Header().Set("Docker-Content-Digest", imageDigest)
				w.WriteHeader(http.StatusOK)
			} else {
				w.Write([]byte(convertedManifest))
			}
		default:
			if strings.HasPrefix(r.URL.Path, "/v2/"+reponame+"/blobs/") {
				for dgst, size := range descriptors {
					if r.URL.Path != "/v2/"+reponame+"/blobs/"+dgst {
						continue
					}
					if r.Method == "HEAD" {
						w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
						w.Header().Set("Docker-Content-Digest", dgst)
						w.WriteHeader(http.StatusOK)
						countStat++
						return
					}
				}
			}
			t.Fatalf("unexpected request %s: %#v", r.URL.Path, r)
		}
	}))

	url, _ := url.Parse(server.URL)

	// start a master
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testImageStreamImport(t, c, imageSize, &api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		Spec: api.ImageStreamImportSpec{
			Import: true,
			Images: []api.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: url.Host + "/" + reponame + ":testtag"},
					To:           &kapi.LocalObjectReference{Name: "other"},
					ImportPolicy: api.TagImportPolicy{Insecure: true},
				},
			},
		},
	})

	if countStat != len(descriptors) {
		t.Fatalf("unexpected number of blob stats %d (expected %d)", countStat, len(descriptors))
	}

	testImageStreamImport(t, c, imageSize, &api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test1",
		},
		Spec: api.ImageStreamImportSpec{
			Import: true,
			Images: []api.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: url.Host + "/" + reponame + ":testtag"},
					To:           &kapi.LocalObjectReference{Name: "other1"},
					ImportPolicy: api.TagImportPolicy{Insecure: true},
				},
			},
		},
	})

	// Test that the global layer cache is working. The counter shouldn't change
	// because all the information is available in the cache.
	if countStat != len(descriptors) {
		t.Fatalf("the global layer cache is not working: unexpected number of blob stats %d (expected %d)", countStat, len(descriptors))
	}
}

func TestImageStreamImportOfV1ImageFromV2Repository(t *testing.T) {
	testImageStreamImportWithPath(t, "test/image")
}

func TestImageStreamImportOfMultiSegmentDockerReference(t *testing.T) {
	testImageStreamImportWithPath(t, "test/foo/bar/image")
}

func TestImageStreamImportAuthenticated(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	// start regular HTTP servers
	count := 0
	server := httptest.NewServer(mockRegistryHandler(t, true, &count))
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		if len(r.Header.Get("Authorization")) == 0 {
			w.Header().Set("WWW-Authenticate", "BASIC")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))

	// start a TLS server
	count2 := 0
	server3 := httptest.NewTLSServer(mockRegistryHandler(t, true, &count2))

	url1, _ := url.Parse(server.URL)
	url2, _ := url.Parse(server2.URL)
	url3, _ := url.Parse(server3.URL)

	// start a master
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kc, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	specFn := func(insecure bool, host1, host2 string) *api.ImageStreamImport {
		return &api.ImageStreamImport{
			ObjectMeta: kapi.ObjectMeta{Name: "test"},
			Spec: api.ImageStreamImportSpec{
				Import: true,
				Images: []api.ImageImportSpec{
					{
						From:         kapi.ObjectReference{Kind: "DockerImage", Name: host1 + "/test/image@" + phpDigest},
						To:           &kapi.LocalObjectReference{Name: "latest"},
						ImportPolicy: api.TagImportPolicy{Insecure: insecure},
					},
					{
						From:         kapi.ObjectReference{Kind: "DockerImage", Name: host1 + "/test/image2@" + etcdDigest},
						To:           &kapi.LocalObjectReference{Name: "other"},
						ImportPolicy: api.TagImportPolicy{Insecure: insecure},
					},
					{
						From:         kapi.ObjectReference{Kind: "DockerImage", Name: host2 + "/test/image:other"},
						To:           &kapi.LocalObjectReference{Name: "failed"},
						ImportPolicy: api.TagImportPolicy{Insecure: insecure},
					},
				},
			},
		}
	}

	// import expecting auth errors
	importSpec := specFn(true, url1.Host, url2.Host)
	isi, err := c.ImageStreams(testutil.Namespace()).Import(importSpec)
	if err != nil {
		t.Fatal(err)
	}
	if isi == nil || isi.Status.Import == nil {
		t.Fatalf("unexpected responses: %#v", isi)
	}
	for i, image := range isi.Status.Images {
		if image.Status.Status != unversioned.StatusFailure || image.Status.Reason != unversioned.StatusReasonUnauthorized {
			t.Fatalf("import of image %d did not report unauthorized: %#v", i, image.Status)
		}
	}

	servers := []string{url1.Host, url3.Host}

	// test import of both TLS and non-TLS with an insecure input
	for i, host := range servers {
		t.Logf("testing %s host", host)

		// add secrets for subsequent checks
		_, err = kc.Secrets(testutil.Namespace()).Create(&kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{Name: fmt.Sprintf("secret-%d", i+1)},
			Type:       kapi.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				kapi.DockerConfigJsonKey: []byte(`{"auths":{"` + host + `/test/image/":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("user:password")) + `"}}}`),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		importSpec := specFn(true, host, url2.Host)

		// import expecting regular image to pass
		isi, err = c.ImageStreams(testutil.Namespace()).Import(importSpec)
		if err != nil {
			t.Fatal(err)
		}
		for i, image := range isi.Status.Images {
			switch i {
			case 1, 2:
				if image.Status.Status != unversioned.StatusFailure || image.Status.Reason != unversioned.StatusReasonUnauthorized {
					t.Fatalf("import of image %d did not report unauthorized: %#v", i, image.Status)
				}
			default:
				if image.Status.Status != unversioned.StatusSuccess {
					t.Fatalf("import of image %d did not succeed: %#v", i, image.Status)
				}
			}
		}

		if items := isi.Status.Import.Status.Tags["latest"].Items; len(items) != 1 || items[0].Image != phpDigest {
			t.Fatalf("import of first image does not point to the correct image: %#v", items)
		}
		if isi.Status.Images[0].Image == nil || isi.Status.Images[0].Image.DockerImageMetadata.Size == 0 || len(isi.Status.Images[0].Image.DockerImageLayers) == 0 {
			t.Fatalf("unexpected image output: %#v", isi.Status.Images[0].Image)
		}

		is, err := c.ImageStreams(testutil.Namespace()).Get("test")
		if err != nil {
			t.Fatal(err)
		}
		tagEvent := api.LatestTaggedImage(is, "latest")
		if tagEvent == nil {
			t.Fatalf("no image tagged for latest: %#v", is)
		}

		var expectedGen int64
		switch i {
		case 0:
			expectedGen = 2
		case 1:
			expectedGen = 3
		}

		if tagEvent == nil || tagEvent.Image != phpDigest || tagEvent.Generation != expectedGen || tagEvent.DockerImageReference != host+"/test/image@"+phpDigest {
			t.Fatalf("expected the php image to be tagged: %#v", tagEvent)
		}
		tag, ok := is.Spec.Tags["latest"]
		if !ok {
			t.Fatalf("object at generation %d did not have tag latest: %#v", is.Generation, is)
		}
		tagGen := tag.Generation
		if is.Generation != expectedGen || tagGen == nil || *tagGen != expectedGen {
			t.Fatalf("expected generation %d for stream and spec tag: %d %#v", expectedGen, *tagGen, is)
		}
		if len(is.Status.Tags["latest"].Conditions) > 0 {
			t.Fatalf("incorrect conditions: %#v", is.Status.Tags["latest"].Conditions)
		}
		if !api.HasTagCondition(is, "other", api.TagEventCondition{Type: api.ImportSuccess, Status: kapi.ConditionFalse, Reason: "Unauthorized"}) {
			t.Fatalf("incorrect condition: %#v", is.Status.Tags["other"].Conditions)
		}
	}
}

// Verifies that individual errors for particular tags are handled properly when pulling all tags from a
// repository.
func TestImageStreamImportTagsFromRepository(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	// start regular HTTP servers
	count := 0
	server := httptest.NewServer(mockRegistryHandler(t, false, &count))

	url, _ := url.Parse(server.URL)

	// start a master
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	/*
		_, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	*/
	c, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	importSpec := &api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{Name: "test"},
		Spec: api.ImageStreamImportSpec{
			Import: true,
			Repository: &api.RepositoryImportSpec{
				From:            kapi.ObjectReference{Kind: "DockerImage", Name: url.Host + "/test/image3"},
				ImportPolicy:    api.TagImportPolicy{Insecure: true},
				IncludeManifest: true,
			},
		},
	}

	// import expecting regular image to pass
	isi, err := c.ImageStreams(testutil.Namespace()).Import(importSpec)
	if err != nil {
		t.Fatal(err)
	}
	if len(isi.Status.Images) != 0 {
		t.Errorf("imported unexpected number of images (%d != 0)", len(isi.Status.Images))
	}
	if isi.Status.Repository == nil {
		t.Fatalf("exported non-nil repository status")
	}
	if len(isi.Status.Repository.Images) != 3 {
		t.Fatalf("imported unexpected number of tags (%d != 3)", len(isi.Status.Repository.Images))
	}
	for i, image := range isi.Status.Repository.Images {
		switch i {
		case 2:
			if image.Status.Status != unversioned.StatusSuccess {
				t.Errorf("import of image %d did not succeed: %#v", i, image.Status)
			}
			if image.Tag != "v1" {
				t.Errorf("unexpected tag at position %d (%s != v1)", i, image.Tag)
			}
			if image.Image == nil {
				t.Fatalf("expected image to be set")
			}
			if image.Image.DockerImageReference != url.Host+"/test/image3@"+etcdDigest {
				t.Errorf("unexpected DockerImageReference (%s != %s)", image.Image.DockerImageReference, url.Host+"/test/image3@"+etcdDigest)
			}
			if image.Image.Name != etcdDigest {
				t.Errorf("expected etcd digest as a name of the image (%s != %s)", image.Image.Name, etcdDigest)
			}
		default:
			if image.Status.Status != unversioned.StatusFailure || image.Status.Reason != unversioned.StatusReasonInternalError {
				t.Fatalf("import of image %d did not report internal server error: %#v", i, image.Status)
			}
			expectedTags := []string{"latest", "v2"}[i]
			if image.Tag != expectedTags {
				t.Errorf("unexpected tag at position %d (%s != %s)", i, image.Tag, expectedTags)
			}
		}
	}

	is, err := c.ImageStreams(testutil.Namespace()).Get("test")
	if err != nil {
		t.Fatal(err)
	}
	tagEvent := api.LatestTaggedImage(is, "v1")
	if tagEvent == nil {
		t.Fatalf("no image tagged for v1: %#v", is)
	}

	if tagEvent == nil || tagEvent.Image != etcdDigest || tagEvent.DockerImageReference != url.Host+"/test/image3@"+etcdDigest {
		t.Fatalf("expected the etcd image to be tagged: %#v", tagEvent)
	}
}

// Verifies that the import scheduler fetches an image repeatedly (every 1s as per the default
// test controller interval), updates the image stream only when there are changes, and if an
// error occurs writes the error only once (instead of every interval)
func TestImageStreamImportScheduled(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	written := make(chan struct{}, 1)
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("got %s %s", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/v2/":
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.Write([]byte(`{}`))
		case "/v2/test/image/manifests/latest", "/v2/test/image/manifests/" + etcdDigest, "/v2/test/image/manifests/" + phpDigest:
			count++
			t.Logf("serving %d", count)
			var manifest, digest string
			switch count {
			case 1, 2:
				digest = etcdDigest
				manifest = etcdManifest
			case 3, 4, 5, 6:
				digest = phpDigest
				manifest = phpManifest
			default:
				w.WriteHeader(500)
				return
			}
			if r.Method == "HEAD" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest)))
				w.Header().Set("Docker-Content-Digest", digest)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Write([]byte(manifest))
			written <- struct{}{}
		default:
			t.Fatalf("unexpected request %s: %#v", r.URL.Path, r)
		}
	}))
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	url, _ := url.Parse(server.URL)

	// import with commit
	isi, err := c.ImageStreams(testutil.Namespace()).Import(&api.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		Spec: api.ImageStreamImportSpec{
			Import: true,
			Images: []api.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: url.Host + "/test/image:latest"},
					To:           &kapi.LocalObjectReference{Name: "latest"},
					ImportPolicy: api.TagImportPolicy{Insecure: true, Scheduled: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if isi == nil || isi.Status.Import == nil {
		t.Fatalf("unexpected responses: %#v", isi)
	}

	if !isi.Status.Import.Spec.Tags["latest"].ImportPolicy.Scheduled {
		t.Fatalf("scheduled tag not saved: %#v", isi.Status.Import)
	}
	if items := isi.Status.Import.Status.Tags["latest"].Items; len(items) != 1 || items[0].Image != etcdDigest {
		t.Fatalf("import of first image does not point to the correct image: %#v", items)
	}

	if isi.Status.Images[0].Image == nil || isi.Status.Images[0].Image.DockerImageMetadata.Size == 0 || len(isi.Status.Images[0].Image.DockerImageLayers) == 0 {
		t.Fatalf("unexpected image output: %#v", isi.Status.Images[0].Image)
	}

	// initial import
	<-written
	// scheduled import
	<-written

	is := isi.Status.Import
	w, err := c.ImageStreams(is.Namespace).Watch(kapi.ListOptions{ResourceVersion: is.ResourceVersion})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	ch := w.ResultChan()
	var event watch.Event
	select {
	case event = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("never got watch event")
	}
	change, ok := event.Object.(*api.ImageStream)
	if !ok {
		t.Fatalf("unexpected object: %#v", event.Object)
	}
	tagEvent := api.LatestTaggedImage(change, "latest")
	if tagEvent == nil {
		t.Fatalf("no image tagged for latest: %#v", change)
	}
	if tagEvent == nil || tagEvent.Image != phpDigest || tagEvent.Generation != 2 {
		t.Fatalf("expected the php image to be tagged: %#v", tagEvent)
	}
	tagGen := change.Spec.Tags["latest"].Generation
	if change.Generation != 2 || tagGen == nil || *tagGen != 2 {
		t.Fatalf("expected generation 2 for stream and spec tag: %v %#v", tagGen, change)
	}

	tag, ok := change.Spec.Tags["latest"]
	if !ok {
		t.Fatalf("object at generation %d did not have tag latest: %#v", change.Generation, change)
	}
	if gen := tag.Generation; gen == nil || *gen != 2 {
		t.Fatalf("object at generation %d had spec tag: %#v", change.Generation, tag)
	}
	items := change.Status.Tags["latest"].Items
	if len(items) != 2 {
		t.Fatalf("object at generation %d should have two tagged images: %#v", change.Generation, change.Status.Tags["latest"])
	}
	if items[0].Image != phpDigest || items[0].DockerImageReference != url.Host+"/test/image@"+phpDigest {
		t.Fatalf("expected tagged image: %#v", items[0])
	}
	if items[1].Image != etcdDigest || items[1].DockerImageReference != url.Host+"/test/image@"+etcdDigest {
		t.Fatalf("expected tagged image: %#v", items[1])
	}

	// wait for next event
	select {
	case <-written:
	case <-time.After(2 * time.Second):
		t.Fatalf("waited too long for 3rd write")
	}

	// expect to have the error recorded on the server
	event = <-ch
	change, ok = event.Object.(*api.ImageStream)
	if !ok {
		t.Fatalf("unexpected object: %#v", event.Object)
	}
	tagEvent = api.LatestTaggedImage(change, "latest")
	if tagEvent == nil {
		t.Fatalf("no image tagged for latest: %#v", change)
	}
	if tagEvent == nil || tagEvent.Image != phpDigest || tagEvent.Generation != 2 {
		t.Fatalf("expected the php image to be tagged: %#v", tagEvent)
	}
	tagGen = change.Spec.Tags["latest"].Generation
	if change.Generation != 3 || tagGen == nil || *tagGen != 3 {
		t.Fatalf("expected generation 2 for stream and spec tag: %v %#v", tagGen, change)
	}
	conditions := change.Status.Tags["latest"].Conditions
	if len(conditions) == 0 || conditions[0].Type != api.ImportSuccess || conditions[0].Generation != 3 {
		t.Fatalf("expected generation 3 for condition and import failed: %#v", conditions)
	}

	// sleep for a period of time to check for a second scheduled import
	time.Sleep(2 * time.Second)
	select {
	case event := <-ch:
		t.Fatalf("there should not have been a second import after failure: %s %#v", event.Type, event.Object)
	default:
	}
}

func TestImageStreamImportDockerHub(t *testing.T) {
	rt, _ := restclient.TransportFor(&restclient.Config{})
	importCtx := importer.NewContext(rt, nil).WithCredentials(importer.NoCredentials)

	imports := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Repository: &api.RepositoryImportSpec{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "mongo"},
			},
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis"}},
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "mysql"}},
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis:latest"}},
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "mysql/doesnotexistinanyform"}},
			},
		},
	}

	err := retryWhenUnreachable(t, func() error {
		i := importer.NewImageStreamImporter(importCtx, 3, nil, nil)
		if err := i.Import(gocontext.Background(), imports); err != nil {
			return err
		}

		errs := []error{}
		for i, d := range imports.Status.Images {
			fromName := imports.Spec.Images[i].From.Name
			if d.Status.Status != unversioned.StatusSuccess && fromName != "mysql/doesnotexistinanyform" {
				errs = append(errs, fmt.Errorf("failed to import an image %s: %v", fromName, d.Status.Message))
			}
		}
		return kerrors.NewAggregate(errs)
	})
	if err != nil {
		t.Fatal(err)
	}

	if imports.Status.Repository.Status.Status != unversioned.StatusSuccess || len(imports.Status.Repository.Images) != 3 || len(imports.Status.Repository.AdditionalTags) < 1 {
		t.Errorf("unexpected repository: %#v", imports.Status.Repository)
	}
	if len(imports.Status.Images) != 4 {
		t.Fatalf("unexpected response: %#v", imports.Status.Images)
	}
	d := imports.Status.Images[0]
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, "redis@") || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		t.Errorf("unexpected object: %#v", d.Image)
	}
	d = imports.Status.Images[1]
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, "mysql@") || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		t.Errorf("unexpected object: %#v", d.Image)
	}
	d = imports.Status.Images[2]
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, "redis@") || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		t.Errorf("unexpected object: %#v", d.Image)
	}
	d = imports.Status.Images[3]
	if d.Image != nil || d.Status.Status != unversioned.StatusFailure || d.Status.Reason != "Unauthorized" {
		t.Errorf("unexpected object: %#v", d)
	}
}

func TestImageStreamImportQuayIO(t *testing.T) {
	rt, _ := restclient.TransportFor(&restclient.Config{})
	importCtx := importer.NewContext(rt, nil).WithCredentials(importer.NoCredentials)

	repositoryName := quayRegistryName + "/coreos/etcd"
	imports := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: repositoryName}},
			},
		},
	}

	err := retryWhenUnreachable(t, func() error {
		i := importer.NewImageStreamImporter(importCtx, 3, nil, nil)
		if err := i.Import(gocontext.Background(), imports); err != nil {
			return err
		}

		errs := []error{}
		for i, d := range imports.Status.Images {
			fromName := imports.Spec.Images[i].From.Name
			if d.Status.Status != unversioned.StatusSuccess {
				if d.Status.Reason == "NotV2Registry" {
					t.Skipf("the server did not report as a v2 registry: %#v", d.Status)
				}
				errs = append(errs, fmt.Errorf("failed to import an image %s: %v", fromName, d.Status.Message))
			}
		}
		return kerrors.NewAggregate(errs)
	}, imageNotFoundErrorPatterns...)
	if err != nil {
		t.Fatal(err)
	}

	if imports.Status.Repository != nil {
		t.Errorf("unexpected repository: %#v", imports.Status.Repository)
	}
	if len(imports.Status.Images) != 1 {
		t.Fatalf("unexpected response: %#v", imports.Status.Images)
	}
	d := imports.Status.Images[0]
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, repositoryName+"@") || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		s := spew.ConfigState{
			Indent: " ",
			// Extra deep spew.
			DisableMethods: true,
		}
		t.Logf("import: %s", s.Sdump(d))
		t.Fatalf("unexpected object: %#v", d.Image)
	}
}

func TestImageStreamImportRedHatRegistry(t *testing.T) {
	rt, _ := restclient.TransportFor(&restclient.Config{})
	importCtx := importer.NewContext(rt, nil).WithCredentials(importer.NoCredentials)

	repositoryName := pulpRegistryName + "/rhel7"
	// test without the client on the context
	imports := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: repositoryName}},
			},
		},
	}

	i := importer.NewImageStreamImporter(importCtx, 3, nil, nil)
	if err := i.Import(gocontext.Background(), imports); err != nil {
		t.Fatal(err)
	}

	if imports.Status.Repository != nil {
		t.Errorf("unexpected repository: %#v", imports.Status.Repository)
	}
	if len(imports.Status.Images) != 1 {
		t.Fatalf("unexpected response: %#v", imports.Status.Images)
	}
	d := imports.Status.Images[0]
	if d.Image == nil || d.Status.Status == unversioned.StatusFailure {
		t.Errorf("unexpected object: %#v", d.Status)
	}

	// test with the client on the context
	imports = &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: repositoryName}},
			},
		},
	}
	context := gocontext.WithValue(gocontext.Background(), importer.ContextKeyV1RegistryClient, dockerregistry.NewClient(20*time.Second, false))
	importCtx = importer.NewContext(rt, nil).WithCredentials(importer.NoCredentials)
	err := retryWhenUnreachable(t, func() error {
		i = importer.NewImageStreamImporter(importCtx, 3, nil, nil)
		if err := i.Import(context, imports); err != nil {
			return err
		}

		errs := []error{}
		for i, d := range imports.Status.Images {
			fromName := imports.Spec.Images[i].From.Name
			if d.Status.Status != unversioned.StatusSuccess {
				errs = append(errs, fmt.Errorf("failed to import an image %s: %v", fromName, d.Status.Message))
			}
		}
		return kerrors.NewAggregate(errs)
	}, imageNotFoundErrorPatterns...)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate has expired or is not yet valid") {
			t.Skip("SKIPPING: due to expired certificate of %s: %v", pulpRegistryName, err)
		}
		t.Fatal(err.Error())
	}

	if imports.Status.Repository != nil {
		t.Errorf("unexpected repository: %#v", imports.Status.Repository)
	}
	if len(imports.Status.Images) != 1 {
		t.Fatalf("unexpected response: %#v", imports.Status.Images)
	}
	d = imports.Status.Images[0]
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, repositoryName) || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		t.Logf("imports: %#v", imports.Status.Images[0].Image)
		t.Fatalf("unexpected object: %#v", d.Image)
	}
}

const etcdDigest = "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238"
const etcdManifest = `{
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

const phpDigest = "sha256:28ba1e77e05a16a44e27250ff4f7116290eba339cc1a57d1652557eca4f25133"
const phpManifest = `{
   "schemaVersion": 1,
   "name": "library/php",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:4f792c7eb9b01ebb4dd917ec65e27792356511424799aa900b5808fd6590be18"
      },
      {
         "blobSum": "sha256:411817073dac032f58aa0914c211dff2eb9c5708689cd4ece56d53e05c21ded2"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:7dd26e858d19ce36e309be7f7491103c7b1c3b79422f6e5da4fb35a10ab9ee63"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:be29438d43e16dc0b63fbf64d7f989e85ccbab9546ac4b23ecd385194d0ff675"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:b0d203755789778bc81f787fcc925880446a55beb6244747719f62a928ff0ec3"
      },
      {
         "blobSum": "sha256:062e822a6a238cb0bb38f986572009b82aad1ab7f6fda0a236fa0fabc1080dc8"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:81cc5f26a6a083c024fb4138326e4d00f9a73f60c0e2a4399e1f7617ebe8c6c9"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"15bf13fd4c6eea6896b5e106fe7afbae13f76814b1752836197511d4d7a9cecf\",\"parent\":\"67f8a8991ac4021603d9a22c9ed1a1761eafe65bc294dd997d5ef3f07912f458\",\"created\":\"2016-01-07T23:11:43.954877182Z\",\"container\":\"b9c0ed8089b555bed5409aa908287247c07eae9f4e692cecfefd956e96181d7f\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"php\\\" \\\"-a\\\"]\"],\"Image\":\"67f8a8991ac4021603d9a22c9ed1a1761eafe65bc294dd997d5ef3f07912f458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"php\",\"-a\"],\"Image\":\"67f8a8991ac4021603d9a22c9ed1a1761eafe65bc294dd997d5ef3f07912f458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"67f8a8991ac4021603d9a22c9ed1a1761eafe65bc294dd997d5ef3f07912f458\",\"parent\":\"968bf28b055cb971927f838f92378f68f554f68438b0f4afa296a4bc1dd02545\",\"created\":\"2016-01-07T23:11:43.425996875Z\",\"container\":\"0cc18444b3e3a49e295b36aeef2fa96084ab10365bb6b173e90865d97a5f0562\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY multi:16473ef1e9e5136ff00d9f0d8e08ce89e246c7c07a954838c49a9bb12d8a777c in /usr/local/bin/\"],\"Image\":\"968bf28b055cb971927f838f92378f68f554f68438b0f4afa296a4bc1dd02545\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"968bf28b055cb971927f838f92378f68f554f68438b0f4afa296a4bc1dd02545\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":3243}"
      },
      {
         "v1Compatibility": "{\"id\":\"968bf28b055cb971927f838f92378f68f554f68438b0f4afa296a4bc1dd02545\",\"parent\":\"8de0073e0049f1f30f744ea3e8b363259d3fe8a2b58920c69e53448effd40b03\",\"created\":\"2016-01-07T23:11:37.735362743Z\",\"container\":\"715ba2bafc47921fae5de9a8a2425fccfda1b618e455bb6c886dfd4e100fa5f3\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"buildDeps=\\\" \\t\\t$PHP_EXTRA_BUILD_DEPS \\t\\tlibcurl4-openssl-dev \\t\\tlibreadline6-dev \\t\\tlibrecode-dev \\t\\tlibsqlite3-dev \\t\\tlibssl-dev \\t\\tlibxml2-dev \\t\\txz-utils \\t\\\" \\t\\u0026\\u0026 set -x \\t\\u0026\\u0026 apt-get update \\u0026\\u0026 apt-get install -y $buildDeps --no-install-recommends \\u0026\\u0026 rm -rf /var/lib/apt/lists/* \\t\\u0026\\u0026 curl -fSL \\\"http://php.net/get/$PHP_FILENAME/from/this/mirror\\\" -o \\\"$PHP_FILENAME\\\" \\t\\u0026\\u0026 echo \\\"$PHP_SHA256 *$PHP_FILENAME\\\" | sha256sum -c - \\t\\u0026\\u0026 curl -fSL \\\"http://php.net/get/$PHP_FILENAME.asc/from/this/mirror\\\" -o \\\"$PHP_FILENAME.asc\\\" \\t\\u0026\\u0026 gpg --verify \\\"$PHP_FILENAME.asc\\\" \\t\\u0026\\u0026 mkdir -p /usr/src/php \\t\\u0026\\u0026 tar -xf \\\"$PHP_FILENAME\\\" -C /usr/src/php --strip-components=1 \\t\\u0026\\u0026 rm \\\"$PHP_FILENAME\\\"* \\t\\u0026\\u0026 cd /usr/src/php \\t\\u0026\\u0026 ./configure \\t\\t--with-config-file-path=\\\"$PHP_INI_DIR\\\" \\t\\t--with-config-file-scan-dir=\\\"$PHP_INI_DIR/conf.d\\\" \\t\\t$PHP_EXTRA_CONFIGURE_ARGS \\t\\t--disable-cgi \\t\\t--enable-mysqlnd \\t\\t--with-curl \\t\\t--with-openssl \\t\\t--with-readline \\t\\t--with-recode \\t\\t--with-zlib \\t\\u0026\\u0026 make -j\\\"$(nproc)\\\" \\t\\u0026\\u0026 make install \\t\\u0026\\u0026 { find /usr/local/bin /usr/local/sbin -type f -executable -exec strip --strip-all '{}' + || true; } \\t\\u0026\\u0026 apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false -o APT::AutoRemove::SuggestsImportant=false $buildDeps \\t\\u0026\\u0026 make clean\"],\"Image\":\"8de0073e0049f1f30f744ea3e8b363259d3fe8a2b58920c69e53448effd40b03\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"8de0073e0049f1f30f744ea3e8b363259d3fe8a2b58920c69e53448effd40b03\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":163873449}"
      },
      {
         "v1Compatibility": "{\"id\":\"8de0073e0049f1f30f744ea3e8b363259d3fe8a2b58920c69e53448effd40b03\",\"parent\":\"f852296c35b495eedab0372f83aa29867033f523457be8f21301b2844369fcbb\",\"created\":\"2016-01-07T23:06:01.686374288Z\",\"container\":\"cfd0a2931a32a9d83b9fe62704ab553c83892819d3e638bee7eaf3fb6f3f173e\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Image\":\"f852296c35b495eedab0372f83aa29867033f523457be8f21301b2844369fcbb\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\",\"PHP_SHA256=556121271a34c442b48e3d7fa3d3bbb4413d91897abbb92aaeced4a7df5f2ab2\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"f852296c35b495eedab0372f83aa29867033f523457be8f21301b2844369fcbb\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"f852296c35b495eedab0372f83aa29867033f523457be8f21301b2844369fcbb\",\"parent\":\"faf69a3945b94cffec8865f042fda50040c2a4787184a41bab795d016cd241bd\",\"created\":\"2016-01-07T23:06:01.114172084Z\",\"container\":\"193d86a0600b1d0f7bf681aeb59691f024aeb0a838af2891f15c4fa04ee17115\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV PHP_FILENAME=php-7.0.2.tar.xz\"],\"Image\":\"faf69a3945b94cffec8865f042fda50040c2a4787184a41bab795d016cd241bd\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\",\"PHP_FILENAME=php-7.0.2.tar.xz\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"faf69a3945b94cffec8865f042fda50040c2a4787184a41bab795d016cd241bd\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"faf69a3945b94cffec8865f042fda50040c2a4787184a41bab795d016cd241bd\",\"parent\":\"515ddb30f9738bb7c640c7b8d0db0310f2680069de0d64f6a03d20e3b5867c2c\",\"created\":\"2016-01-07T23:06:00.510214735Z\",\"container\":\"d08440176be49255bc6aa3aa62a8297ede04a543c4f4391a6af61fda8a0cb90e\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV PHP_VERSION=7.0.2\"],\"Image\":\"515ddb30f9738bb7c640c7b8d0db0310f2680069de0d64f6a03d20e3b5867c2c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\",\"PHP_VERSION=7.0.2\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"515ddb30f9738bb7c640c7b8d0db0310f2680069de0d64f6a03d20e3b5867c2c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"515ddb30f9738bb7c640c7b8d0db0310f2680069de0d64f6a03d20e3b5867c2c\",\"parent\":\"7f0a7d8f6cb84260f4c9a9faec7ad5de6612a1cbe8b0a3c224f4cbd34abb8919\",\"created\":\"2016-01-07T18:41:49.343381571Z\",\"container\":\"acd6399faa8484a67b99760e34692152e3b1a700250f48b0b1bd7c9807cd12d2\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"set -xe \\t\\u0026\\u0026 for key in $GPG_KEYS; do \\t\\tgpg --keyserver ha.pool.sks-keyservers.net --recv-keys \\\"$key\\\"; \\tdone\"],\"Image\":\"7f0a7d8f6cb84260f4c9a9faec7ad5de6612a1cbe8b0a3c224f4cbd34abb8919\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"7f0a7d8f6cb84260f4c9a9faec7ad5de6612a1cbe8b0a3c224f4cbd34abb8919\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":13364}"
      },
      {
         "v1Compatibility": "{\"id\":\"7f0a7d8f6cb84260f4c9a9faec7ad5de6612a1cbe8b0a3c224f4cbd34abb8919\",\"parent\":\"2f593162b03c46b0c42b4391b667c86a42f68ec627fba6e004ed4f48ccd1fdf2\",\"created\":\"2016-01-07T18:41:47.002504914Z\",\"container\":\"66952bd5d81f704838b23414dc9df69a428c9f36de7593f734d4a799aea7175a\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\"],\"Image\":\"2f593162b03c46b0c42b4391b667c86a42f68ec627fba6e004ed4f48ccd1fdf2\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\",\"GPG_KEYS=1A4E8B7277C42E53DBA9C7B9BCAA30EA9C0D5763\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"2f593162b03c46b0c42b4391b667c86a42f68ec627fba6e004ed4f48ccd1fdf2\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"2f593162b03c46b0c42b4391b667c86a42f68ec627fba6e004ed4f48ccd1fdf2\",\"parent\":\"f24f231219c8b4ae4f455072c50c384876d6e1bef1279d06c9b20afcd197279a\",\"created\":\"2016-01-07T17:56:35.94340792Z\",\"container\":\"d4d137cb31f5892b41211de536574f55f5c1f9900fc586071bef36c6f4fbc352\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"mkdir -p $PHP_INI_DIR/conf.d\"],\"Image\":\"f24f231219c8b4ae4f455072c50c384876d6e1bef1279d06c9b20afcd197279a\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"f24f231219c8b4ae4f455072c50c384876d6e1bef1279d06c9b20afcd197279a\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"f24f231219c8b4ae4f455072c50c384876d6e1bef1279d06c9b20afcd197279a\",\"parent\":\"c45a8c0edcbfa141fcec8f774beefd76c4dfca71fd9e0080a20a10058ae94c87\",\"created\":\"2016-01-07T17:56:34.330366706Z\",\"container\":\"7e7788ced2d8dfd25ad8ca3e59939d54a54a374cb35fd5561f9a01a66cf8ba76\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENV PHP_INI_DIR=/usr/local/etc/php\"],\"Image\":\"c45a8c0edcbfa141fcec8f774beefd76c4dfca71fd9e0080a20a10058ae94c87\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PHP_INI_DIR=/usr/local/etc/php\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"c45a8c0edcbfa141fcec8f774beefd76c4dfca71fd9e0080a20a10058ae94c87\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"c45a8c0edcbfa141fcec8f774beefd76c4dfca71fd9e0080a20a10058ae94c87\",\"parent\":\"3d6e8b29f2649dbe7fcae02dc5c89e011f5f39b4fc9a5a5364e187e23bd35936\",\"created\":\"2016-01-07T17:56:32.007219215Z\",\"container\":\"dd2435019bd6d833dec2d621aa8b297fc6cdd265a1684d64767e0c3a33e171f8\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"apt-get update \\u0026\\u0026 apt-get install -y autoconf file g++ gcc libc-dev make pkg-config re2c --no-install-recommends \\u0026\\u0026 rm -r /var/lib/apt/lists/*\"],\"Image\":\"3d6e8b29f2649dbe7fcae02dc5c89e011f5f39b4fc9a5a5364e187e23bd35936\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"3d6e8b29f2649dbe7fcae02dc5c89e011f5f39b4fc9a5a5364e187e23bd35936\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":177217456}"
      },
      {
         "v1Compatibility": "{\"id\":\"3d6e8b29f2649dbe7fcae02dc5c89e011f5f39b4fc9a5a5364e187e23bd35936\",\"parent\":\"d4b2ba78e3b4b44bdfab5b625c210d6e410debba50446520fe1c3e1a5ee9cdea\",\"created\":\"2016-01-07T17:55:06.00749166Z\",\"container\":\"56966f28765b9579d30b6a6faf2401e9d4686741ee28c85d269bd3670d05bae9\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"apt-get update \\u0026\\u0026 apt-get install -y ca-certificates curl librecode0 libsqlite3-0 libxml2 --no-install-recommends \\u0026\\u0026 rm -r /var/lib/apt/lists/*\"],\"Image\":\"d4b2ba78e3b4b44bdfab5b625c210d6e410debba50446520fe1c3e1a5ee9cdea\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"d4b2ba78e3b4b44bdfab5b625c210d6e410debba50446520fe1c3e1a5ee9cdea\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":18619656}"
      },
      {
         "v1Compatibility": "{\"id\":\"d4b2ba78e3b4b44bdfab5b625c210d6e410debba50446520fe1c3e1a5ee9cdea\",\"parent\":\"cb6fb082434ea9d7f25798e96abc06cb176cbe910970ec86874555e7c9fbc04a\",\"created\":\"2016-01-07T01:07:11.982173215Z\",\"container\":\"1a9173a681853efd414d3ffc036871ac5a6c46e2aefbe839d186cce595d48a4d\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"/bin/bash\\\"]\"],\"Image\":\"cb6fb082434ea9d7f25798e96abc06cb176cbe910970ec86874555e7c9fbc04a\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/bash\"],\"Image\":\"cb6fb082434ea9d7f25798e96abc06cb176cbe910970ec86874555e7c9fbc04a\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"cb6fb082434ea9d7f25798e96abc06cb176cbe910970ec86874555e7c9fbc04a\",\"created\":\"2016-01-07T01:07:09.137000568Z\",\"container\":\"30db80bfe262b3b727e41bfe6e627075f5918e4b9f5c1276e626ff20a3dd6725\",\"container_config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:0098703cdfd5b5eda3aececc4d4600b0fb4b753e19c832c73df4f9d5fdcf3598 in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"30db80bfe262\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":125115267}"
      }
   ],
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
         "signature": "-A245lemaBLzMCdlIwtSIJGcAUsMae5s1hBZdRNAJ_0VuX6hm-hFe4zL5zEt0NREsgtTpY1oZzAIvu4bLXG9ig",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjI1ODkxLCJmb3JtYXRUYWlsIjoiQ24wIiwidGltZSI6IjIwMTYtMDEtMDhUMDI6MTg6MzVaIn0"
      },
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
         "signature": "1Xd_eR22enboeB638OlQf_r7Q4PSNrqxSWMJWSisiNVZfgcE7kpCkOWxmB0e28MXQp6LEgoJFkC9mYgUHhXZLw",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjI1ODkxLCJmb3JtYXRUYWlsIjoiQ24wIiwidGltZSI6IjIwMTYtMDEtMTJUMDA6NDI6MTZaIn0"
      }
   ]
}`

const danglingDigest = `sha256:f374c0d9b59e6fdf9f8922d59e946b05fbeabaed70b0639d7b6b524f3299e87b`

// This manifest obtained by conversion of manifest v2 -> v1.
const convertedDigest = `sha256:2a3b2f1a74f4351d1faf7ca48ea7f0ca97fad0801053fcf16df8474af6b89229`
const convertedManifest = `{
   "schemaVersion": 1,
   "name": "testrepo",
   "tag": "testtag",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"architecture\":\"amd64\",\"config\":{\"AttachStderr\":false,\"AttachStdin\":false,\"AttachStdout\":false,\"Cmd\":[\"/bin/sh\",\"-c\",\"echo hi\"],\"Domainname\":\"\",\"Entrypoint\":null,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"derived=true\",\"asdf=true\"],\"Hostname\":\"23304fc829f9\",\"Image\":\"sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246\",\"Labels\":{},\"OnBuild\":[],\"OpenStdin\":false,\"StdinOnce\":false,\"Tty\":false,\"User\":\"\",\"Volumes\":null,\"WorkingDir\":\"\"},\"container\":\"e91032eb0403a61bfe085ff5a5a48e3659e5a6deae9f4d678daa2ae399d5a001\",\"container_config\":{\"AttachStderr\":false,\"AttachStdin\":false,\"AttachStdout\":false,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"/bin/sh\\\" \\\"-c\\\" \\\"echo hi\\\"]\"],\"Domainname\":\"\",\"Entrypoint\":null,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"derived=true\",\"asdf=true\"],\"Hostname\":\"23304fc829f9\",\"Image\":\"sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246\",\"Labels\":{},\"OnBuild\":[],\"OpenStdin\":false,\"StdinOnce\":false,\"Tty\":false,\"User\":\"\",\"Volumes\":null,\"WorkingDir\":\"\"},\"created\":\"2015-02-21T02:11:06.735146646Z\",\"docker_version\":\"1.9.0-dev\",\"id\":\"cbd3d33071cb117ec9a5fb93474252564645a524c235c2ccd4c7307fce2797a0\",\"os\":\"linux\",\"parent\":\"74cf9c92699240efdba1903c2748ef57105d5bedc588084c4e88f3bb1c3ef0b0\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"74cf9c92699240efdba1903c2748ef57105d5bedc588084c4e88f3bb1c3ef0b0\",\"parent\":\"178be37afc7c49e951abd75525dbe0871b62ad49402f037164ee6314f754599d\",\"created\":\"2015-11-04T23:06:32.083868454Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c dd if=/dev/zero of=/file bs=1024 count=1024\"]}}"
      },
      {
         "v1Compatibility": "{\"id\":\"178be37afc7c49e951abd75525dbe0871b62ad49402f037164ee6314f754599d\",\"parent\":\"b449305a55a283538c4574856a8b701f2a3d5ec08ef8aec47f385f20339a4866\",\"created\":\"2015-11-04T23:06:31.192097572Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) ENV asdf=true\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"b449305a55a283538c4574856a8b701f2a3d5ec08ef8aec47f385f20339a4866\",\"parent\":\"9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e\",\"created\":\"2015-11-04T23:06:30.934316144Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) ENV derived=true\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2015-10-31T22:22:55.613815829Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) CMD [\\\"sh\\\"]\"]}}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2015-10-31T22:22:54.690851953Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /\"]}}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "TIOP:AA2T:3IBK:KFN7:7QHD:K6WD:B4AI:VPLM:WLSQ:42FK:EVFE:MCZX",
               "kty": "EC",
               "x": "uCM0GNdYDKESot2imey-klB8XVBrK1hK4qFRkknbob0",
               "y": "hQ6HVG870LIx3nC_H08lg6rWT6Qc90rPLuFAq1_p5ms"
            },
            "alg": "ES256"
         },
         "signature": "fNLStdEXg7gDEFMxUcMgVx1gojZKOsm1Vm5dVpCSqH3yMPlGyxrITigFJrQwXRXmQoLM30-3TDdFFZ59T94ZLA",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjM5OTYsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNi0wNy0wOFQwOTo1OTo0NloifQ"
      }
   ]
}`
