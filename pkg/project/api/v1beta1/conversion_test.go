package v1beta1_test

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	oldkapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	newer "github.com/openshift/origin/pkg/project/api"
	current "github.com/openshift/origin/pkg/project/api/v1beta1"
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

	currentProj := current.Project{
		ObjectMeta: oldkapi.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"description": "This is a description",
				"displayName": "hi",
			},
		},
		DisplayName: "hi",
	}

	newProject := newer.Project{}
	if err := kapi.Scheme.Convert(&currentProj, &newProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(newProject, newProj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(newProject, newProj))
	}

	backProject := current.Project{}
	if err := kapi.Scheme.Convert(&newProj, &backProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(backProject, currentProj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(backProject, currentProj))
	}
}
