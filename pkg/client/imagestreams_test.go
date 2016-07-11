package client

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	_ "github.com/openshift/origin/pkg/image/api/install"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestImageStreamImportUnsupported(t *testing.T) {
	testCases := []struct {
		status unversioned.Status
		errFn  func(err error) bool
	}{
		{
			status: errors.NewNotFound(api.Resource(""), "").ErrStatus,
			errFn:  func(err error) bool { return err == ErrImageStreamImportUnsupported },
		},
		{
			status: errors.NewNotFound(api.Resource("ImageStreamImport"), "").ErrStatus,
			errFn:  func(err error) bool { return err != ErrImageStreamImportUnsupported && errors.IsNotFound(err) },
		},
		{
			status: errors.NewConflict(api.Resource("ImageStreamImport"), "", nil).ErrStatus,
			errFn:  func(err error) bool { return err != ErrImageStreamImportUnsupported && errors.IsConflict(err) },
		},
		{
			status: errors.NewForbidden(api.Resource("ImageStreamImport"), "", nil).ErrStatus,
			errFn:  func(err error) bool { return err == ErrImageStreamImportUnsupported },
		},
	}
	for i, test := range testCases {
		c, err := New(&restclient.Config{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				buf := bytes.NewBuffer([]byte(runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(api.SchemeGroupVersion), &test.status)))
				return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(buf)}, nil
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ImageStreams("test").Import(&api.ImageStreamImport{}); !test.errFn(err) {
			t.Errorf("%d: error: %v", i, err)
		}
	}
}
