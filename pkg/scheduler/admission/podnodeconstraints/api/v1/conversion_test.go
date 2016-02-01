package v1_test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"
	_ "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api/install"
	versioned "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api/v1"
)

func TestConversions(t *testing.T) {
	input := &versioned.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: []string{"test"},
	}
	expected := &api.PodNodeConstraintsConfig{
		NodeSelectorLabelBlacklist: sets.NewString([]string{"test"}...),
	}
	output := &api.PodNodeConstraintsConfig{}
	err := configapi.Scheme.Convert(input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !kapi.Semantic.DeepEqual(&output, &expected) {
		t.Errorf("unexpected conversion; Expected %+v; Got %+v", expected, output)
	}
}
