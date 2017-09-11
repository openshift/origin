package cmd

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	osautil "github.com/openshift/origin/pkg/serviceaccounts/util"
)

func TestExport(t *testing.T) {
	exporter := &DefaultExporter{}

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
			name:   "export deploymentConfig",
			object: appstest.OkDeploymentConfig(1),
			expectedObj: &appsapi.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "config",
					Generation: 1,
				},
				Spec:   appstest.OkDeploymentConfigSpec(),
				Status: appsapi.DeploymentConfigStatus{},
			},
			expectedErr: nil,
		},
		{
			name: "export imageStream",
			object: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"v1": {
							Annotations: map[string]string{"an": "annotation"},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					DockerImageRepository: "foo/bar",
					Tags: map[string]imageapi.TagEventList{
						"v1": {
							Items: []imageapi.TagEvent{{Image: "the image"}},
						},
					},
				},
			},
			expectedObj: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"v1": {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "foo/bar:v1",
							},
							Annotations: map[string]string{"an": "annotation"},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{},
				},
			},
			expectedErr: nil,
		},
		{
			name: "remove unexportable SA secrets",
			object: &kapi.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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

	for i := range tests {
		test := tests[i]

		if err := exporter.Export(test.object, test.exact); err != test.expectedErr {
			t.Errorf("%s: error mismatch: expected %v, got %v", test.name, test.expectedErr, err)
		}

		if !reflect.DeepEqual(test.object, test.expectedObj) {
			t.Errorf("%s: object mismatch: expected \n%#v\ngot \n%#v\n", test.name, test.expectedObj, test.object)
		}
	}
}
