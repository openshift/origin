package create

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	ktesting "k8s.io/client-go/testing"

	"github.com/openshift/library-go/pkg/assets"
)

func init() {
	fetchLatestDiscoveryInfoFn = func(dc *discovery.DiscoveryClient) (meta.RESTMapper, error) {
		resourcesForEnsureMutex.Lock()
		defer resourcesForEnsureMutex.Unlock()
		return restmapper.NewDiscoveryRESTMapper(resourcesForEnsure), nil
	}
	newClientsFn = func(config *rest.Config) (dynamic.Interface, *discovery.DiscoveryClient, error) {
		fakeScheme := runtime.NewScheme()
		// TODO: This is a workaround for dynamic fake client bug where the List kind is enforced and duplicated in object reactor.
		fakeScheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ListList"}, &unstructured.UnstructuredList{})
		dynamicClient := dynamicfake.NewSimpleDynamicClient(fakeScheme)
		return dynamicClient, nil, nil
	}
}

var (
	resources = []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "kubeapiserver.operator.openshift.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1alpha1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1alpha1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1alpha1": {
					{Name: "kubeapiserveroperatorconfigs", Namespaced: false, Kind: "KubeAPIServerOperatorConfig"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: "apiextensions.k8s.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1beta1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1beta1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {
					{Name: "customresourcedefinitions", Namespaced: false, Kind: "CustomResourceDefinition"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: "",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
					{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
					{Name: "secrets", Namespaced: true, Kind: "Secret"},
				},
			},
		},
	}

	// Copy this to not overlap with other tests if ran in parallel
	resourcesForEnsure      = resources
	resourcesForEnsureMutex sync.Mutex
)

func TestEnsureManifestsCreated(t *testing.T) {
	// Success
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := EnsureManifestsCreated(ctx, "testdata", nil, CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Missing discovery info for kubeapiserverconfig
	out := &bytes.Buffer{}
	operatorResource := resourcesForEnsure[0]
	resourcesForEnsure = resourcesForEnsure[1:]
	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = EnsureManifestsCreated(ctx, "testdata", nil, CreateOptions{Verbose: true, StdErr: out})
	if err == nil {
		t.Fatal("expected error creating kubeapiserverconfig resource, got none")
	}
	if !strings.Contains(out.String(), "unable to get REST mapping") {
		t.Fatalf("expected error logged to output when verbose is on, got: %s\n", out.String())
	}

	// Should succeed on updated discovery info
	go func() {
		time.Sleep(2 * time.Second)
		resourcesForEnsureMutex.Lock()
		defer resourcesForEnsureMutex.Unlock()
		resourcesForEnsure = append(resourcesForEnsure, operatorResource)
	}()
	out = &bytes.Buffer{}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = EnsureManifestsCreated(ctx, "testdata", nil, CreateOptions{Verbose: true, StdErr: out})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `no matches for kind "KubeAPIServerOperatorConfig"`) {
		t.Fatalf("expected error logged to output when verbose is on, got: %s\n", out.String())
	}
	if !strings.Contains(out.String(), `Created apiextensions.k8s.io/v1beta1`) {
		t.Fatalf("expected success logged to output when verbose is on, got: %s\n", out.String())
	}
}

func TestCreate(t *testing.T) {
	ctx := context.Background()

	resourcesWithoutKubeAPIServer := resources[1:]
	testConfigMap := &unstructured.Unstructured{}
	testConfigMap.SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "ConfigMap",
	})
	testConfigMap.SetName("aggregator-client-ca")
	testConfigMap.SetNamespace("openshift-kube-apiserver")

	tests := []struct {
		name              string
		discovery         []*restmapper.APIGroupResources
		expectError       bool
		expectFailedCount int
		expectReload      bool
		existingObjects   []runtime.Object
		evalActions       func(*testing.T, []ktesting.Action)
	}{
		{
			name:      "create all resources",
			discovery: resources,
		},
		{
			name:              "fail to create kube apiserver operator config",
			discovery:         resourcesWithoutKubeAPIServer,
			expectFailedCount: 1,
			expectError:       true,
			expectReload:      true,
		},
		{
			name:            "create all resources",
			discovery:       resources,
			existingObjects: []runtime.Object{testConfigMap},
		},
	}

	fakeScheme := runtime.NewScheme()
	// TODO: This is a workaround for dynamic fake client bug where the List kind is enforced and duplicated in object reactor.
	fakeScheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ListList"}, &unstructured.UnstructuredList{})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifests, err := load("testdata", CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}

			dynamicClient := dynamicfake.NewSimpleDynamicClient(fakeScheme, tc.existingObjects...)
			restMapper := restmapper.NewDiscoveryRESTMapper(tc.discovery)

			err, reload := create(ctx, manifests, dynamicClient, restMapper, CreateOptions{Verbose: true, StdErr: os.Stderr})
			if tc.expectError && err == nil {
				t.Errorf("expected error, got no error")
				return
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectReload && !reload {
				t.Errorf("expected reload, got none")
				return
			}
			if !tc.expectReload && reload {
				t.Errorf("unexpected reload, got one")
				return
			}
			if len(manifests) != tc.expectFailedCount {
				t.Errorf("expected %d failed manifests, got %d", tc.expectFailedCount, len(manifests))
				return
			}
			if tc.evalActions != nil {
				tc.evalActions(t, dynamicClient.Actions())
			}
		})

	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name                  string
		options               CreateOptions
		assetDir              string
		expectedManifestCount int
		expectError           bool
	}{
		{
			name:                  "read all manifests",
			assetDir:              "testdata",
			expectedManifestCount: 5,
		},
		{
			name:        "handle missing dir",
			assetDir:    "foo",
			expectError: true,
		},
		{
			name: "read only 00_ prefixed files",
			options: CreateOptions{
				Filters: []assets.FileInfoPredicate{
					func(info os.FileInfo) bool {
						return strings.HasPrefix(info.Name(), "00")
					},
				},
			},
			assetDir:              "testdata",
			expectedManifestCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := load(tc.assetDir, tc.options)
			if tc.expectError && err == nil {
				t.Errorf("expected error, got no error")
				return
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(result) != tc.expectedManifestCount {
				t.Errorf("expected %d manifests loaded, got %d", tc.expectedManifestCount, len(result))
				return
			}
		})
	}
}
