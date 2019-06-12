package securitycontextconstraints

import (
	"bytes"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apiserver/pkg/admission"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDefaultingHappens(t *testing.T) {
	inputSCC := `{
	"allowHostDirVolumePlugin": true,
	"allowHostNetwork": true,
	"allowHostPID": true,
	"allowHostPorts": true,
	"apiVersion": "security.openshift.io/v1",
	"kind": "SecurityContextConstraints",
	"metadata": {
		"annotations": {
			"kubernetes.io/description": "node-exporter scc is used for the Prometheus node exporter"
		},
		"name": "node-exporter"
	},
	"readOnlyRootFilesystem": false,
	"runAsUser": {
		"type": "RunAsAny"
	},
	"seLinuxContext": {
		"type": "RunAsAny"
	},
	"users": []
}`

	inputUnstructured := &unstructured.Unstructured{}
	_, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(inputSCC), nil, inputUnstructured)
	if err != nil {
		t.Fatal(err)
	}

	attributes := admission.NewAttributesRecord(inputUnstructured, nil, schema.GroupVersionKind{}, "", "", schema.GroupVersionResource{Group: "security.openshift.io", Resource: "securitycontextconstraints"}, "", admission.Create, false, nil)
	defaulter := NewDefaulter()
	if err := defaulter.(*defaultSCC).Admit(attributes, nil); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	if err := unstructured.UnstructuredJSONScheme.Encode(inputUnstructured, buf); err != nil {
		t.Fatal(err)
	}

	expectedSCC := `{
	"allowHostDirVolumePlugin": true,
	"allowHostIPC": false,
	"allowHostNetwork": true,
	"allowHostPID": true,
	"allowHostPorts": true,
	"allowPrivilegeEscalation": true,
	"allowPrivilegedContainer": false,
	"allowedCapabilities": null,
	"apiVersion": "security.openshift.io/v1",
	"defaultAddCapabilities": null,
	"fsGroup": {
		"type": "RunAsAny"
	},
	"groups": [],
	"kind": "SecurityContextConstraints",
	"metadata": {
		"annotations": {
			"kubernetes.io/description": "node-exporter scc is used for the Prometheus node exporter"
		},
		"name": "node-exporter",
		"creationTimestamp":null
	},
	"priority": null,
	"readOnlyRootFilesystem": false,
	"requiredDropCapabilities": null,
	"runAsUser": {
		"type": "RunAsAny"
	},
	"seLinuxContext": {
		"type": "RunAsAny"
	},
	"supplementalGroups": {
		"type": "RunAsAny"
	},
	"users": [],
	"volumes": [
		"*"
	]
}`
	expectedUnstructured := &unstructured.Unstructured{}
	if _, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(expectedSCC), nil, expectedUnstructured); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expectedUnstructured.Object, inputUnstructured.Object) {
		t.Fatal(diff.ObjectDiff(expectedUnstructured.Object, inputUnstructured.Object))
	}
}
