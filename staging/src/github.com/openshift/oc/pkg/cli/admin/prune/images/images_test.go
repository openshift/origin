package images

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/api"
	fakeappsclient "github.com/openshift/client-go/apps/clientset/versioned/fake"
	fakeappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1/fake"
	fakebuildclient "github.com/openshift/client-go/build/clientset/versioned/fake"
	fakebuildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1/fake"
	fakeimageclient "github.com/openshift/client-go/image/clientset/versioned/fake"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1/fake"
	imagetest "github.com/openshift/oc/pkg/helpers/image/test"
)

var logLevel = flag.Int("loglevel", 0, "")

func TestImagePruneNamespaced(t *testing.T) {
	var level klog.Level
	level.Set(fmt.Sprint(*logLevel))

	kFake := fakekubernetes.NewSimpleClientset()
	imageFake := &fakeimagev1client.FakeImageV1{Fake: &(fakeimageclient.NewSimpleClientset().Fake)}
	opts := &PruneImagesOptions{
		Namespace: "foo",

		AppsClient:  &fakeappsv1client.FakeAppsV1{Fake: &(fakeappsclient.NewSimpleClientset().Fake)},
		BuildClient: &fakebuildv1client.FakeBuildV1{Fake: &(fakebuildclient.NewSimpleClientset().Fake)},
		ImageClient: imageFake,
		KubeClient:  kFake,
		Out:         ioutil.Discard,
		ErrOut:      os.Stderr,
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(imageFake.Actions()) == 0 || len(kFake.Actions()) == 0 {
		t.Errorf("Missing get images actions")
	}
	for _, a := range imageFake.Actions() {
		// images are non-namespaced
		if a.GetResource().Resource == "images" {
			continue
		}
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
	for _, a := range kFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
}

func TestImagePruneErrOnBadReference(t *testing.T) {
	var level klog.Level
	level.Set(fmt.Sprint(*logLevel))

	podBad := imagetest.Pod("foo", "pod1", corev1.PodRunning, "invalid image reference")
	podGood := imagetest.Pod("foo", "pod2", corev1.PodRunning, "example.com/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")
	dep := imagetest.Deployment("foo", "dep1", "do not blame me")
	bcBad := imagetest.BC("foo", "bc1", "source", "ImageStreamImage", "foo", "bar:invalid-digest")

	kFake := fakekubernetes.NewSimpleClientset(&podBad, &podGood, &dep)
	imageFake := &fakeimagev1client.FakeImageV1{Fake: &(fakeimageclient.NewSimpleClientset().Fake)}
	fakeDiscovery := &fakeVersionDiscovery{
		masterVersion: version.Info{},
	}

	// we need to install OpenShift API types to kubectl's scheme for GetReference to work
	api.Install(scheme.Scheme)

	switch d := kFake.Discovery().(type) {
	case *fakediscovery.FakeDiscovery:
		fakeDiscovery.FakeDiscovery = d
	default:
		t.Fatalf("unexpected discovery type: %T != %T", d, &fakediscovery.FakeDiscovery{})
	}

	errBuf := bytes.NewBuffer(make([]byte, 0, 4096))
	opts := &PruneImagesOptions{
		AppsClient:      &fakeappsv1client.FakeAppsV1{Fake: &(fakeappsclient.NewSimpleClientset().Fake)},
		BuildClient:     &fakebuildv1client.FakeBuildV1{Fake: &(fakebuildclient.NewSimpleClientset(&bcBad).Fake)},
		ImageClient:     imageFake,
		KubeClient:      kFake,
		DiscoveryClient: fakeDiscovery,
		Timeout:         time.Second,
		Out:             ioutil.Discard,
		ErrOut:          errBuf,
	}

	verifyOutput := func(out string, expectClientVersionMismatch bool) {
		t.Logf("pruner error output: %s\n", out)

		badRefErrors := sets.NewString()
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "  ") {
				badRefErrors.Insert(l[2:])
			}
		}
		expBadRefErrors := sets.NewString(
			`Pod[foo/pod1]: invalid container image reference "invalid image reference": invalid reference format`,
			`BuildConfig[foo/bc1]: invalid ImageStreamImage reference "bar:invalid-digest": expected exactly one @ in the isimage name "bar:invalid-digest"`,
			`Deployment[foo/dep1]: invalid container image reference "do not blame me": invalid reference format`)

		if a, e := badRefErrors, expBadRefErrors; !a.Equal(e) {
			t.Fatalf("got unexpected invalid reference errors: %s", diff.ObjectDiff(a, e))
		}

		if expectClientVersionMismatch {
			if msg := "client version"; !strings.Contains(strings.ToLower(out), msg) {
				t.Errorf("expected message %q is not contained in the output", msg)
			}
		} else {
			for _, msg := range []string{"failed to get master api version", "client version"} {
				if strings.Contains(strings.ToLower(out), msg) {
					t.Errorf("got unexpected message %q in the output", msg)
				}
			}
		}
	}

	err := opts.Run()
	if err == nil {
		t.Fatal("Unexpected non-error")
	}

	t.Logf("pruner error: %s\n", err)
	verifyOutput(errBuf.String(), false)

	t.Logf("bump master version and try again")
	fakeDiscovery.masterVersion.Minor += "1"
	errBuf.Reset()
	err = opts.Run()
	if err == nil {
		t.Fatal("Unexpected non-error")
	}

	t.Logf("pruner error: %s\n", err)
	verifyOutput(errBuf.String(), true)
}

type fakeVersionDiscovery struct {
	*fakediscovery.FakeDiscovery
	masterVersion apimachineryversion.Info
}

func (f *fakeVersionDiscovery) RESTClient() restclient.Interface {
	return &restfake.RESTClient{
		NegotiatedSerializer: kubernetesscheme.Codecs,
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/version/openshift" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
				}, nil
			}
			header := http.Header{}
			header.Set("Content-Type", runtime.ContentTypeJSON)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       objBody(&f.masterVersion),
			}, nil
		}),
	}
}

func objBody(object interface{}) io.ReadCloser {
	output, err := json.MarshalIndent(object, "", "")
	if err != nil {
		panic(err)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
}

func TestValidateRegistryURL(t *testing.T) {
	for _, tc := range []struct {
		input               string
		expectedError       bool
		expectedErrorString string
	}{
		{input: "172.30.30.30:5000"},
		{input: ":5000"},
		{input: "[fd12:3456:789a:1::1]:80/"},
		{input: "[fd12:3456:789a:1::1]:80"},
		{input: "http://172.30.30.30:5000"},
		{input: "http://[fd12:3456:789a:1::1]:5000/"},
		{input: "http://[fd12:3456:789a:1::1]:5000"},
		{input: "http://registry.org:5000"},
		{input: "https://172.30.30.30:5000"},
		{input: "https://:80/"},
		{input: "https://[fd12:3456:789a:1::1]/"},
		{input: "https://[fd12:3456:789a:1::1]"},
		{input: "https://[fd12:3456:789a:1::1]:5000/"},
		{input: "https://[fd12:3456:789a:1::1]:5000"},
		{input: "https://registry.org/"},
		{input: "https://registry.org"},
		{input: "localhost/"},
		{input: "localhost"},
		{input: "localhost:80"},
		{input: "registry.org/"},
		{input: "registry.org"},
		{input: "registry.org:5000"},

		{
			input:               "httpss://registry.org",
			expectedErrorString: "unsupported scheme: httpss",
		},
		{
			input:               "ftp://registry.org",
			expectedErrorString: "unsupported scheme: ftp",
		},
		{
			input:               "http://registry.org://",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:               "http://registry.org/path",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:         "[fd12:3456:789a:1::1",
			expectedError: true,
		},
		{
			input:         "bad url",
			expectedError: true,
		},
		{
			input:               "/registry.org",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:               "https:///",
			expectedErrorString: errRegistryURLHostEmpty.Error(),
		},
		{
			input:               "http://registry.org?parm=arg",
			expectedErrorString: errNoRegistryURLQueryAllowed.Error(),
		},
	} {

		err := validateRegistryURL(tc.input)
		if err != nil {
			if len(tc.expectedErrorString) > 0 && err.Error() != tc.expectedErrorString {
				t.Errorf("[%s] unexpected error string: %q != %q", tc.input, err.Error(), tc.expectedErrorString)
			} else if len(tc.expectedErrorString) == 0 && !tc.expectedError {
				t.Errorf("[%s] unexpected error: %q", tc.input, err.Error())
			}
		} else if len(tc.expectedErrorString) > 0 {
			t.Errorf("[%s] got non-error while expecting %q", tc.input, tc.expectedErrorString)
		} else if tc.expectedError {
			t.Errorf("[%s] got unexpected non-error", tc.input)
		}
	}
}
