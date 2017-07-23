package integration

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	build "github.com/openshift/origin/pkg/build/apis/build"
	buildv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// expectedIndex contains the routes expected at the api root /. Keep them sorted.
var expectedIndex = []string{
	"/api",
	"/api/v1",
	"/apis",
	"/apis/",
	"/apis/apiextensions.k8s.io",
	"/apis/apiextensions.k8s.io/v1beta1",
	"/apis/apiregistration.k8s.io",
	"/apis/apiregistration.k8s.io/v1beta1",
	"/apis/apps",
	"/apis/apps.openshift.io",
	"/apis/apps.openshift.io/v1",
	"/apis/apps/v1beta1",
	"/apis/authentication.k8s.io",
	"/apis/authentication.k8s.io/v1",
	"/apis/authentication.k8s.io/v1beta1",
	"/apis/authorization.k8s.io",
	"/apis/authorization.k8s.io/v1",
	"/apis/authorization.k8s.io/v1beta1",
	"/apis/authorization.openshift.io",
	"/apis/authorization.openshift.io/v1",
	"/apis/autoscaling",
	"/apis/autoscaling/v1",
	"/apis/batch",
	"/apis/batch/v1",
	"/apis/batch/v2alpha1",
	"/apis/build.openshift.io",
	"/apis/build.openshift.io/v1",
	"/apis/certificates.k8s.io",
	"/apis/certificates.k8s.io/v1beta1",
	"/apis/extensions",
	"/apis/extensions/v1beta1",
	"/apis/image.openshift.io",
	"/apis/image.openshift.io/v1",
	"/apis/network.openshift.io",
	"/apis/network.openshift.io/v1",
	"/apis/networking.k8s.io",
	"/apis/networking.k8s.io/v1",
	"/apis/oauth.openshift.io",
	"/apis/oauth.openshift.io/v1",
	"/apis/policy",
	"/apis/policy/v1beta1",
	"/apis/project.openshift.io",
	"/apis/project.openshift.io/v1",
	"/apis/quota.openshift.io",
	"/apis/quota.openshift.io/v1",
	"/apis/rbac.authorization.k8s.io",
	"/apis/rbac.authorization.k8s.io/v1beta1",
	"/apis/route.openshift.io",
	"/apis/route.openshift.io/v1",
	"/apis/security.openshift.io",
	"/apis/security.openshift.io/v1",
	"/apis/storage.k8s.io",
	"/apis/storage.k8s.io/v1",
	"/apis/storage.k8s.io/v1beta1",
	"/apis/template.openshift.io",
	"/apis/template.openshift.io/v1",
	"/apis/user.openshift.io",
	"/apis/user.openshift.io/v1",
	"/controllers",
	"/healthz",
	"/healthz/autoregister-completion",
	"/healthz/ping",
	"/healthz/poststarthook/apiservice-registration-controller",
	"/healthz/poststarthook/apiservice-status-available-controller",
	"/healthz/poststarthook/bootstrap-controller",
	"/healthz/poststarthook/ca-registration",
	// "/healthz/poststarthook/extensions/third-party-resources",  // Do not enable this controller, we do not support it
	"/healthz/poststarthook/generic-apiserver-start-informers",
	"/healthz/poststarthook/kube-apiserver-autoregistration",
	"/healthz/poststarthook/start-apiextensions-controllers",
	"/healthz/poststarthook/start-apiextensions-informers",
	"/healthz/poststarthook/start-kube-aggregator-informers",
	"/healthz/ready",
	"/metrics",
	"/oapi",
	"/oapi/v1",
	"/swagger-2.0.0.json",
	"/swagger-2.0.0.pb-v1",
	"/swagger-2.0.0.pb-v1.gz",
	"/swagger.json",
	"/swaggerapi",
	"/version",
	"/version/openshift",
}

func TestRootRedirect(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Expected %s, got %s", "application/json", resp.Header.Get("Content-Type"))
	}
	type result struct {
		Paths []string
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Unexpected error reading the body: %v", err)
	}
	var got result
	json.Unmarshal(body, &got)
	sort.Strings(got.Paths)
	if !reflect.DeepEqual(got.Paths, expectedIndex) {
		t.Fatalf("Unexpected index: \ngot=%v,\n\n expected=%v", got, expectedIndex)
	}

	req, err = http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "text/html")
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if resp.Header.Get("Location") != masterConfig.AssetConfig.PublicURL {
		t.Errorf("Expected %s, got %s", masterConfig.AssetConfig.PublicURL, resp.Header.Get("Location"))
	}

	// TODO add a test for when asset config is nil, the redirect should not occur in this case even when
	// accept header contains text/html
}

func TestWellKnownOAuth(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+"/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Unexpected error reading the body: %v", err)
	}
	if !strings.Contains(string(body), "authorization_endpoint") {
		t.Fatal("Expected \"authorization_endpoint\" in the body.")
	}
}

func TestWellKnownOAuthOff(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterConfig.OAuthConfig = nil
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, err := anonymousHttpTransport(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+"/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

var preferredVersions = map[string]string{
	"":                          "v1",
	"apps":                      "v1beta1",
	"apiextensions.k8s.io":      "v1beta1",
	"apiregistration.k8s.io":    "v1beta1",
	"authentication.k8s.io":     "v1",
	"authorization.k8s.io":      "v1",
	"autoscaling":               "v1",
	"batch":                     "v1",
	"certificates.k8s.io":       "v1beta1",
	"extensions":                "v1beta1",
	"networking.k8s.io":         "v1",
	"policy":                    "v1beta1",
	"rbac.authorization.k8s.io": "v1beta1",
	"storage.k8s.io":            "v1",

	"apps.openshift.io":          "v1",
	"authorization.openshift.io": "v1",
	"build.openshift.io":         "v1",
	"image.openshift.io":         "v1",
	"network.openshift.io":       "v1",
	"oauth.openshift.io":         "v1",
	"project.openshift.io":       "v1",
	"quota.openshift.io":         "v1",
	"route.openshift.io":         "v1",
	"security.openshift.io":      "v1",
	"template.openshift.io":      "v1",
	"user.openshift.io":          "v1",
}

func TestApiGroupPreferredVersions(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterConfig.OAuthConfig = nil
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kclientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Looking for build api group in server group discovery")
	groups, err := kclientset.Discovery().ServerGroups()
	if err != nil {
		t.Fatalf("unexpected group discovery error: %v", err)
	}

	found := sets.NewString()
	for _, g := range groups.Groups {
		found.Insert(g.Name)
		preferred, found := preferredVersions[g.Name]
		if !found {
			t.Errorf("Unexpected group %q in discovery", g.Name)
			continue
		}

		if g.PreferredVersion.Version != preferred {
			t.Errorf("Unexpected preferred version for group %q: got %q, expected %q", g.Name, g.PreferredVersion.Version, preferred)
		}
	}

	for g := range preferredVersions {
		if !found.Has(g) {
			t.Errorf("Didn't see group %q in discovery", g)
		}
	}
}

func TestApiGroups(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterConfig.OAuthConfig = nil
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kclientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Looking for build api group in server group discovery")
	groups, err := kclientset.Discovery().ServerGroups()
	if err != nil {
		t.Fatalf("unexpected group discovery error: %v", err)
	}
	found := false
	for _, g := range groups.Groups {
		if g.Name == buildv1.GroupName {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected to find api group %q in discovery, got: %+v", buildv1.GroupName, groups)
	}

	t.Logf("Looking for builds resource in resource discovery")
	resources, err := kclientset.Discovery().ServerResourcesForGroupVersion(buildv1.SchemeGroupVersion.String())
	if err != nil {
		t.Fatalf("unexpected resource discovery error: %v", err)
	}
	found = false
	got := []string{}
	for _, r := range resources.APIResources {
		got = append(got, r.Name)
	}
	sort.Strings(got)
	expected := []string{
		"buildconfigs",
		"buildconfigs/instantiate",
		"buildconfigs/instantiatebinary",
		"buildconfigs/webhooks",
		"builds",
		"builds/clone",
		"builds/details",
		"builds/log",
	}
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Expected different build resources: got=%v, expect=%v", got, expected)
	}

	ns := testutil.RandomNamespace("testapigroup")
	t.Logf("Creating test namespace %q", ns)
	err = testutil.CreateNamespace(clusterAdminKubeConfig, ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer kclientset.Core().Namespaces().Delete(ns, &metav1.DeleteOptions{})

	t.Logf("GETting builds")
	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+fmt.Sprintf("/apis/%s/%s", buildv1.GroupName, buildv1.SchemeGroupVersion.Version), nil)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Client.Transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected GET error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	t.Logf("Creating a Build")
	originalBuild := testBuild()
	_, err = client.Builds(ns).Create(originalBuild)
	if err != nil {
		t.Fatalf("Unexpected BuildConfig create error: %v", err)
	}

	t.Logf("GETting builds again")
	req, err = http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL+fmt.Sprintf("/apis/%s/%s/namespaces/%s/builds/%s", buildv1.GroupName, buildv1.SchemeGroupVersion.Version, ns, originalBuild.Name), nil)
	req.Header.Set("Accept", "*/*")
	resp, err = client.Client.Transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected GET error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	codec := kapi.Codecs.LegacyCodec(buildv1.SchemeGroupVersion)
	respBuild := &buildv1.Build{}
	gvk := buildv1.SchemeGroupVersion.WithKind("Build")
	respObj, _, err := codec.Decode(body, &gvk, respBuild)
	if err != nil {
		t.Fatalf("Unexpected conversion error, body=%q: %v", string(body), err)
	}
	respBuild, ok := respObj.(*buildv1.Build)
	if !ok {
		t.Fatalf("Unexpected type %T, expected buildv1.Build", respObj)
	}
	if got, expected := respBuild.APIVersion, buildv1.SchemeGroupVersion.String(); got != expected {
		t.Fatalf("Unexpected APIVersion: got=%q, expected=%q", got, expected)
	}
	if got, expected := respBuild.Name, originalBuild.Name; got != expected {
		t.Fatalf("Unexpected name: got=%q, expected=%q", got, expected)
	}
}

func anonymousHttpTransport(clusterAdminKubeConfig string) (*http.Transport, error) {
	restConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(restConfig.TLSClientConfig.CAData); !ok {
		return nil, errors.New("failed to add server CA certificates to client pool")
	}
	return knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{
			// only use RootCAs from client config, especially no client certs
			RootCAs: pool,
		},
	}), nil
}

func testBuild() *build.Build {
	return &build.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: build.BuildSpec{
			CommonSpec: build.CommonSpec{
				Source: build.BuildSource{
					Git: &build.GitBuildSource{
						URI: "git://github.com/openshift/ruby-hello-world.git",
					},
					ContextDir: "contextimage",
				},
				Strategy: build.BuildStrategy{
					DockerStrategy: &build.DockerBuildStrategy{},
				},
				Output: build.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "test-image-trigger-repo:outputtag",
					},
				},
			},
		},
	}
}
