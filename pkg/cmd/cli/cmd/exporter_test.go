package cmd

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestExport(t *testing.T) {
	exporter := &defaultExporter{}

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
					Name: "config",
				},
				LatestVersion: 0,
				Triggers: []deployapi.DeploymentTriggerPolicy{
					deploytest.OkImageChangeTrigger(),
				},
				Template: deploytest.OkDeploymentTemplate(),
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
