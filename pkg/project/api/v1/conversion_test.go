package v1_test

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	newer "github.com/openshift/origin/pkg/project/api"
	current "github.com/openshift/origin/pkg/project/api/v1"
)

func TestProjectConversion(t *testing.T) {
	newProj := newer.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"description": "This is a description",
				"displayName": "hi",
			},
		},
	}

	v1Proj := current.Project{
		ObjectMeta: v1.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"description": "This is a description",
				"displayName": "hi",
			},
		},
	}

	newProject := newer.Project{}
	if err := kapi.Scheme.Convert(&v1Proj, &newProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(newProject, newProj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(newProject, newProj))
	}

	backProject := current.Project{}
	if err := kapi.Scheme.Convert(&newProj, &backProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(backProject, v1Proj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(backProject, v1Proj))
	}
}
