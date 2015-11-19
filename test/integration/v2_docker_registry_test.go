// +build integration,etcd

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/docker/libtrust"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func init() {
	testutil.RequireEtcd()
}

func signedManifest(name string) ([]byte, digest.Digest, error) {
	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return []byte{}, "", fmt.Errorf("error generating EC key: %s", err)
	}

	mappingManifest := manifest.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
		Name:         name,
		Tag:          imageapi.DefaultImageTag,
		Architecture: "amd64",
		History: []manifest.History{
			{
				V1Compatibility: `{"id": "foo"}`,
			},
		},
	}

	manifestBytes, err := json.MarshalIndent(mappingManifest, "", "    ")
	if err != nil {
		return []byte{}, "", fmt.Errorf("error marshaling manifest: %s", err)
	}
	dgst, err := digest.FromBytes(manifestBytes)
	if err != nil {
		return []byte{}, "", fmt.Errorf("error calculating manifest digest: %s", err)
	}

	jsonSignature, err := libtrust.NewJSONSignature(manifestBytes)
	if err != nil {
		return []byte{}, "", fmt.Errorf("error creating json signature: %s", err)
	}

	if err = jsonSignature.Sign(key); err != nil {
		return []byte{}, "", fmt.Errorf("error signing manifest: %s", err)
	}

	signedBytes, err := jsonSignature.PrettySignature("signatures")
	if err != nil {
		return []byte{}, "", fmt.Errorf("error invoking PrettySignature: %s", err)
	}

	return signedBytes, dgst, nil
}

func TestV2RegistryGetTags(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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

	config := `version: 0.1
loglevel: debug
http:
  addr: 127.0.0.1:5000
storage:
  inmemory: {}
auth:
  openshift:
middleware:
  repository:
    - name: openshift
`

	os.Setenv("OPENSHIFT_CA_DATA", string(clusterAdminClientConfig.CAData))
	os.Setenv("OPENSHIFT_CERT_DATA", string(clusterAdminClientConfig.CertData))
	os.Setenv("OPENSHIFT_KEY_DATA", string(clusterAdminClientConfig.KeyData))
	os.Setenv("OPENSHIFT_MASTER", clusterAdminClientConfig.Host)
	os.Setenv("REGISTRY_URL", "127.0.0.1:5000")

	go dockerregistry.Execute(strings.NewReader(config))

	stream := imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      "test",
		},
	}
	if _, err := adminClient.ImageStreams(testutil.Namespace()).Create(&stream); err != nil {
		t.Fatalf("error creating image stream: %s", err)
	}

	tags, err := getTags(stream.Name, user, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) > 0 {
		t.Fatalf("expected 0 tags, got: %#v", tags)
	}

	dgst, err := putManifest(stream.Name, user, token)
	if err != nil {
		t.Fatal(err)
	}

	tags, err = getTags(stream.Name, user, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %v", len(tags), tags)
	}
	if tags[0] != imageapi.DefaultImageTag {
		t.Fatalf("expected latest, got %q", tags[0])
	}

	// test get by tag
	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", testutil.Namespace(), stream.Name, imageapi.DefaultImageTag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	req.SetBasicAuth(user, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error retrieving manifest from registry: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	var retrievedManifest manifest.Manifest
	if err := json.Unmarshal(body, &retrievedManifest); err != nil {
		t.Fatalf("error unmarshaling retrieved manifest")
	}
	if retrievedManifest.Name != fmt.Sprintf("%s/%s", testutil.Namespace(), stream.Name) {
		t.Fatalf("unexpected manifest name: %s", retrievedManifest.Name)
	}
	if retrievedManifest.Tag != imageapi.DefaultImageTag {
		t.Fatalf("unexpected manifest tag: %s", retrievedManifest.Tag)
	}

	// test get by digest
	url = fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", testutil.Namespace(), stream.Name, dgst.String())
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	req.SetBasicAuth(user, token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error retrieving manifest from registry: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &retrievedManifest); err != nil {
		t.Fatalf("error unmarshaling retrieved manifest")
	}
	if retrievedManifest.Name != fmt.Sprintf("%s/%s", testutil.Namespace(), stream.Name) {
		t.Fatalf("unexpected manifest name: %s", retrievedManifest.Name)
	}
	if retrievedManifest.Tag != imageapi.DefaultImageTag {
		t.Fatalf("unexpected manifest tag: %s", retrievedManifest.Tag)
	}

	image, err := adminClient.ImageStreamImages(testutil.Namespace()).Get(stream.Name, dgst.String())
	if err != nil {
		t.Fatalf("error getting imageStreamImage: %s", err)
	}
	if e, a := fmt.Sprintf("test@%s", dgst.Hex()[:7]), image.Name; e != a {
		t.Errorf("image name: expected %q, got %q", e, a)
	}
	if e, a := dgst.String(), image.Image.Name; e != a {
		t.Errorf("image name: expected %q, got %q", e, a)
	}
	if e, a := fmt.Sprintf("127.0.0.1:5000/%s/%s@%s", testutil.Namespace(), stream.Name, dgst.String()), image.Image.DockerImageReference; e != a {
		t.Errorf("image dockerImageReference: expected %q, got %q", e, a)
	}
	if e, a := "foo", image.Image.DockerImageMetadata.ID; e != a {
		t.Errorf("image dockerImageMetadata.ID: expected %q, got %q", e, a)
	}

	// test auto provisioning
	otherStream, err := adminClient.ImageStreams(testutil.Namespace()).Get("otherrepo")
	t.Logf("otherStream=%#v, err=%v", otherStream, err)
	if err == nil {
		t.Fatalf("expected error getting otherrepo")
	}

	otherDigest, err := putManifest("otherrepo", user, token)
	if err != nil {
		t.Fatal(err)
	}

	otherStream, err = adminClient.ImageStreams(testutil.Namespace()).Get("otherrepo")
	if err != nil {
		t.Fatalf("unexpected error getting otherrepo: %s", err)
	}
	if otherStream == nil {
		t.Fatalf("unexpected nil otherrepo")
	}
	if len(otherStream.Status.Tags) != 1 {
		t.Errorf("expected 1 tag, got %#v", otherStream.Status.Tags)
	}
	history, ok := otherStream.Status.Tags[imageapi.DefaultImageTag]
	if !ok {
		t.Fatal("unable to find 'latest' tag")
	}
	if len(history.Items) != 1 {
		t.Errorf("expected 1 tag event, got %#v", history.Items)
	}
	if e, a := otherDigest.String(), history.Items[0].Image; e != a {
		t.Errorf("digest: expected %q, got %q", e, a)
	}
}

func putManifest(name, user, token string) (digest.Digest, error) {
	putUrl := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", testutil.Namespace(), name, imageapi.DefaultImageTag)
	signedManifest, dgst, err := signedManifest(fmt.Sprintf("%s/%s", testutil.Namespace(), name))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("PUT", putUrl, bytes.NewReader(signedManifest))
	if err != nil {
		return "", fmt.Errorf("error creating put request: %s", err)
	}
	req.SetBasicAuth(user, token)
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error putting manifest: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected put status code: %d", resp.StatusCode)
	}
	return dgst, nil
}

func getTags(streamName, user, token string) ([]string, error) {
	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/tags/list", testutil.Namespace(), streamName)
	client := http.DefaultClient
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []string{}, fmt.Errorf("error creating request: %v", err)
	}
	req.SetBasicAuth(user, token)
	resp, err := client.Do(req)
	if err != nil {
		return []string{}, fmt.Errorf("error retrieving tags from registry: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []string{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	m := make(map[string]interface{})
	err = json.Unmarshal(body, &m)
	if err != nil {
		return []string{}, fmt.Errorf("error unmarhsaling response %q: %s", body, err)
	}
	arr, ok := m["tags"].([]interface{})
	if !ok {
		return []string{}, fmt.Errorf("couldn't convert tags")
	}
	tags := []string{}
	for _, value := range arr {
		tag, ok := value.(string)
		if !ok {
			return []string{}, fmt.Errorf("tag %#v is not a string", value)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}
