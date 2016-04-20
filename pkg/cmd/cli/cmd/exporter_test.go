package cmd

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	osautil "github.com/openshift/origin/pkg/serviceaccounts/util"
)

func TestExport(t *testing.T) {
	exporter := &defaultExporter{}

	baseSA := &kapi.ServiceAccount{}
	baseSA.Name = "my-sa"

	tests := []struct {
		name        string
		object      runtime.Object
		exact       bool
		expectedObj runtime.Object
		expectedErr error
	}{
		{
			name: "remove unexportable SA secrets",
			object: &kapi.ServiceAccount{
				ObjectMeta: kapi.ObjectMeta{
					Name: baseSA.Name,
				},
				ImagePullSecrets: []kapi.LocalObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-pull-secret"},
				},
				Secrets: []kapi.ObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: osautil.GetTokenSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-mountable-secret"},
				},
			},
			expectedObj: &kapi.ServiceAccount{
				ObjectMeta: kapi.ObjectMeta{
					Name: baseSA.Name,
				},
				ImagePullSecrets: []kapi.LocalObjectReference{
					{Name: "another-pull-secret"},
				},
				Secrets: []kapi.ObjectReference{
					{Name: "another-mountable-secret"},
				},
			},
			expectedErr: nil,
		},
		{
			name: "do not remove unexportable SA secrets with exact",
			object: &kapi.ServiceAccount{
				ObjectMeta: kapi.ObjectMeta{
					Name: baseSA.Name,
				},
				ImagePullSecrets: []kapi.LocalObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-pull-secret"},
				},
				Secrets: []kapi.ObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: osautil.GetTokenSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-mountable-secret"},
				},
			},
			expectedObj: &kapi.ServiceAccount{
				ObjectMeta: kapi.ObjectMeta{
					Name: baseSA.Name,
				},
				ImagePullSecrets: []kapi.LocalObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-pull-secret"},
				},
				Secrets: []kapi.ObjectReference{
					{Name: osautil.GetDockercfgSecretNamePrefix(baseSA) + "-foo"},
					{Name: osautil.GetTokenSecretNamePrefix(baseSA) + "-foo"},
					{Name: "another-mountable-secret"},
				},
			},
			exact:       true,
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		if err := exporter.Export(test.object, test.exact); err != test.expectedErr {
			t.Errorf("error mismatch: expected %v, got %v", test.expectedErr, err)
		}

		if !reflect.DeepEqual(test.object, test.expectedObj) {
			t.Errorf("object mismatch: expected \n%v\ngot \n%v\n", test.expectedObj, test.object)
		}
	}
}
