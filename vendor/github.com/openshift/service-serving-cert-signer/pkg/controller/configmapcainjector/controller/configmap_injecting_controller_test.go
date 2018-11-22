package controller

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/kubernetes/fake"
	listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

func TestSyncConfigMapCABundle(t *testing.T) {
	tests := []struct {
		name               string
		startingConfigMaps []runtime.Object
		namespace          string
		cmName             string
		caBundle           string
		validateActions    func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:      "missing",
			namespace: "foo",
			cmName:    "foo",
			caBundle:  "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "requested and empty",
			startingConfigMaps: []runtime.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"},
						Namespace:   "foo",
					},
					Data: map[string]string{},
				},
			},
			namespace: "foo",
			cmName:    "foo",
			caBundle:  "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "configmaps") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*v1.ConfigMap)
				if expected := "content"; string(actual.Data[api.InjectionDataKey]) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
		{
			name: "requested and different",
			startingConfigMaps: []runtime.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"},
						Namespace:   "foo",
					},
					Data: map[string]string{
						api.InjectionDataKey: "foo",
					},
				},
			},
			namespace: "foo",
			cmName:    "foo",
			caBundle:  "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "configmaps") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*v1.ConfigMap)
				if expected := "content"; string(actual.Data[api.InjectionDataKey]) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
		{
			name: "requested and same",
			startingConfigMaps: []runtime.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Annotations: map[string]string{api.InjectCABundleAnnotationName: "true"},
						Namespace:   "foo",
					},
					Data: map[string]string{
						api.InjectionDataKey: "content",
					},
				},
			},
			namespace: "foo",
			cmName:    "foo",
			caBundle:  "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tc.startingConfigMaps...)
			index := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, configMap := range tc.startingConfigMaps {
				index.Add(configMap)
			}
			c := &configMapCABundleInjectionController{
				configMapLister: listers.NewConfigMapLister(index),
				configMapClient: fakeClient.CoreV1(),
				ca:              tc.caBundle,
			}

			obj, err := c.Key(tc.namespace, tc.cmName)
			if err == nil {
				if err := c.Sync(obj); err != nil {
					t.Fatal(err)
				}
			}

			tc.validateActions(t, fakeClient.Actions())
		})
	}
}
