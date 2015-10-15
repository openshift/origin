package buildconfig

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/util/rest"
)

type buildConfigInstantiator struct {
	Build   *api.Build
	Err     error
	Request *api.BuildRequest
}

func (i *buildConfigInstantiator) Instantiate(namespace string, request *api.BuildRequest) (*api.Build, error) {
	i.Request = request
	return i.Build, i.Err
}

type plugin struct {
	Secret, Path string
	Err          error
}

func (p *plugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, bool, error) {
	p.Secret, p.Path = secret, path
	return nil, true, p.Err
}

func newStorage() (*rest.WebHook, *buildConfigInstantiator, *test.BuildConfigRegistry) {
	mockRegistry := &test.BuildConfigRegistry{}
	bci := &buildConfigInstantiator{}
	hook := NewWebHookREST(mockRegistry, bci, map[string]webhook.Plugin{
		"ok":        &plugin{},
		"errsecret": &plugin{Err: webhook.ErrSecretMismatch},
		"errhook":   &plugin{Err: webhook.ErrHookNotEnabled},
		"err":       &plugin{Err: fmt.Errorf("test error")},
	})
	return hook, bci, mockRegistry
}

func TestNewWebHook(t *testing.T) {
	hook, _, _ := newStorage()
	if out, ok := hook.New().(*unversioned.Status); !ok {
		t.Errorf("unexpected new: %#v", out)
	}
}

func TestConnectWebHook(t *testing.T) {
	testCases := map[string]struct {
		Name   string
		Path   string
		Obj    *api.BuildConfig
		RegErr error
		ErrFn  func(error) bool
		WFn    func(*httptest.ResponseRecorder) bool
	}{
		"hook returns generic error": {
			Name: "test",
			Path: "secret/err/extra",
			ErrFn: func(err error) bool {
				return strings.Contains(err.Error(), "Internal error occurred: hook failed: test error")
			},
		},
		"hook returns unauthorized for bad secret": {
			Name:  "test",
			Path:  "secret/errsecret/extra",
			ErrFn: errors.IsUnauthorized,
		},
		"hook returns unauthorized for bad hook": {
			Name:  "test",
			Path:  "secret/errhook/extra",
			ErrFn: errors.IsUnauthorized,
		},
		"hook returns unauthorized for missing build config": {
			Name:   "test",
			Path:   "secret/errhook/extra",
			RegErr: fmt.Errorf("any old error"),
			ErrFn:  errors.IsUnauthorized,
		},
		"hook returns 200 for ok hook": {
			Name:  "test",
			Path:  "secret/ok/extra",
			Obj:   &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				return w.Code == http.StatusOK
			},
		},
	}
	for k, testCase := range testCases {
		hook, bci, registry := newStorage()
		if testCase.Obj != nil {
			registry.BuildConfig = testCase.Obj
		}
		if testCase.RegErr != nil {
			registry.Err = testCase.RegErr
		}
		handler, err := hook.Connect(kapi.NewDefaultContext(), testCase.Name, &kapi.PodProxyOptions{Path: testCase.Path})
		if err != nil {
			t.Errorf("%s: %v", k, err)
			continue
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, &http.Request{})
		if err := handler.RequestError(); !testCase.ErrFn(err) {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if testCase.WFn != nil && !testCase.WFn(w) {
			t.Errorf("%s: unexpected response: %#v", k, w)
			continue
		}
		if testCase.Obj != nil {
			if bci.Request == nil {
				t.Errorf("%s: instantiator not invoked", k)
				continue
			}
			if bci.Request.Name != testCase.Obj.Name {
				t.Errorf("%s: instantiator incorrect: %#v", k, bci)
				continue
			}
		} else {
			if bci.Request != nil {
				t.Errorf("%s: instantiator should not be invoked: %#v", k, bci)
				continue
			}
		}
	}
}
