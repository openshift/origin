package node

import (
	"context"
	"fmt"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"

	resourcev1beta2 "k8s.io/api/resource/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][DRA][OCPFeatureGate:DynamicResourceAllocation]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("dra-scheduling", admissionapi.LevelPrivileged)

	g.Context("Dynamic Resource Allocation", func() {

		g.It("should serve only the expected DRA API versions [apigroup:resource.k8s.io]", func(ctx context.Context) {
			g.By("discovering available API versions for resource.k8s.io group")
			discoveryClient := oc.AdminKubeClient().Discovery()
			apiGroup, err := discoveryClient.ServerResourcesForGroupVersion("resource.k8s.io/v1")
			o.Expect(err).NotTo(o.HaveOccurred(), "v1 API should be available when DRA feature gate is enabled")
			o.Expect(apiGroup).NotTo(o.BeNil())
			framework.Logf("Found resource.k8s.io/v1 API with %d resources", len(apiGroup.APIResources))

			g.By("listing all available versions for resource.k8s.io group")
			apiGroupList, err := discoveryClient.ServerGroups()
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to list API groups")

			var resourceAPIGroup *metav1.APIGroup
			for _, group := range apiGroupList.Groups {
				if group.Name == "resource.k8s.io" {
					resourceAPIGroup = &group
					break
				}
			}
			o.Expect(resourceAPIGroup).NotTo(o.BeNil(), "resource.k8s.io group should exist")

			framework.Logf("Available versions for resource.k8s.io: %v", resourceAPIGroup.Versions)
			v1 := metav1.GroupVersionForDiscovery{GroupVersion: "resource.k8s.io/v1", Version: "v1"}
			o.Expect(resourceAPIGroup.Versions).To(o.ContainElement(v1), "v1 should be available")

			featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to read the cluster FeatureGate")
			o.Expect(featureGate.Status.FeatureGates).NotTo(o.BeEmpty(), "feature gate status should be populated")
			// Allow beta if any reported version enables the gate.
			taintRulesEnabled := draDeviceTaintRulesEnabled(featureGate.Status.FeatureGates)
			serverVersion, err := discoveryClient.ServerVersion()
			o.Expect(err).NotTo(o.HaveOccurred(), "should be able to read the Kubernetes version")
			allowedVersions, err := allowedDRAAPIVersions(taintRulesEnabled, serverVersion.GitVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(resourceAPIGroup.Versions).To(o.HaveEach(o.BeElementOf(allowedVersions)), "v1 is required; v1beta2 is optional only on Kubernetes 1.36 with DRADeviceTaintRules enabled")
			o.Expect(resourceAPIGroup.PreferredVersion.Version).To(o.Equal("v1"), "v1 should be the preferred version")
		})

		g.It("should manage DeviceTaintRules through the v1beta2 API [apigroup:resource.k8s.io] [OCPFeatureGate:DRADeviceTaintRules]", func(ctx context.Context) {
			serverVersion, err := oc.AdminKubeClient().Discovery().ServerVersion()
			o.Expect(err).NotTo(o.HaveOccurred())
			kubeVersion, err := version.ParseGeneric(serverVersion.GitVersion)
			o.Expect(err).NotTo(o.HaveOccurred())
			if kubeVersion.Major() != 1 || kubeVersion.Minor() != 36 {
				g.Skip("DeviceTaintRule uses the v1beta2 API only on Kubernetes 1.36")
			}

			featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !draDeviceTaintRulesEnabled(featureGate.Status.FeatureGates) {
				g.Skip("DRADeviceTaintRules is not enabled")
			}

			rules := oc.AdminKubeClient().ResourceV1beta2().DeviceTaintRules()
			g.By("creating a DeviceTaintRule through the v1beta2 API")
			rule, err := rules.Create(ctx, &resourcev1beta2.DeviceTaintRule{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "e2e-dra-api-"},
				Spec: resourcev1beta2.DeviceTaintRuleSpec{
					// A nil selector matches no devices, so the test cannot affect workloads.
					Taint: resourcev1beta2.DeviceTaint{
						Key:    "e2e.openshift.io/dra-api",
						Value:  "created",
						Effect: resourcev1beta2.DeviceTaintEffectNone,
					},
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.DeferCleanup(func(cleanupCtx context.Context) error {
				err := rules.Delete(cleanupCtx, rule.Name, metav1.DeleteOptions{})
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			})

			g.By("updating and reading back the DeviceTaintRule")
			_, err = rules.Patch(ctx, rule.Name, types.MergePatchType, []byte(`{"spec":{"taint":{"value":"updated"}}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			rule.Spec.Taint.Value = "updated"
			stored, err := rules.Get(ctx, rule.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(stored.Spec).To(o.Equal(rule.Spec))

			g.By("deleting the DeviceTaintRule")
			o.Expect(rules.Delete(ctx, rule.Name, metav1.DeleteOptions{})).To(o.Succeed())
			_, err = rules.Get(ctx, rule.Name, metav1.GetOptions{})
			o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "DeviceTaintRule should be deleted: %v", err)
		})

	})
})

func draDeviceTaintRulesEnabled(featureGates []configv1.FeatureGateDetails) bool {
	return slices.ContainsFunc(featureGates, func(details configv1.FeatureGateDetails) bool {
		return slices.Contains(details.Enabled, configv1.FeatureGateAttributes{Name: "DRADeviceTaintRules"})
	})
}

func allowedDRAAPIVersions(taintRulesEnabled bool, kubeVersion string) ([]metav1.GroupVersionForDiscovery, error) {
	allowed := []metav1.GroupVersionForDiscovery{{GroupVersion: "resource.k8s.io/v1", Version: "v1"}}
	if !taintRulesEnabled {
		return allowed, nil
	}
	parsedVersion, err := version.ParseGeneric(kubeVersion)
	if err != nil {
		return nil, fmt.Errorf("could not parse Kubernetes version %q: %w", kubeVersion, err)
	}
	// DeviceTaintRule uses v1beta2 only in 1.36; it graduates to v1 in 1.37.
	if parsedVersion.Major() == 1 && parsedVersion.Minor() == 36 {
		allowed = append(allowed, metav1.GroupVersionForDiscovery{GroupVersion: "resource.k8s.io/v1beta2", Version: "v1beta2"})
	}
	return allowed, nil
}
