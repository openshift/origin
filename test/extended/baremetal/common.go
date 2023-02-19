package baremetal

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
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

// Starting from 4.10, metal3 resources could be created in the vSphere, OpenStack and None
// Platforms in addition to the Baremetal Platform.
// Starting from 4.12, metal3 resources could be created in the AWS Platform too.
// This method can be used to check for the presence of supported Platforms and
// also the specific ProvisioningNetwork config supported in non-Baremetal platforms.
func skipIfUnsupportedPlatformOrConfig(oc *exutil.CLI, dc dynamic.Interface) {
	g.By("checking supported platforms")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	switch infra.Status.PlatformStatus.Type {
	case configv1.BareMetalPlatformType:
		return
	case configv1.OpenStackPlatformType:
		fallthrough
	case configv1.VSpherePlatformType:
		fallthrough
	case configv1.AWSPlatformType:
		fallthrough
	case configv1.AzurePlatformType:
		fallthrough
	case configv1.GCPPlatformType:
		fallthrough
	case configv1.NonePlatformType:
		provisioningNetwork := getProvisioningNetwork(dc)
		if provisioningNetwork != "Disabled" {
			e2eskipper.Skipf("Unsupported config in supported platform detected")
		} else if provisioningNetwork == "" {
			e2eskipper.Skipf("Unable to read ProvisioningNetwork from Provisioning CR")
		} else {
			return
		}
	default:
		e2eskipper.Skipf("No supported platform detected")
	}
}

func getProvisioningNetwork(dc dynamic.Interface) string {
	provisioningGVR := schema.GroupVersionResource{Group: "metal3.io", Resource: "provisionings", Version: "v1alpha1"}
	provisioningClient := dc.Resource(provisioningGVR)
	provisioningConfig, err := provisioningClient.Get(context.Background(), "provisioning-configuration", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	provisioningSpec, found, err := unstructured.NestedMap(provisioningConfig.UnstructuredContent(), "spec")
	if !found || err != nil {
		return ""
	}
	provisioningNetwork, found, err := unstructured.NestedString(provisioningSpec, "provisioningNetwork")
	if !found || err != nil {
		return ""
	}
	return provisioningNetwork
}

func baremetalClient(dc dynamic.Interface) dynamic.ResourceInterface {
	baremetalClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "baremetalhosts", Version: "v1alpha1"})
	return baremetalClient.Namespace("openshift-machine-api")
}

func hostfirmwaresettingsClient(dc dynamic.Interface) dynamic.ResourceInterface {
	hfsClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "hostfirmwaresettings", Version: "v1alpha1"})
	return hfsClient.Namespace("openshift-machine-api")
}

func preprovisioningImagesClient(dc dynamic.Interface) dynamic.ResourceInterface {
	ppiClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "preprovisioningimages", Version: "v1alpha1"})
	return ppiClient.Namespace("openshift-machine-api")
}

type FieldGetterFunc func(obj map[string]interface{}, fields ...string) (interface{}, bool, error)

func expectField(object unstructured.Unstructured, resource string, nestedField string, fieldGetter FieldGetterFunc) o.Assertion {
	fields := strings.Split(nestedField, ".")

	value, found, err := fieldGetter(object.Object, fields...)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("`%s` field `%s` not found", resource, nestedField))
	return o.Expect(value)
}

func expectStringField(object unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(object, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedString(obj, fields...)
	})
}

func expectBoolField(object unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(object, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedBool(obj, fields...)
	})
}

func expectStringMapField(object unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(object, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedStringMap(obj, fields...)
	})
}

func expectSliceField(object unstructured.Unstructured, resource string, nestedField string) o.Assertion {
	return expectField(object, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedSlice(obj, fields...)
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

func getField(object unstructured.Unstructured, resource string, nestedField string, fieldGetter FieldGetterFunc) string {
	fields := strings.Split(nestedField, ".")

	value, found, err := fieldGetter(object.Object, fields...)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("`%s` field `%s` not found", resource, nestedField))
	return value.(string)
}

func getStringField(object unstructured.Unstructured, resource string, nestedField string) string {
	return getField(object, resource, nestedField, func(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
		return unstructured.NestedFieldNoCopy(obj, fields...)
	})
}
