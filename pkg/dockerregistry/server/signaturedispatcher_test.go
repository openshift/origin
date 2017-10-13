package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	registryclient "github.com/openshift/origin/pkg/dockerregistry/server/client"
	registryconfig "github.com/openshift/origin/pkg/dockerregistry/server/configuration"
	registrytest "github.com/openshift/origin/pkg/dockerregistry/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	imagefakeclient "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1/fake"
)

func TestSignatureGet(t *testing.T) {
	installFakeAccessController(t)

	testSignature := imageapiv1.ImageSignature{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sha256:4028782c08eae4a8c9a28bf661c0a8d1c2fc8e19dbaae2b018b21011197e1484@cddeb7006d914716e2728000746a0b23",
		},
		Type:    "atomic",
		Content: []byte("owGbwMvMwMQorp341GLVgXeMpw9kJDFE1LxLq1ZKLsosyUxOzFGyqlbKTEnNK8ksqQSxU/KTs1OLdItS01KLUvOSU5WslHLygeoy8otLrEwNDAz0S1KLS8CEVU4iiFKq1VHKzE1MT0XSnpuYl5kGlNNNyUwHKbFSKs5INDI1szIxMLIwtzBKNrBITUw1SbRItkw0skhKMzMzTDZItEgxTDZKS7ZINbRMSUpMTDVKMjC0SDIyNDA0NLQ0TzU0sTABWVZSWQByVmJJfm5mskJyfl5JYmZeapFCcWZ6XmJJaVE"),
	}

	testImage, err := registrytest.NewImageForManifest("user/app", registrytest.SampleImageManifestSchema1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	testImage.DockerImageManifest = ""
	testImage.Signatures = append(testImage.Signatures, testSignature)

	ctx := context.Background()
	ctx = registrytest.WithTestLogger(ctx, t)

	fos, imageClient := registrytest.NewFakeOpenShiftWithClient(ctx)
	registrytest.AddImageStream(t, fos, "user", "app", map[string]string{
		imageapi.InsecureRepositoryAnnotation: "true",
	})
	registrytest.AddImage(t, fos, testImage, "user", "app", "latest")

	osclient, err := registryclient.NewFakeRegistryClient(imageClient).Client()
	if err != nil {
		t.Fatal(err)
	}

	ctx = withUserClient(ctx, osclient)
	registryApp := NewApp(ctx, registryclient.NewFakeRegistryClient(imageClient), &configuration.Configuration{
		Loglevel: "debug",
		Auth: map[string]configuration.Parameters{
			fakeAuthorizerName: {"realm": fakeAuthorizerName},
		},
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
			"cache": configuration.Parameters{
				"blobdescriptor": "inmemory",
			},
			"delete": configuration.Parameters{
				"enabled": true,
			},
			"maintenance": configuration.Parameters{
				"uploadpurging": map[interface{}]interface{}{
					"enabled": false,
				},
			},
		},
		Middleware: map[string][]configuration.Middleware{
			"registry":   {{Name: "openshift"}},
			"repository": {{Name: "openshift", Options: configuration.Parameters{"dockerregistryurl": "localhost:5000"}}},
			"storage":    {{Name: "openshift"}},
		},
	}, &registryconfig.Configuration{}, nil)
	registryServer := httptest.NewServer(registryApp)
	defer registryServer.Close()

	serverURL, err := url.Parse(registryServer.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}

	url := fmt.Sprintf("http://%s/extensions/v2/user/app/signatures/%s", serverURL.Host, testImage.Name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		t.Fatalf("failed to do the request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status: %v", resp.StatusCode)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if len(content) == 0 {
		t.Fatalf("unexpected empty body")
	}

	var ans signatureList

	if err := json.Unmarshal(content, &ans); err != nil {
		t.Logf("received body: %v", string(content))
		t.Fatalf("failed to parse body: %v", err)
	}

	if len(ans.Signatures) == 0 {
		t.Fatalf("unexpected empty signature list")
	}

	if testSignature.Name != ans.Signatures[0].Name {
		t.Fatalf("unexpected signature: %#v", ans)
	}
}

func TestSignaturePut(t *testing.T) {
	imageClient := &imagefakeclient.FakeImageV1{&clientgotesting.Fake{}}

	installFakeAccessController(t)

	testSignature := signature{
		Version: 2,
		Name:    "sha256:4028782c08eae4a8c9a28bf661c0a8d1c2fc8e19dbaae2b018b21011197e1484@cddeb7006d914716e2728000746a0b23",
		Type:    "atomic",
		Content: []byte("owGbwMvMwMQorp341GLVgXeMpw9kJDFE1LxLq1ZKLsosyUxOzFGyqlbKTEnNK8ksqQSxU/KTs1OLdItS01KLUvOSU5WslHLygeoy8otLrEwNDAz0S1KLS8CEVU4iiFKq1VHKzE1MT0XSnpuYl5kGlNNNyUwHKbFSKs5INDI1szIxMLIwtzBKNrBITUw1SbRItkw0skhKMzMzTDZItEgxTDZKS7ZINbRMSUpMTDVKMjC0SDIyNDA0NLQ0TzU0sTABWVZSWQByVmJJfm5mskJyfl5JYmZeapFCcWZ6XmJJaVE"),
	}
	var newImageSignature *imageapiv1.ImageSignature

	imageClient.AddReactor("create", "imagesignatures", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		sign, ok := action.(clientgotesting.CreateAction).GetObject().(*imageapiv1.ImageSignature)
		if !ok {
			return true, nil, fmt.Errorf("unexpected object received: %#v", sign)
		}
		newImageSignature = sign
		return true, sign, nil
	})

	osclient, err := registryclient.NewFakeRegistryClient(imageClient).Client()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ctx = registrytest.WithTestLogger(ctx, t)
	ctx = withUserClient(ctx, osclient)
	registryApp := NewApp(ctx, registryclient.NewFakeRegistryClient(imageClient), &configuration.Configuration{
		Loglevel: "debug",
		Auth: map[string]configuration.Parameters{
			fakeAuthorizerName: {"realm": fakeAuthorizerName},
		},
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
			"cache": configuration.Parameters{
				"blobdescriptor": "inmemory",
			},
			"delete": configuration.Parameters{
				"enabled": true,
			},
			"maintenance": configuration.Parameters{
				"uploadpurging": map[interface{}]interface{}{
					"enabled": false,
				},
			},
		},
		Middleware: map[string][]configuration.Middleware{
			"registry":   {{Name: "openshift"}},
			"repository": {{Name: "openshift", Options: configuration.Parameters{"dockerregistryurl": "localhost:5000"}}},
			"storage":    {{Name: "openshift"}},
		},
	}, &registryconfig.Configuration{}, nil)
	registryServer := httptest.NewServer(registryApp)
	defer registryServer.Close()

	serverURL, err := url.Parse(registryServer.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}

	signData, err := json.Marshal(testSignature)
	if err != nil {
		t.Fatalf("unable to serialize signature: %v", err)
	}

	url := fmt.Sprintf("http://%s/extensions/v2/user/app/signatures/%s", serverURL.Host, etcdDigest)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(signData))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		t.Fatalf("failed to do the request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected response status: %v", resp.StatusCode)
	}

	if testSignature.Name != newImageSignature.Name {
		t.Errorf("unexpected signature: name %#+v", newImageSignature.Name)
	}
	if testSignature.Type != newImageSignature.Type {
		t.Errorf("unexpected signature type: %#+v", newImageSignature.Type)
	}
	if !reflect.DeepEqual(testSignature.Content, newImageSignature.Content) {
		t.Errorf("unexpected signature content: %#+v", newImageSignature.Content)
	}
}
