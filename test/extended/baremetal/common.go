package baremetal

import (
	"context"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
		if provisioningNetwork == "" {
			e2eskipper.Skipf("Unable to read ProvisioningNetwork from Provisioning CR")
		} else if provisioningNetwork != "Disabled" {
			e2eskipper.Skipf("Unsupported config in supported platform detected")
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

func firmwareSchemaClient(dc dynamic.Interface, namespace string) dynamic.ResourceInterface {
	fsClient := dc.Resource(schema.GroupVersionResource{Group: "metal3.io", Resource: "firmwareschemas", Version: "v1alpha1"})
	return fsClient.Namespace(namespace)
}

func provisioningGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "metal3.io", Resource: "provisionings", Version: "v1alpha1"}
}

func toBMH(obj unstructured.Unstructured) metal3v1alpha1.BareMetalHost {
	var bmh metal3v1alpha1.BareMetalHost
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &bmh)
	o.Expect(err).NotTo(o.HaveOccurred())
	return bmh
}

func toHFS(obj unstructured.Unstructured) metal3v1alpha1.HostFirmwareSettings {
	var hfs metal3v1alpha1.HostFirmwareSettings
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &hfs)
	o.Expect(err).NotTo(o.HaveOccurred())
	return hfs
}
