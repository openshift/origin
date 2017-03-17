package api

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
)

func CheckFieldLabelConversions(t *testing.T, version, kind string, namespaceScoped bool, expectedLabels map[string]string, customLabels ...string) {
	if _, ok := expectedLabels["metadata.namespace"]; namespaceScoped != ok {
		if namespaceScoped {
			t.Errorf("%s %s requires the 'metadata.namespace' field label", version, kind)
		} else {
			t.Errorf("%s %s should not contain the 'metadata.namespace' field label", version, kind)
		}
	}
	if _, ok := expectedLabels["metadata.name"]; !ok {
		t.Errorf("%s %s requires the 'metadata.name' field label", version, kind)
	}

	for label := range expectedLabels {
		_, _, err := kapi.Scheme.ConvertFieldLabel(version, kind, label, "")
		if err != nil {
			t.Errorf("No conversion registered for %s for %s %s", label, version, kind)
		}
	}
	for _, label := range customLabels {
		_, _, err := kapi.Scheme.ConvertFieldLabel(version, kind, label, "")
		if err != nil {
			t.Errorf("No conversion registered for %s for %s %s", label, version, kind)
		}
	}
}
