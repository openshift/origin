// +build integration,etcd

package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	gocontext "golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"github.com/davecgh/go-spew/spew"
)

func init() {
	testutil.RequireEtcd()
}

func TestImageStreamImport(t *testing.T) {
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
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "a/a/a/a/a/redis:latest"}, To: &kapi.LocalObjectReference{Name: "tag"}},
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

/*
// re-add this test as an integration test
func TestOpenShiftRegistry(t *testing.T) {
  token := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlZmF1bHQtdG9rZW4tNmpsOW0iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVmYXVsdCIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjNiNWRmMGRlLWExZTctMTFlNS05ZDkzLTA4MDAyN2M1YmZhOSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlZmF1bHQifQ.L5Hc0eHjZHo5BGeAI_SeHMSpYy4WK4_bSsbm-4UGBoyti7WOhTlJcAgUFTgEGu1mxDRmNtEA-xXdv0jXe377Q52C73Oli0ZuAnLgkBbL3wnIkWKUOZvrbcDw5hJaeTdhUvri5_ZkC4kbNXwJKpAIh8MonOfUjnmY7hQbISMLirhIj_orAKMql9nQbQTOfO4goAqscNMRHsJqYTCneMBuWbO2apZX5t--JTycgsxdMejms4XCbSg0m_jjmWyBJtM3BI3_k0kU4mDDv3rY_XHflcoGnpFOVs3BjhCJOYR0h7BNke4IYrta7XGc88OasbIZ1m-UbMQaPvfeht0t9IgkoQ`
  creds := NewBasicCredentials()
  creds.Add(&url.URL{}, "anything", token)
  rt, _ := client.TransportFor(&client.Config{})
  importCtx := NewContext(rt, kapi.NewContext()).WithCredentials(creds)

  imports := &api.ImageStreamImport{
    Images: []api.ImageImport{
      {From: kapi.ObjectReference{Kind: "DockerImage", Name: "172.30.213.112:5000/default/redis:test"}, Insecure: true},
    },
  }
  NewImageStreamImporter(100, nil).Import(importCtx, imports)
  d := imports.Images[0].ImageImportStatus
  if d.Image == nil || len(d.Image.DockerImageManifest) > 0 || d.Image.DockerImageReference != "172.30.213.112:5000/default/redis:test" || len(d.Image.DockerImageMetadata.ID) == 0 {
    t.Errorf("unexpected object: %#v", d.Image)
  }
  t.Logf("image: %#v\nstatus: %#v", d.Image, d.Status)
}
*/

func TestImportImageDockerHub(t *testing.T) {
	rt, _ := kclient.TransportFor(&kclient.Config{})
	importCtx := importer.NewContext(rt).WithCredentials(importer.NoCredentials)

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

	i := importer.NewImageStreamImporter(importCtx, 3, nil)
	if err := i.Import(gocontext.Background(), imports); err != nil {
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

func TestImportImageQuayIO(t *testing.T) {
	rt, _ := kclient.TransportFor(&kclient.Config{})
	importCtx := importer.NewContext(rt).WithCredentials(importer.NoCredentials)

	imports := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "quay.io/coreos/etcd"}},
			},
		},
	}

	i := importer.NewImageStreamImporter(importCtx, 3, nil)
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
	if d.Status.Status != unversioned.StatusSuccess {
		if d.Status.Reason == "NotV2Registry" {
			t.Skipf("the server did not report as a v2 registry: %#v", d.Status)
		}
		t.Fatalf("unexpected error: %#v", d.Status)
	}
	if d.Image == nil || len(d.Image.DockerImageManifest) == 0 || !strings.HasPrefix(d.Image.DockerImageReference, "quay.io/coreos/etcd@") || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) == 0 {
		t.Errorf("unexpected object: %#v", d.Image)
		s := spew.ConfigState{
			Indent: " ",
			// Extra deep spew.
			DisableMethods: true,
		}
		t.Logf("import: %s", s.Sdump(d))
	}
}

func TestImportImageRedHatRegistry(t *testing.T) {
	rt, _ := kclient.TransportFor(&kclient.Config{})
	importCtx := importer.NewContext(rt).WithCredentials(importer.NoCredentials)

	// test without the client on the context
	imports := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "registry.access.redhat.com/rhel7"}},
			},
		},
	}

	i := importer.NewImageStreamImporter(importCtx, 3, nil)
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
	if d.Image != nil || d.Status.Status != unversioned.StatusFailure || d.Status.Reason != "NotV2Registry" {
		t.Errorf("unexpected object: %#v", d.Status)
	}

	// test with the client on the context
	imports = &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Images: []api.ImageImportSpec{
				{From: kapi.ObjectReference{Kind: "DockerImage", Name: "registry.access.redhat.com/rhel7"}},
			},
		},
	}
	context := gocontext.WithValue(gocontext.Background(), importer.ContextKeyV1RegistryClient, dockerregistry.NewClient(20*time.Second, false))
	importCtx = importer.NewContext(rt).WithCredentials(importer.NoCredentials)
	i = importer.NewImageStreamImporter(importCtx, 3, nil)
	if err := i.Import(context, imports); err != nil {
		t.Fatal(err)
	}

	if imports.Status.Repository != nil {
		t.Errorf("unexpected repository: %#v", imports.Status.Repository)
	}
	if len(imports.Status.Images) != 1 {
		t.Fatalf("unexpected response: %#v", imports.Status.Images)
	}
	d = imports.Status.Images[0]
	if d.Image == nil || len(d.Image.DockerImageManifest) != 0 || d.Image.DockerImageReference != "registry.access.redhat.com/rhel7:latest" || len(d.Image.DockerImageMetadata.ID) == 0 || len(d.Image.DockerImageLayers) != 0 {
		t.Errorf("unexpected object: %#v", d.Status)
		t.Logf("imports: %#v", imports.Status.Images[0].Image)
	}
}
