package apiservicecabundle

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	apiregistrationapiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiserviceclientfake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	apiservicelister "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"
)

func TestSyncAPIService(t *testing.T) {
	tests := []struct {
		name                string
		startingAPIServices []runtime.Object
		key                 string
		caBundle            []byte
		validateActions     func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:     "missing",
			key:      "foo",
			caBundle: []byte("content"),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "requested and empty",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{InjectCABundleAnnotationName: "true"}},
				},
			},
			key:      "foo",
			caBundle: []byte("content"),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "apiservices") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*apiregistrationapiv1.APIService)
				if expected := "content"; string(actual.Spec.CABundle) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
		{
			name: "requested and nochange",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{InjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("content"),
					},
				},
			},
			key:      "foo",
			caBundle: []byte("content"),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "requested and differe",
			startingAPIServices: []runtime.Object{
				&apiregistrationapiv1.APIService{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{InjectCABundleAnnotationName: "true"}},
					Spec: apiregistrationapiv1.APIServiceSpec{
						CABundle: []byte("old"),
					},
				},
			},
			key:      "foo",
			caBundle: []byte("content"),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "apiservices") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*apiregistrationapiv1.APIService)
				if expected := "content"; string(actual.Spec.CABundle) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := apiserviceclientfake.NewSimpleClientset(tc.startingAPIServices...)
			index := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, apiService := range tc.startingAPIServices {
				index.Add(apiService)
			}

			c := &ServiceServingCertUpdateController{
				apiServiceLister: apiservicelister.NewAPIServiceLister(index),
				apiServiceClient: fakeClient.ApiregistrationV1(),
				caBundle:         tc.caBundle,
			}

			err := c.syncAPIService(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			tc.validateActions(t, fakeClient.Actions())
		})
	}
}
