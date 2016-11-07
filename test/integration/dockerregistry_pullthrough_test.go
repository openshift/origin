package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/manifest/schema1"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// gzippedEmptyTar is a gzip-compressed version of an empty tar file
// (1024 NULL bytes)
var gzippedEmptyTar = []byte{
	31, 139, 8, 0, 0, 9, 110, 136, 0, 255, 98, 24, 5, 163, 96, 20, 140, 88,
	0, 8, 0, 0, 255, 255, 46, 175, 181, 239, 0, 4, 0, 0,
}

func runRegistry() error {
	config := `version: 0.1
log:
  level: debug
http:
  addr: 127.0.0.1:5000
storage:
  inmemory: {}
auth:
  openshift:
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: false
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
`
	os.Setenv("DOCKER_REGISTRY_URL", "127.0.0.1:5000")

	go dockerregistry.Execute(strings.NewReader(config))

	if err := cmdutil.WaitForSuccessfulDial(false, "tcp", "127.0.0.1:5000", 100*time.Millisecond, 1*time.Second, 35); err != nil {
		return err
	}
	return nil
}

func testPullThroughGetManifest(stream *imageapi.ImageStreamImport, user, token, urlPart string) error {
	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", stream.Namespace, stream.Name, urlPart)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.SetBasicAuth(user, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error retrieving manifest from registry: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)

	var retrievedManifest schema1.Manifest

	if err := json.Unmarshal(body, &retrievedManifest); err != nil {
		return fmt.Errorf("error unmarshaling retrieved manifest")
	}

	return nil
}

func testPullThroughStatBlob(stream *imageapi.ImageStreamImport, user, token, digest string) error {
	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/blobs/%s", stream.Namespace, stream.Name, digest)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.SetBasicAuth(user, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error retrieving manifest from registry: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if resp.Header.Get("Docker-Content-Digest") != digest {
		return fmt.Errorf("unexpected blob digest: %s (expected %s)", resp.Header.Get("Docker-Content-Digest"), digest)
	}

	return nil
}

func TestPullThroughInsecure(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	user := "admin"
	adminClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, testutil.Namespace(), user)
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}
	token, err := tokencmd.RequestToken(clusterAdminClientConfig, nil, user, "password")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}

	os.Setenv("OPENSHIFT_CA_DATA", string(clusterAdminClientConfig.CAData))
	os.Setenv("OPENSHIFT_CERT_DATA", string(clusterAdminClientConfig.CertData))
	os.Setenv("OPENSHIFT_KEY_DATA", string(clusterAdminClientConfig.KeyData))
	os.Setenv("OPENSHIFT_MASTER", clusterAdminClientConfig.Host)

	// start regular HTTP server
	reponame := "testrepo"
	repotag := "testtag"
	isname := "test/" + reponame
	countStat := 0

	descriptors := map[string]int64{
		"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4": 3000,
		"sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa": 200,
		"sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4": 10,
	}
	imageSize := int64(0)
	for _, size := range descriptors {
		imageSize += size
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("External registry got %s %s", r.Method, r.URL.Path)

		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

		switch r.URL.Path {
		case "/v2/":
			w.Write([]byte(`{}`))
		case "/v2/" + isname + "/tags/list":
			w.Write([]byte("{\"name\": \"" + isname + "\", \"tags\": [\"latest\", \"" + repotag + "\"]}"))
		case "/v2/" + isname + "/manifests/latest", "/v2/" + isname + "/manifests/" + repotag, "/v2/" + isname + "/manifests/" + etcdDigest:
			if r.Method == "HEAD" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(etcdManifest)))
				w.Header().Set("Docker-Content-Digest", etcdDigest)
				w.WriteHeader(http.StatusOK)
			} else {
				w.Write([]byte(etcdManifest))
			}
		default:
			if strings.HasPrefix(r.URL.Path, "/v2/"+isname+"/blobs/") {
				for dgst, size := range descriptors {
					if r.URL.Path != "/v2/"+isname+"/blobs/"+dgst {
						continue
					}
					if r.Method == "HEAD" {
						w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
						w.Header().Set("Docker-Content-Digest", dgst)
						w.WriteHeader(http.StatusOK)
						countStat++
						return
					}
					w.Write(gzippedEmptyTar)
					return
				}
			}
			t.Fatalf("unexpected request %s: %#v", r.URL.Path, r)
		}
	}))
	srvurl, _ := url.Parse(server.URL)

	stream := imageapi.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      "myimagestream",
			Annotations: map[string]string{
				imageapi.InsecureRepositoryAnnotation: "true",
			},
		},
		Spec: imageapi.ImageStreamImportSpec{
			Import: true,
			Images: []imageapi.ImageImportSpec{
				{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: srvurl.Host + "/" + isname + ":" + repotag,
					},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				},
			},
		},
	}

	isi, err := adminClient.ImageStreams(testutil.Namespace()).Import(&stream)
	if err != nil {
		t.Fatal(err)
	}

	if len(isi.Status.Images) != 1 {
		t.Fatalf("imported unexpected number of images (%d != 1)", len(isi.Status.Images))
	}
	for i, image := range isi.Status.Images {
		if image.Status.Status != unversioned.StatusSuccess {
			t.Fatalf("unexpected status %d: %#v", i, image.Status)
		}

		if image.Image == nil {
			t.Fatalf("unexpected empty image %d", i)
		}

		// the image name is always the sha256, and size is calculated
		if image.Image.Name != etcdDigest {
			t.Fatalf("unexpected image %d: %#v (expect %q)", i, image.Image.Name, etcdDigest)
		}
	}

	istream, err := adminClient.ImageStreams(stream.Namespace).Get(stream.Name)
	if err != nil {
		t.Fatal(err)
	}

	if istream.Annotations == nil {
		istream.Annotations = make(map[string]string)
	}
	istream.Annotations[imageapi.InsecureRepositoryAnnotation] = "true"

	_, err = adminClient.ImageStreams(istream.Namespace).Update(istream)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Run registry...")
	if err := runRegistry(); err != nil {
		t.Fatal(err)
	}

	t.Logf("Run testPullThroughGetManifest with tag...")
	if err := testPullThroughGetManifest(&stream, user, token, repotag); err != nil {
		t.Fatal(err)
	}

	t.Logf("Run testPullThroughGetManifest with digest...")
	if err := testPullThroughGetManifest(&stream, user, token, etcdDigest); err != nil {
		t.Fatal(err)
	}

	t.Logf("Run testPullThroughStatBlob (%s == true)...", imageapi.InsecureRepositoryAnnotation)
	for digest := range descriptors {
		if err := testPullThroughStatBlob(&stream, user, token, digest); err != nil {
			t.Fatal(err)
		}
	}

	istream, err = adminClient.ImageStreams(stream.Namespace).Get(stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	istream.Annotations[imageapi.InsecureRepositoryAnnotation] = "false"

	_, err = adminClient.ImageStreams(istream.Namespace).Update(istream)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Run testPullThroughStatBlob (%s == false)...", imageapi.InsecureRepositoryAnnotation)
	for digest := range descriptors {
		if err := testPullThroughStatBlob(&stream, user, token, digest); err == nil {
			t.Fatal("unexpexted access to insecure blobs")
		}
	}
}
