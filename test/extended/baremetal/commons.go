package baremetal

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func skipIfNotBaremetal(oc *exutil.CLI) {
	g.By("checking platform type")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type != configv1.BareMetalPlatformType {
		e2eskipper.Skipf("No baremetal platform detected")
	}
}

func baremetalClient(dc dynamic.Interface) dynamic.ResourceInterface {
	baremetalClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "baremetalhosts", Version: "v1alpha1"})
	return baremetalClient.Namespace("openshift-machine-api")
}

func hostfirmwaresettingsClient(dc dynamic.Interface) dynamic.ResourceInterface {
	hfsClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "hostfirmwaresettings", Version: "v1alpha1"})
	return hfsClient.Namespace("openshift-machine-api")
}

type FieldGetterFunc func(obj map[string]interface{}, fields ...string) (interface{}, bool, error)

func expectField(host unstructured.Unstructured, resource string, nestedField string, fieldGetter FieldGetterFunc) o.Assertion {
	fields := strings.Split(nestedField, ".")

	value, found, err := fieldGetter(host.Object, fields...)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("`%s` field `%s` not found", resource, nestedField))
	return o.Expect(value)
}

func expectStringField(host unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(host, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedString(host.Object, fields...)
	})
}

func expectBoolField(host unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(host, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedBool(host.Object, fields...)
	})
}

func expectStringMapField(host unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(host, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedStringMap(host.Object, fields...)
	})
}

func expectSliceField(host unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(host, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedSlice(host.Object, fields...)
	})
}

// Conditions are stored as a slice of maps, check that the type has the correct status
func checkConditionStatus(hfs unstructured.Unstructured, condType string, condStatus string) {

	conditions, _, err := unstructured.NestedSlice(hfs.Object, "status", "conditions")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(conditions).ToNot(o.BeEmpty())

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		o.Expect(ok).To(o.BeTrue())

		t, ok := condition["type"]
		o.Expect(ok).To(o.BeTrue())
		if t == condType {
			s, ok := condition["status"]
			o.Expect(ok).To(o.BeTrue())
			o.Expect(s).To(o.Equal(condStatus))
		}
	}
}

func getField(host unstructured.Unstructured, resource string, nestedField string, fieldGetter FieldGetterFunc) string {
	fields := strings.Split(nestedField, ".")

	value, found, err := fieldGetter(host.Object, fields...)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("`%s` field `%s` not found", resource, nestedField))
	return value.(string)
}

func getStringField(host unstructured.Unstructured, resource string, nestedField string) string {
	return getField(host, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedFieldNoCopy(host.Object, fields...)
	})
}
