package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	registryauth "github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/middleware/registry"
	"github.com/docker/distribution/registry/storage"

	registrytest "github.com/openshift/origin/pkg/dockerregistry/testutil"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/restclient"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
)

const testPassthroughToUpstream = "openshift.test.passthrough-to-upstream"

func WithTestPassthroughToUpstream(ctx context.Context, passthrough bool) context.Context {
	return context.WithValue(ctx, testPassthroughToUpstream, passthrough)
}

func GetTestPassThroughToUpstream(ctx context.Context) bool {
	passthrough, found := ctx.Value(testPassthroughToUpstream).(bool)
	return found && passthrough
}

// TestBlobDescriptorServiceIsApplied ensures that blobDescriptorService middleware gets applied.
// It relies on the fact that blobDescriptorService requires higher levels to set repository object on given
// context. If the object isn't given, its method will err out.
func TestBlobDescriptorServiceIsApplied(t *testing.T) {
	// don't do any authorization check
	installFakeAccessController(t)
	m := fakeBlobDescriptorService(t)
	// to make other unit tests working
	defer m.changeUnsetRepository(false)

	testImage, err := registrytest.NewImageForManifest("user/app", registrytest.SampleImageManifestSchema1, "", true)
	if err != nil {
		t.Fatal(err)
	}
	testImageStream := registrytest.TestNewImageStreamObject("user", "app", "latest", testImage.Name, "")
	client := &testclient.Fake{}
	client.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, *testImageStream))
	client.AddReactor("get", "images", registrytest.GetFakeImageGetHandler(t, *testImage))

	ctx := context.Background()
	ctx = WithRegistryClient(ctx, makeFakeRegistryClient(client, nil))
	app := handlers.NewApp(ctx, &configuration.Configuration{
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
			"repository": {{Name: "openshift"}},
			"storage":    {{Name: "openshift"}},
		},
	})
	server := httptest.NewServer(app)
	router := v2.Router()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}
	os.Setenv("DOCKER_REGISTRY_URL", serverURL.Host)

	desc, _, err := registrytest.UploadRandomTestBlob(serverURL, nil, "user/app")
	if err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		name                      string
		method                    string
		endpoint                  string
		vars                      []string
		unsetRepository           bool
		expectedStatus            int
		expectedMethodInvocations map[string]int
	}

	doTest := func(tc testCase) {
		m.clearStats()
		m.changeUnsetRepository(tc.unsetRepository)

		route := router.GetRoute(tc.endpoint).Host(serverURL.Host)
		u, err := route.URL(tc.vars...)
		if err != nil {
			t.Errorf("[%s] failed to build route: %v", tc.name, err)
			return
		}

		req, err := http.NewRequest(tc.method, u.String(), nil)
		if err != nil {
			t.Errorf("[%s] failed to make request: %v", tc.name, err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("[%s] failed to do the request: %v", tc.name, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != tc.expectedStatus {
			t.Errorf("[%s] unexpected status code: %v != %v", tc.name, resp.StatusCode, tc.expectedStatus)
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("[%s] failed to read body: %v", tc.name, err)
			} else if len(content) > 0 {
				errs := errcode.Errors{}
				err := errs.UnmarshalJSON(content)
				if err != nil {
					t.Logf("[%s] failed to parse body as error: %v", tc.name, err)
					t.Logf("[%s] received body: %v", tc.name, string(content))
				} else {
					t.Logf("[%s] received errors: %#+v", tc.name, errs)
				}
			}
		}

		stats, err := m.getStats(tc.expectedMethodInvocations, time.Second*5)
		if err != nil {
			t.Fatalf("[%s] failed to get stats: %v", tc.name, err)
		}
		for method, exp := range tc.expectedMethodInvocations {
			invoked := stats[method]
			if invoked != exp {
				t.Errorf("[%s] unexpected number of invocations of method %q: %v != %v", tc.name, method, invoked, exp)
			}
		}
		for method, invoked := range stats {
			if _, ok := tc.expectedMethodInvocations[method]; !ok {
				t.Errorf("[%s] unexpected method %q invoked %d times", tc.name, method, invoked)
			}
		}
	}

	for _, tc := range []testCase{
		{
			name:     "get blob with repository unset",
			method:   http.MethodGet,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "get blob",
			method:   http.MethodGet,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus: http.StatusOK,
			// 1st stat is invoked in (*distribution/registry/handlers.blobHandler).GetBlob() as a
			//   check of blob existence
			// 2nd stat happens in (*errorBlobStore).ServeBlob() invoked by the same GetBlob handler
			// 3rd stat is done by (*blobServiceListener).ServeBlob once the blob serving is finished;
			//     it may happen with a slight delay after the blob was served
			expectedMethodInvocations: map[string]int{"Stat": 3},
		},

		{
			name:     "stat blob with repository unset",
			method:   http.MethodHead,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "stat blob",
			method:   http.MethodHead,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus: http.StatusOK,
			// 1st stat is invoked in (*distribution/registry/handlers.blobHandler).GetBlob() as a
			//   check of blob existence
			// 2nd stat happens in (*errorBlobStore).ServeBlob() invoked by the same GetBlob handler
			// 3rd stat is done by (*blobServiceListener).ServeBlob once the blob serving is finished;
			//     it may happen with a slight delay after the blob was served
			expectedMethodInvocations: map[string]int{"Stat": 3},
		},

		{
			name:     "delete blob with repository unset",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "delete blob",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus:            http.StatusAccepted,
			expectedMethodInvocations: map[string]int{"Stat": 1, "Clear": 1},
		},

		{
			name:     "delete manifest with repository unset",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", testImage.Name,
			},
			unsetRepository: true,
			expectedStatus:  http.StatusInternalServerError,
			// we don't allow to delete manifests from etcd; in this case, we attempt to delete layer link
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "delete manifest",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", testImage.Name,
			},
			expectedStatus: http.StatusNotFound,
			// we don't allow to delete manifests from etcd; in this case, we attempt to delete layer link
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "get manifest with repository unset",
			method:   http.MethodGet,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", "latest",
			},
			unsetRepository: true,
			// failed because we trying to get manifest from storage driver first.
			expectedStatus: http.StatusNotFound,
			// manifest can't be retrieved from etcd
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "get manifest",
			method:   http.MethodGet,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", "latest",
			},
			expectedStatus: http.StatusOK,
			// manifest is retrieved from etcd
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},
	} {
		doTest(tc)
	}
}

type testBlobDescriptorManager struct {
	mu              sync.Mutex
	cond            *sync.Cond
	stats           map[string]int
	unsetRepository bool
}

// NewTestBlobDescriptorManager allows to control blobDescriptorService and collects statistics of called
// methods.
func NewTestBlobDescriptorManager() *testBlobDescriptorManager {
	m := &testBlobDescriptorManager{
		stats: make(map[string]int),
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *testBlobDescriptorManager) clearStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k := range m.stats {
		delete(m.stats, k)
	}
}

func (m *testBlobDescriptorManager) methodInvoked(methodName string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCount := m.stats[methodName] + 1
	m.stats[methodName] = newCount
	m.cond.Signal()

	return newCount
}

// unsetRepository returns true if the testBlobDescriptorService should unset repository from context before
// passing down the call
func (m *testBlobDescriptorManager) getUnsetRepository() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.unsetRepository
}

// changeUnsetRepository allows to configure whether the testBlobDescriptorService should unset repository
// from context before passing down the call
func (m *testBlobDescriptorManager) changeUnsetRepository(unset bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unsetRepository = unset
}

// getStats waits until blob descriptor service's methods are called specified number of times and returns
// collected numbers of invocations per each method watched. An error will be returned if a given timeout is
// reached without satisfying minimum limit.s
func (m *testBlobDescriptorManager) getStats(minimumLimits map[string]int, timeout time.Duration) (map[string]int, error) {
	end := time.Now().Add(timeout)
	stats := make(map[string]int)

	if len(minimumLimits) == 0 {
		m.mu.Lock()
		for k, v := range m.stats {
			stats[k] = v
		}
		m.mu.Unlock()
		return stats, nil
	}

	c := make(chan struct{})
	go func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		for !statsGreaterThanOrEqual(m.stats, minimumLimits) {
			m.cond.Wait()
		}
		c <- struct{}{}
	}()

	var err error
	select {
	case <-time.After(end.Sub(time.Now())):
		err = fmt.Errorf("timeout while waiting on expected stats")
	case <-c:
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.stats {
		stats[k] = v
	}

	return stats, err
}

func statsGreaterThanOrEqual(stats, minimumLimits map[string]int) bool {
	for key, val := range minimumLimits {
		if val > stats[key] {
			return false
		}
	}
	return true
}

// fakeBlobDescriptorService installs a fake blob descriptor on top of blobDescriptorService that collects
// stats of method invocations. unsetRepository commands the controller to remove repository object from
// context passed down to blobDescriptorService if true.
func fakeBlobDescriptorService(t *testing.T) *testBlobDescriptorManager {
	m := NewTestBlobDescriptorManager()
	middleware.RegisterOptions(storage.BlobDescriptorServiceFactory(&testBlobDescriptorServiceFactory{t: t, m: m}))
	return m
}

type testBlobDescriptorServiceFactory struct {
	t *testing.T
	m *testBlobDescriptorManager
}

func (bf *testBlobDescriptorServiceFactory) BlobAccessController(svc distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	if _, ok := svc.(*blobDescriptorService); !ok {
		svc = (&blobDescriptorServiceFactory{}).BlobAccessController(svc)
	}
	return &testBlobDescriptorService{BlobDescriptorService: svc, t: bf.t, m: bf.m}
}

type testBlobDescriptorService struct {
	distribution.BlobDescriptorService
	t *testing.T
	m *testBlobDescriptorManager
}

func (bs *testBlobDescriptorService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	bs.m.methodInvoked("Stat")
	if bs.m.getUnsetRepository() {
		bs.t.Logf("unsetting repository from the context")
		ctx = withRepository(ctx, nil)
	}

	return bs.BlobDescriptorService.Stat(ctx, dgst)
}
func (bs *testBlobDescriptorService) Clear(ctx context.Context, dgst digest.Digest) error {
	bs.m.methodInvoked("Clear")
	if bs.m.getUnsetRepository() {
		bs.t.Logf("unsetting repository from the context")
		ctx = withRepository(ctx, nil)
	}
	return bs.BlobDescriptorService.Clear(ctx, dgst)
}

const fakeAuthorizerName = "fake"

// installFakeAccessController installs an authorizer that allows access anywhere to anybody.
func installFakeAccessController(t *testing.T) {
	registryauth.Register(fakeAuthorizerName, registryauth.InitFunc(
		func(options map[string]interface{}) (registryauth.AccessController, error) {
			t.Log("instantiating fake access controller")
			return &fakeAccessController{t: t}, nil
		}))
}

type fakeAccessController struct {
	t *testing.T
}

var _ registryauth.AccessController = &fakeAccessController{}

func (f *fakeAccessController) Authorized(ctx context.Context, access ...registryauth.Access) (context.Context, error) {
	for _, access := range access {
		f.t.Logf("fake authorizer: authorizing access to %s:%s:%s", access.Resource.Type, access.Resource.Name, access.Action)
	}

	ctx = withAuthPerformed(ctx)
	return ctx, nil
}

func makeFakeRegistryClient(client osclient.Interface, kCoreClient kcoreclient.CoreInterface) RegistryClient {
	return &fakeRegistryClient{
		client:      client,
		kCoreClient: kCoreClient,
	}
}

type fakeRegistryClient struct {
	client      osclient.Interface
	kCoreClient kcoreclient.CoreInterface
}

func (f *fakeRegistryClient) Clients() (osclient.Interface, kcoreclient.CoreInterface, error) {
	return f.client, f.kCoreClient, nil
}
func (f *fakeRegistryClient) SafeClientConfig() restclient.Config {
	return (&registryClient{}).SafeClientConfig()
}

// passthroughBlobDescriptorService passes all Stat and Clear requests to
// custom blobDescriptorService by default. If
// "openshift.test.passthrough-to-upstream" is set on context with value
// "true", all the requests will be passed straight to the upstream blob
// descriptor service.
type passthroughBlobDescriptorService struct {
	distribution.BlobDescriptorService
}

func (pbds *passthroughBlobDescriptorService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	passthrough := GetTestPassThroughToUpstream(ctx)
	if passthrough {
		context.GetLogger(ctx).Debugf("(*passthroughBlobDescriptorService).Stat: passing down to upstream blob descriptor service")
		return pbds.BlobDescriptorService.Stat(ctx, dgst)
	}
	context.GetLogger(ctx).Debugf("(*passthroughBlobDescriptorService).Stat: passing to openshift wrapper")
	return (&blobDescriptorServiceFactory{}).BlobAccessController(pbds.BlobDescriptorService).Stat(ctx, dgst)
}

func (pbds *passthroughBlobDescriptorService) Clear(ctx context.Context, dgst digest.Digest) error {
	passthrough := GetTestPassThroughToUpstream(ctx)
	if passthrough {
		context.GetLogger(ctx).Debugf("(*passthroughBlobDescriptorService).Clear: passing down to upstream blob descriptor service")
		return pbds.BlobDescriptorService.Clear(ctx, dgst)
	}
	context.GetLogger(ctx).Debugf("(*passthroughBlobDescriptorService).Clear: passing to openshift wrapper")
	return (&blobDescriptorServiceFactory{}).BlobAccessController(pbds.BlobDescriptorService).Clear(ctx, dgst)
}

type passthroughBlobDescriptorServiceFactory struct{}

func (pbf *passthroughBlobDescriptorServiceFactory) BlobAccessController(svc distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	return &passthroughBlobDescriptorService{svc}
}

func setPassthroughBlobDescriptorServiceFactory() {
	middleware.RegisterOptions(storage.BlobDescriptorServiceFactory(&passthroughBlobDescriptorServiceFactory{}))
}
