package networking

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

// These tests patch platformStatus members other than the cluster's own with
// server-side dry run, so they run on every platform and topology: nothing is
// persisted and the CRD schema and CEL rules are those served by the cluster.
var _ = g.Describe(onPremDNSRecordsAPITags+" on-prem DNS records API validation", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("onprem-dns-records-api")

	var keys []string
	g.BeforeEach(func(ctx context.Context) {
		infra := getOnPremDNSInfrastructure(ctx, oc)
		keys = onPremDNSValidationKeys(infra.Status.PlatformStatus)
	})

	g.It("should reject dnsRecordsType External without a UserManaged load balancer", func(ctx context.Context) {
		var problems []string
		for _, key := range keys {
			if err := expectOnPremDNSPatchRejected(ctx, oc, key, string(configv1.DNSRecordsTypeExternal), configv1.LoadBalancerTypeOpenShiftManagedDefault, onPremDNSRecordsMismatchMessage); err != nil {
				problems = append(problems, err.Error())
			}
			// An omitted load balancer defaults to OpenShiftManagedDefault.
			if err := expectOnPremDNSPatchRejected(ctx, oc, key, string(configv1.DNSRecordsTypeExternal), "", onPremDNSRecordsMismatchMessage); err != nil {
				problems = append(problems, err.Error())
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should accept dnsRecordsType External with a UserManaged load balancer", func(ctx context.Context) {
		var problems []string
		for _, key := range keys {
			stored, err := expectOnPremDNSPatchAccepted(ctx, oc, key, string(configv1.DNSRecordsTypeExternal), configv1.LoadBalancerTypeUserManaged)
			switch {
			case err != nil:
				problems = append(problems, err.Error())
			case stored != configv1.DNSRecordsTypeExternal:
				problems = append(problems, fmt.Sprintf("platformStatus.%s: dnsRecordsType was not stored (got %q): the field is pruned when OnPremDNSRecords is disabled", key, stored))
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should accept dnsRecordsType Internal with any load balancer type", func(ctx context.Context) {
		var problems []string
		for _, key := range keys {
			for _, lb := range []configv1.PlatformLoadBalancerType{configv1.LoadBalancerTypeOpenShiftManagedDefault, configv1.LoadBalancerTypeUserManaged} {
				stored, err := expectOnPremDNSPatchAccepted(ctx, oc, key, string(configv1.DNSRecordsTypeInternal), lb)
				switch {
				case err != nil:
					problems = append(problems, err.Error())
				case stored != configv1.DNSRecordsTypeInternal:
					problems = append(problems, fmt.Sprintf("platformStatus.%s with loadBalancer %s: dnsRecordsType was not stored (got %q)", key, lb, stored))
				}
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should reject an unknown dnsRecordsType value", func(ctx context.Context) {
		var problems []string
		for _, key := range keys {
			if err := expectOnPremDNSPatchRejected(ctx, oc, key, "Invalid", configv1.LoadBalancerTypeUserManaged, `Unsupported value: "Invalid"`); err != nil {
				problems = append(problems, err.Error())
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should serve dnsRecordsType for every on-prem platform in the Infrastructure CRD schema", func(ctx context.Context) {
		crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "infrastructures.config.openshift.io", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var problems []string
		found := false
		for _, version := range crd.Spec.Versions {
			if version.Name != "v1" {
				continue
			}
			found = true
			o.Expect(version.Schema).NotTo(o.BeNil())
			o.Expect(version.Schema.OpenAPIV3Schema).NotTo(o.BeNil())
			platformStatus := version.Schema.OpenAPIV3Schema.Properties["status"].Properties["platformStatus"]
			for _, key := range onPremDNSSchemaKeys {
				field, ok := platformStatus.Properties[key].Properties["dnsRecordsType"]
				if !ok {
					problems = append(problems, fmt.Sprintf("status.platformStatus.%s has no dnsRecordsType", key))
					continue
				}
				enum := sets.New[string]()
				for _, v := range field.Enum {
					enum.Insert(strings.Trim(string(v.Raw), `"`))
				}
				if want := sets.New(string(configv1.DNSRecordsTypeInternal), string(configv1.DNSRecordsTypeExternal)); !enum.Equal(want) {
					problems = append(problems, fmt.Sprintf("status.platformStatus.%s.dnsRecordsType allows %v, expected %v", key, sets.List(enum), sets.List(want)))
				}
			}
		}
		o.Expect(found).To(o.BeTrue(), "the Infrastructure CRD does not serve v1")
		o.Expect(problems).To(o.BeEmpty())
	})
})

// These tests check how dnsRecordsType is applied on self-managed on-prem IPI
// clusters, in whichever mode the cluster was installed.
var _ = g.Describe(onPremDNSRecordsNodeTags+" on-prem DNS records", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("onprem-dns-records", admissionapi.LevelPrivileged)

	var (
		infra    *configv1.Infrastructure
		platform onPremDNSPlatform
	)
	g.BeforeEach(func(ctx context.Context) {
		infra, platform = skipUnlessOnPremDNS(ctx, oc)
		e2e.Logf("platform %s: dnsRecordsType=%q (effective %s), loadBalancer=%s", platform.Type, platform.DNSRecordsType, platform.Effective(), platform.LoadBalancer)
	})

	g.It("should report a dnsRecordsType consistent with the load balancer type", func(ctx context.Context) {
		o.Expect(platform.Effective()).To(o.BeElementOf(configv1.DNSRecordsTypeInternal, configv1.DNSRecordsTypeExternal))
		if platform.Effective() == configv1.DNSRecordsTypeExternal {
			o.Expect(platform.LoadBalancer).To(o.Equal(configv1.LoadBalancerTypeUserManaged),
				"dnsRecordsType External requires a UserManaged load balancer")
		}
	})

	g.It("should render the CoreDNS cluster records only when dnsRecordsType is Internal", func(ctx context.Context) {
		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1()
		pools, err := mcClient.MachineConfigPools().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var problems []string
		checked := 0
		for _, pool := range pools.Items {
			if pool.Spec.Configuration.Name == "" {
				continue
			}
			mc, err := oc.AdminDynamicClient().Resource(machineConfigGVR).Get(ctx, pool.Spec.Configuration.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			corefile, found, err := corefileTemplateFromRenderedMachineConfig(mc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if !found {
				problems = append(problems, fmt.Sprintf("pool %s: rendered MachineConfig %s has no %s", pool.Name, mc.GetName(), onPremDNSCorefileTemplatePath))
				continue
			}
			checked++
			if err := checkOnPremDNSCorefile(corefile, platform.Effective()); err != nil {
				problems = append(problems, fmt.Sprintf("pool %s (%s): %v", pool.Name, mc.GetName(), err))
			}
		}
		o.Expect(problems).To(o.BeEmpty())
		o.Expect(checked).To(o.BeNumerically(">", 0), "no MachineConfigPool targets a rendered MachineConfig")
	})

	g.It("should run CoreDNS with a Corefile matching dnsRecordsType on every node", func(ctx context.Context) {
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).NotTo(o.BeEmpty())

		label := "app=" + platform.ShortName + "-infra-coredns"
		err = onPremDNSEventually(ctx, onPremDNSSyncTimeout, "see a Ready CoreDNS static pod on every node", func(ctx context.Context) error {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(platform.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: label})
			if err != nil {
				return err
			}
			ready := sets.New[string]()
			for i := range pods.Items {
				if podReady(&pods.Items[i]) {
					ready.Insert(pods.Items[i].Spec.NodeName)
				}
			}
			var missing []string
			for _, node := range nodes.Items {
				if !ready.Has(node.Name) {
					missing = append(missing, node.Name)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("no Ready %s pod in %s on %v", label, platform.Namespace(), missing)
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		var problems []string
		for _, node := range nodes.Items {
			corefile, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, oc.Namespace(), "cat", onPremDNSCorefilePath)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: cannot read %s: %v", node.Name, onPremDNSCorefilePath, err))
				continue
			}
			if err := checkOnPremDNSCorefile(corefile, platform.Effective()); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", node.Name, err))
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should resolve api, api-int and ingress names from every node", func(ctx context.Context) {
		domain, err := onPremDNSClusterDomain(infra)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).NotTo(o.BeEmpty())

		// With Internal records the node CoreDNS answers with the VIPs. With
		// External records the names must still resolve, through the
		// user-provided DNS server, to whatever the user configured.
		apiVIPs, ingressVIPs := platform.APIVIPs, platform.IngressVIPs
		if platform.Effective() == configv1.DNSRecordsTypeExternal {
			apiVIPs, ingressVIPs = nil, nil
		}
		names := []struct {
			name string
			vips []string
		}{
			{"api." + domain, apiVIPs},
			{"api-int." + domain, apiVIPs},
			{"onprem-dns-records-test.apps." + domain, ingressVIPs},
		}

		var problems []string
		for _, node := range nodes.Items {
			for _, n := range names {
				err := onPremDNSEventually(ctx, onPremDNSAPITimeout, fmt.Sprintf("resolve %s on %s", n.name, node.Name), func(ctx context.Context) error {
					out, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, oc.Namespace(), "getent", "ahosts", n.name)
					if err != nil {
						return fmt.Errorf("%s does not resolve: %v", n.name, err)
					}
					return checkOnPremDNSResolution(n.name, parseGetentAhosts(out), n.vips)
				})
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", node.Name, err))
				}
			}
		}
		o.Expect(problems).To(o.BeEmpty())
	})

	g.It("should propagate dnsRecordsType to the ControllerConfig", func(ctx context.Context) {
		key := onPremDNSStatusKeys[platform.Type]
		err := onPremDNSEventually(ctx, onPremDNSSyncTimeout, "see the ControllerConfig carry the Infrastructure dnsRecordsType", func(ctx context.Context) error {
			cc, err := oc.MachineConfigurationClient().MachineconfigurationV1().ControllerConfigs().Get(ctx, "machine-config-controller", metav1.GetOptions{})
			if err != nil {
				return err
			}
			if cc.Spec.Infra == nil {
				return fmt.Errorf("controllerconfig/machine-config-controller has no spec.infra")
			}
			got := dnsRecordsTypeForStatusKey(cc.Spec.Infra.Status.PlatformStatus, key)
			if got != platform.DNSRecordsType {
				return fmt.Errorf("controllerconfig spec.infra.status.platformStatus.%s.dnsRecordsType is %q, Infrastructure has %q", key, got, platform.DNSRecordsType)
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
