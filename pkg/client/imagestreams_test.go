package client

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	_ "k8s.io/kubernetes/pkg/api/install"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	_ "github.com/openshift/origin/pkg/image/apis/image/install"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestImageStreamImportUnsupported(t *testing.T) {
	testCases := []struct {
		status metav1.Status
		errFn  func(err error) bool
	}{
		{
			status: errors.NewNotFound(imageapi.Resource(""), "").ErrStatus,
			errFn:  func(err error) bool { return err == ErrImageStreamImportUnsupported },
		},
		{
			status: errors.NewNotFound(imageapi.Resource("ImageStreamImport"), "").ErrStatus,
			errFn:  func(err error) bool { return err != ErrImageStreamImportUnsupported && errors.IsNotFound(err) },
		},
		{
			status: errors.NewConflict(imageapi.Resource("ImageStreamImport"), "", nil).ErrStatus,
			errFn:  func(err error) bool { return err != ErrImageStreamImportUnsupported && errors.IsConflict(err) },
		},
		{
			status: errors.NewForbidden(imageapi.Resource("ImageStreamImport"), "", nil).ErrStatus,
			errFn:  func(err error) bool { return err == ErrImageStreamImportUnsupported },
		},
	}
	for i, test := range testCases {
		c, err := New(&restclient.Config{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				buf := bytes.NewBuffer([]byte(runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(imageapi.SchemeGroupVersion), &test.status)))
				return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(buf)}, nil
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ImageStreams("test").Import(&imageapi.ImageStreamImport{}); !test.errFn(err) {
			t.Errorf("%d: error: %v", i, err)
		}
	}
}
