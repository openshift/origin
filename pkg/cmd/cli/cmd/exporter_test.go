package cmd

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
			object: deploytest.OkDeploymentConfig(1),
			expectedObj: &deployapi.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name:       "config",
					Generation: 1,
				},
				Spec:   deploytest.OkDeploymentConfigSpec(),
				Status: deployapi.DeploymentConfigStatus{},
			},
			expectedErr: nil,
		},
		{
			name: "export imageStream",
			object: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
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
				ObjectMeta: kapi.ObjectMeta{
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
