package prune

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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/diff"

	restfake "k8s.io/client-go/rest/fake"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	"github.com/openshift/origin/pkg/oc/admin/prune/imageprune/testutil"
	"github.com/openshift/origin/pkg/version"
)

var logLevel = flag.Int("loglevel", 0, "")

func TestImagePruneNamespaced(t *testing.T) {
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	kFake := fake.NewSimpleClientset()
	imageFake := imageclient.NewSimpleClientset()
	opts := &PruneImagesOptions{
		Namespace: "foo",

		AppsClient:  appsclient.NewSimpleClientset().Apps(),
		BuildClient: buildclient.NewSimpleClientset().Build(),
		ImageClient: imageFake.Image(),
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
	flag.Lookup("v").Value.Set(fmt.Sprint(*logLevel))
	podBad := testutil.Pod("foo", "pod1", kapi.PodRunning, "invalid image reference")
	podGood := testutil.Pod("foo", "pod2", kapi.PodRunning, "example.com/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000000")
	dep := testutil.Deployment("foo", "dep1", "do not blame me")
	bcBad := testutil.BC("foo", "bc1", "source", "ImageStreamImage", "foo", "bar:invalid-digest")

	kFake := fake.NewSimpleClientset(&podBad, &podGood, &dep)
	imageFake := imageclient.NewSimpleClientset()
	fakeDiscovery := &fakeVersionDiscovery{
		masterVersion: version.Get(),
	}

	switch d := kFake.Discovery().(type) {
	case *fakediscovery.FakeDiscovery:
		fakeDiscovery.FakeDiscovery = d
	default:
		t.Fatalf("unexpected discovery type: %T != %T", d, &fakediscovery.FakeDiscovery{})
	}

	errBuf := bytes.NewBuffer(make([]byte, 0, 4096))
	opts := &PruneImagesOptions{
		AppsClient:      appsclient.NewSimpleClientset().Apps(),
		BuildClient:     buildclient.NewSimpleClientset(&bcBad).Build(),
		ImageClient:     imageFake.Image(),
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
			`Pod[foo/pod1]: invalid docker image reference "invalid image reference": invalid reference format`,
			`BuildConfig[foo/bc1]: invalid ImageStreamImage reference "bar:invalid-digest": expected exactly one @ in the isimage name "bar:invalid-digest"`,
			`Deployment[foo/dep1]: invalid docker image reference "do not blame me": invalid reference format`)

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
	masterVersion version.Info
}

func (f *fakeVersionDiscovery) RESTClient() restclient.Interface {
	return &restfake.RESTClient{
		NegotiatedSerializer: scheme.Codecs,
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
