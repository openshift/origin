package httpcli

import (
	"net/http"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/client"
	"github.com/mesos/mesos-go/api/v1/lib/encoding"
)

func TestPrepareForResponse(t *testing.T) {
	codec := encoding.Codec{
		Type: encoding.MediaType("foo"),
	}
	for ti, tc := range []struct {
		rc           client.ResponseClass
		wantsHeaders []string
		wantsErr     bool
	}{
		{client.ResponseClass(-1), nil, true},
		{client.ResponseClassNoData, []string{"Accept", "foo"}, false},
		{client.ResponseClassSingleton, []string{"Accept", "foo"}, false},
		{client.ResponseClassAuto, []string{"Accept", "foo"}, false},
		{client.ResponseClassStreaming, []string{"Accept", "application/recordio", "Message-Accept", "foo"}, false},
	} {
		opts, err := prepareForResponse(tc.rc, codec)
		if (err != nil) != tc.wantsErr {
			if tc.wantsErr {
				t.Fatalf("test case %d failed: expected error", ti)
			} else {
				t.Fatalf("test case %d failed: unexpected error %v", ti, err)
			}
		} else {
			req := http.Request{Header: http.Header(make(map[string][]string))}
			opts.Apply(&req)
			if wants := len(tc.wantsHeaders) / 2; wants != len(req.Header) {
				t.Fatalf("test case %d failed: header count mismatch, expected %d instead of %d", ti, wants, len(req.Header))
			}
			for i := 0; i < len(tc.wantsHeaders); i++ {
				name := tc.wantsHeaders[i]
				v := req.Header.Get(name)
				i++
				if wants := tc.wantsHeaders[i]; v != wants {
					t.Fatalf("test case %d failed: unexpected value %q for header %q, wanted %q instead", ti, v, name, wants)
				}
			}
		}
	}
}

func TestNewSourceFactory(t *testing.T) {
	for ti, tc := range []struct {
		rc         client.ResponseClass
		wantsNil   bool
		wantsPanic bool
	}{
		{client.ResponseClass(-1), false, true},
		{client.ResponseClassNoData, true, false},
		{client.ResponseClassAuto, false, false},
		{client.ResponseClassSingleton, false, false},
		{client.ResponseClassStreaming, false, false},
	} {
		f := func() encoding.SourceFactoryFunc {
			defer func() {
				if x := recover(); (x != nil) != tc.wantsPanic {
					panic(x)
				}
			}()
			return newSourceFactory(tc.rc)
		}()
		if tc.wantsPanic {
			continue
		}
		if (f == nil) != tc.wantsNil {
			if tc.wantsNil {
				t.Fatalf("test case %d failed: expected nil factory func instead of %v", ti, f)
			} else {
				t.Fatalf("test case %d failed: expected non-nil factory func", ti)
			}
		}
	}
}
