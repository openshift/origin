package v1beta3_test

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	newer "github.com/openshift/origin/pkg/project/api"
	current "github.com/openshift/origin/pkg/project/api/v1beta3"
)

func TestProjectConversion(t *testing.T) {
	newProj := newer.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"openshift.io/description":  "This is a description",
				"openshift.io/display-name": "hi",
			},
		},
	}

	v1beta3Proj := current.Project{
		ObjectMeta: v1beta3.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				"openshift.io/description":  "This is a description",
				"openshift.io/display-name": "hi",
			},
		},
	}

	newProject := newer.Project{}
	if err := kapi.Scheme.Convert(&v1beta3Proj, &newProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(newProject, newProj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(newProject, newProj))
	}

	backProject := current.Project{}
	if err := kapi.Scheme.Convert(&newProj, &backProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(backProject, v1beta3Proj) {
		t.Errorf("conversion error: %s", util.ObjectDiff(backProject, v1beta3Proj))
	}
}
