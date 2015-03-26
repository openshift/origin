// +build integration,!no-etcd

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

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/docker/libtrust"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func signedManifest() ([]byte, digest.Digest, error) {
	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return []byte{}, "", fmt.Errorf("error generating EC key: %s", err)
	}

	mappingManifest := manifest.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
		Name:         "test-integration/test",
		Tag:          "latest",
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
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	config := `version: 0.1
loglevel: debug
http:
  addr: 127.0.0.1:5000
storage:
  inmemory: {}
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
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(&stream); err != nil {
		t.Fatalf("error creating image stream: %s", err)
	}

	tags, err := getTags(stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) > 0 {
		t.Fatalf("expected 0 tags, got: %#v", tags)
	}

	putUrl := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", testutil.Namespace(), stream.Name, "latest")
	signedManifest, dgst, err := signedManifest()
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("PUT", putUrl, bytes.NewReader(signedManifest))
	if err != nil {
		t.Fatalf("error creating put request: %s", err)
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error putting manifest: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected put status code: %d", resp.StatusCode)
	}

	tags, err = getTags(stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %v", len(tags), tags)
	}
	if tags[0] != "latest" {
		t.Fatalf("expected latest, got %q", tags[0])
	}

	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/manifests/%s", testutil.Namespace(), stream.Name, dgst.String())
	resp, err = http.Get(url)
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
	if retrievedManifest.Name != "test-integration/test" {
		t.Fatalf("unexpected manifest name: %s", retrievedManifest.Name)
	}
	if retrievedManifest.Tag != "latest" {
		t.Fatalf("unexpected manifest tag: %s", retrievedManifest.Tag)
	}

	image, err := clusterAdminClient.ImageStreamImages(testutil.Namespace()).Get(stream.Name, dgst.String())
	if err != nil {
		t.Fatalf("error getting imageStreamImage: %s", err)
	}
	if e, a := dgst.String(), image.Name; e != a {
		t.Errorf("image name: expected %q, got %q", e, a)
	}
	if e, a := fmt.Sprintf("127.0.0.1:5000/%s/%s@%s", testutil.Namespace(), stream.Name, dgst.String()), image.DockerImageReference; e != a {
		t.Errorf("image dockerImageReference: expected %q, got %q", e, a)
	}
	if e, a := "foo", image.DockerImageMetadata.ID; e != a {
		t.Errorf("image dockerImageMetadata.ID: expected %q, got %q", e, a)
	}
}

func getTags(repoName string) ([]string, error) {
	url := fmt.Sprintf("http://127.0.0.1:5000/v2/%s/%s/tags/list", testutil.Namespace(), repoName)
	resp, err := http.Get(url)
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
