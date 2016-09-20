package clientcmd

import (
	"io"
	"net/http"
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
)

type fakeLimiter struct {
	FakeSaturation float64
	FakeQPS        float32
}

func (t *fakeLimiter) TryAccept() bool {
	return true
}

func (t *fakeLimiter) Saturation() float64 {
	return t.FakeSaturation
}

func (t *fakeLimiter) QPS() float32 {
	return t.FakeQPS
}

func (t *fakeLimiter) Stop() {}

func (t *fakeLimiter) Accept() {}

type fakeCodec struct{}

func (c *fakeCodec) Decode([]byte, *unversioned.GroupVersionKind, runtime.Object) (runtime.Object, *unversioned.GroupVersionKind, error) {
	return nil, nil, nil
}

func (c *fakeCodec) Encode(obj runtime.Object, stream io.Writer) error {
	return nil
}

type fakeRoundTripper struct{}

func (r *fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

var fakeWrapperFunc = func(http.RoundTripper) http.RoundTripper {
	return &fakeRoundTripper{}
}

type fakeNegotiatedSerializer struct{}

func (n *fakeNegotiatedSerializer) SupportedMediaTypes() []string {
	return []string{}
}

func (n *fakeNegotiatedSerializer) SerializerForMediaType(mediaType string, params map[string]string) (s runtime.SerializerInfo, ok bool) {
	return runtime.SerializerInfo{}, true
}

func (n *fakeNegotiatedSerializer) SupportedStreamingMediaTypes() []string {
	return []string{}
}

func (n *fakeNegotiatedSerializer) StreamingSerializerForMediaType(mediaType string, params map[string]string) (s runtime.StreamSerializerInfo, ok bool) {
	return runtime.StreamSerializerInfo{}, true
}

func (n *fakeNegotiatedSerializer) EncoderForVersion(serializer runtime.Encoder, gv runtime.GroupVersioner) runtime.Encoder {
	return &fakeCodec{}
}

func (n *fakeNegotiatedSerializer) DecoderToVersion(serializer runtime.Decoder, gv runtime.GroupVersioner) runtime.Decoder {
	return &fakeCodec{}
}

func TestAnonymousConfig(t *testing.T) {
	f := fuzz.New().NilChance(0.0).NumElements(1, 1)
	f.Funcs(
		func(r *runtime.Codec, f fuzz.Continue) {
			codec := &fakeCodec{}
			f.Fuzz(codec)
			*r = codec
		},
		func(r *http.RoundTripper, f fuzz.Continue) {
			roundTripper := &fakeRoundTripper{}
			f.Fuzz(roundTripper)
			*r = roundTripper
		},
		func(fn *func(http.RoundTripper) http.RoundTripper, f fuzz.Continue) {
			*fn = fakeWrapperFunc
		},
		func(r *runtime.NegotiatedSerializer, f fuzz.Continue) {
			serializer := &fakeNegotiatedSerializer{}
			f.Fuzz(serializer)
			*r = serializer
		},
		func(r *flowcontrol.RateLimiter, f fuzz.Continue) {
			limiter := &fakeLimiter{}
			f.Fuzz(limiter)
			*r = limiter
		},
		// Authentication does not require fuzzer
		func(r *restclient.AuthProviderConfigPersister, f fuzz.Continue) {},
		func(r *api.AuthProviderConfig, f fuzz.Continue) {
			r.Config = map[string]string{}
		},
	)
	for i := 0; i < 20; i++ {
		original := &restclient.Config{}
		f.Fuzz(original)
		actual := AnonymousClientConfig(original)
		expected := *original

		// this is the list of known security related fields, add to this list if a new field
		// is added to restclient.Config, update AnonymousClientConfig to preserve the field otherwise.
		expected.Impersonate = ""
		expected.BearerToken = ""
		expected.Username = ""
		expected.Password = ""
		expected.AuthProvider = nil
		expected.AuthConfigPersister = nil
		expected.TLSClientConfig.CertData = nil
		expected.TLSClientConfig.CertFile = ""
		expected.TLSClientConfig.KeyData = nil
		expected.TLSClientConfig.KeyFile = ""

		// The DeepEqual cannot handle the func comparison, so we just verify if the
		// function return the expected object.
		if actual.WrapTransport == nil || !reflect.DeepEqual(expected.WrapTransport(nil), &fakeRoundTripper{}) {
			t.Fatalf("AnonymousClientConfig dropped the WrapTransport field")
		} else {
			actual.WrapTransport = nil
			expected.WrapTransport = nil
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("AnonymousClientConfig dropped unexpected fields, identify whether they are security related or not: %s", diff.ObjectGoPrintDiff(expected, actual))
		}
	}
}
