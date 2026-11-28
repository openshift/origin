package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// onPremDNSRecordsTags is the frozen tag set of every OnPremDNSRecords test.
	// Renaming any part of it resets the test history in Sippy.
	onPremDNSRecordsTags = `[sig-network][Jira:"Networking / On-Prem DNS"][OCPFeatureGate:OnPremDNSRecords]`
	// onPremDNSRecordsAPITags only requires config.openshift.io so the API
	// validation tests also run on clusters without the MCO API (HyperShift).
	onPremDNSRecordsAPITags  = onPremDNSRecordsTags + `[apigroup:config.openshift.io]`
	onPremDNSRecordsNodeTags = onPremDNSRecordsTags + `[apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io]`

	onPremDNSPollInterval = 5 * time.Second
	onPremDNSAPITimeout   = time.Minute
	onPremDNSSyncTimeout  = 5 * time.Minute

	// onPremDNSCorefileTemplatePath is the CoreDNS template MCO renders on
	// on-prem platforms; runtimecfg turns it into /etc/coredns/Corefile.
	onPremDNSCorefileTemplatePath = "/etc/kubernetes/static-pod-resources/coredns/Corefile.tmpl"
	onPremDNSCorefilePath         = "/etc/coredns/Corefile"

	onPremDNSRecordsMismatchMessage = "dnsRecordsType may only be set to External when loadBalancer.type is UserManaged"
)

// onPremDNSStatusKeys are the platformStatus members that carry
// dnsRecordsType and are honored by MCO, keyed by platform type.
var onPremDNSStatusKeys = map[configv1.PlatformType]string{
	configv1.BareMetalPlatformType: "baremetal",
	configv1.OpenStackPlatformType: "openstack",
	configv1.VSpherePlatformType:   "vsphere",
	configv1.NutanixPlatformType:   "nutanix",
}

// onPremDNSSchemaKeys are all platformStatus members whose schema must carry
// dnsRecordsType when the OnPremDNSRecords gate is enabled.
var onPremDNSSchemaKeys = []string{"baremetal", "openstack", "ovirt", "vsphere", "nutanix"}

// onPremDNSPlatform is the DNS-relevant part of an on-prem platformStatus.
type onPremDNSPlatform struct {
	Type           configv1.PlatformType
	ShortName      string
	DNSRecordsType configv1.DNSRecordsType
	LoadBalancer   configv1.PlatformLoadBalancerType
	APIVIPs        []string
	IngressVIPs    []string
}

// Effective returns the dnsRecordsType the platform applies: an empty value
// means Internal.
func (p onPremDNSPlatform) Effective() configv1.DNSRecordsType {
	if p.DNSRecordsType == "" {
		return configv1.DNSRecordsTypeInternal
	}
	return p.DNSRecordsType
}

// Namespace is the namespace of the on-prem infra static pods.
func (p onPremDNSPlatform) Namespace() string {
	return "openshift-" + p.ShortName + "-infra"
}

// onPremDNSPlatformFromStatus extracts the DNS-relevant fields of the
// platforms MCO honors dnsRecordsType for. ok is false for any other platform.
func onPremDNSPlatformFromStatus(ps *configv1.PlatformStatus) (onPremDNSPlatform, bool) {
	if ps == nil {
		return onPremDNSPlatform{}, false
	}
	p := onPremDNSPlatform{Type: ps.Type, LoadBalancer: configv1.LoadBalancerTypeOpenShiftManagedDefault}
	switch ps.Type {
	case configv1.BareMetalPlatformType:
		if ps.BareMetal == nil {
			return p, false
		}
		p.ShortName = "kni"
		p.DNSRecordsType = ps.BareMetal.DNSRecordsType
		p.APIVIPs, p.IngressVIPs = ps.BareMetal.APIServerInternalIPs, ps.BareMetal.IngressIPs
		if ps.BareMetal.LoadBalancer != nil && ps.BareMetal.LoadBalancer.Type != "" {
			p.LoadBalancer = ps.BareMetal.LoadBalancer.Type
		}
	case configv1.OpenStackPlatformType:
		if ps.OpenStack == nil {
			return p, false
		}
		p.ShortName = "openstack"
		p.DNSRecordsType = ps.OpenStack.DNSRecordsType
		p.APIVIPs, p.IngressVIPs = ps.OpenStack.APIServerInternalIPs, ps.OpenStack.IngressIPs
		if ps.OpenStack.LoadBalancer != nil && ps.OpenStack.LoadBalancer.Type != "" {
			p.LoadBalancer = ps.OpenStack.LoadBalancer.Type
		}
	case configv1.VSpherePlatformType:
		if ps.VSphere == nil {
			return p, false
		}
		p.ShortName = "vsphere"
		p.DNSRecordsType = ps.VSphere.DNSRecordsType
		p.APIVIPs, p.IngressVIPs = ps.VSphere.APIServerInternalIPs, ps.VSphere.IngressIPs
		if ps.VSphere.LoadBalancer != nil && ps.VSphere.LoadBalancer.Type != "" {
			p.LoadBalancer = ps.VSphere.LoadBalancer.Type
		}
	case configv1.NutanixPlatformType:
		if ps.Nutanix == nil {
			return p, false
		}
		p.ShortName = "nutanix"
		p.DNSRecordsType = ps.Nutanix.DNSRecordsType
		p.APIVIPs, p.IngressVIPs = ps.Nutanix.APIServerInternalIPs, ps.Nutanix.IngressIPs
		if ps.Nutanix.LoadBalancer != nil && ps.Nutanix.LoadBalancer.Type != "" {
			p.LoadBalancer = ps.Nutanix.LoadBalancer.Type
		}
	default:
		return p, false
	}
	return p, true
}

func getOnPremDNSInfrastructure(ctx context.Context, oc *exutil.CLI) *configv1.Infrastructure {
	var infra *configv1.Infrastructure
	err := onPremDNSEventually(ctx, onPremDNSAPITimeout, "read infrastructures/cluster", func(ctx context.Context) error {
		var err error
		infra, err = oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		return err
	})
	if err != nil {
		e2e.Failf("%v", err)
	}
	return infra
}

// skipUnlessOnPremDNS skips the test unless the cluster is a self-managed
// on-prem IPI cluster running the on-prem CoreDNS static pods.
func skipUnlessOnPremDNS(ctx context.Context, oc *exutil.CLI) (*configv1.Infrastructure, onPremDNSPlatform) {
	infra := getOnPremDNSInfrastructure(ctx, oc)
	if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
		e2eskipper.Skipf("on-prem DNS records are not managed on clusters with an external control plane")
	}
	p, ok := onPremDNSPlatformFromStatus(infra.Status.PlatformStatus)
	if !ok {
		e2eskipper.Skipf("dnsRecordsType only applies to the BareMetal, OpenStack, VSphere and Nutanix platforms")
	}
	if len(p.APIVIPs) == 0 {
		e2eskipper.Skipf("the cluster has no API VIPs (user-provisioned or single-node): the on-prem CoreDNS static pods are not deployed")
	}
	return infra, p
}

// onPremDNSClusterDomain returns the cluster domain (<name>.<baseDomain>)
// from the internal API URL, whose host is api-int.<cluster domain>.
func onPremDNSClusterDomain(infra *configv1.Infrastructure) (string, error) {
	u, err := url.Parse(infra.Status.APIServerInternalURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse apiServerInternalURL %q: %w", infra.Status.APIServerInternalURL, err)
	}
	domain, ok := strings.CutPrefix(u.Hostname(), "api-int.")
	if !ok || domain == "" {
		return "", fmt.Errorf("apiServerInternalURL host %q does not start with api-int.", u.Hostname())
	}
	return domain, nil
}

// onPremDNSEventually polls fn until it returns nil, keeping the last error as
// the reason reported on timeout.
func onPremDNSEventually(ctx context.Context, timeout time.Duration, what string, fn func(ctx context.Context) error) error {
	var last error
	err := wait.PollUntilContextTimeout(ctx, onPremDNSPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		if err := fn(ctx); err != nil {
			if last == nil || last.Error() != err.Error() {
				e2e.Logf("waiting to %s: %v", what, err)
			}
			last = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting to %s: %v", what, last)
	}
	return nil
}

// onPremDNSStatusPatch builds a merge patch setting dnsRecordsType, and the
// load balancer type when lbType is not empty, on platformStatus.<key>.
func onPremDNSStatusPatch(key string, dnsRecordsType string, lbType configv1.PlatformLoadBalancerType) ([]byte, error) {
	platform := map[string]interface{}{"dnsRecordsType": dnsRecordsType}
	if lbType != "" {
		platform["loadBalancer"] = map[string]interface{}{"type": string(lbType)}
	}
	return json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"platformStatus": map[string]interface{}{key: platform},
		},
	})
}

// dryRunOnPremDNSStatusPatch applies the patch to infrastructures/cluster
// status with server-side dry run: the CRD schema and CEL rules are evaluated
// but nothing is persisted.
func dryRunOnPremDNSStatusPatch(ctx context.Context, oc *exutil.CLI, key string, dnsRecordsType string, lbType configv1.PlatformLoadBalancerType) (*configv1.Infrastructure, error) {
	patch, err := onPremDNSStatusPatch(key, dnsRecordsType, lbType)
	if err != nil {
		return nil, err
	}
	return oc.AdminConfigClient().ConfigV1().Infrastructures().Patch(ctx, "cluster", types.MergePatchType, patch,
		metav1.PatchOptions{DryRun: []string{metav1.DryRunAll}}, "status")
}

// expectOnPremDNSPatchAccepted dry-runs the patch until it is accepted and
// returns the dnsRecordsType the API server would have stored.
func expectOnPremDNSPatchAccepted(ctx context.Context, oc *exutil.CLI, key string, dnsRecordsType string, lbType configv1.PlatformLoadBalancerType) (configv1.DNSRecordsType, error) {
	var stored configv1.DNSRecordsType
	err := onPremDNSEventually(ctx, onPremDNSAPITimeout, fmt.Sprintf("dry-run dnsRecordsType=%s loadBalancer=%q on platformStatus.%s", dnsRecordsType, lbType, key), func(ctx context.Context) error {
		infra, err := dryRunOnPremDNSStatusPatch(ctx, oc, key, dnsRecordsType, lbType)
		if apierrors.IsInvalid(err) {
			return fmt.Errorf("unexpectedly rejected: %w", err)
		}
		if err != nil {
			return err
		}
		stored = dnsRecordsTypeForStatusKey(infra.Status.PlatformStatus, key)
		return nil
	})
	return stored, err
}

// expectOnPremDNSPatchRejected dry-runs the patch until it is rejected as
// Invalid and checks the error contains want.
func expectOnPremDNSPatchRejected(ctx context.Context, oc *exutil.CLI, key string, dnsRecordsType string, lbType configv1.PlatformLoadBalancerType, want string) error {
	var invalid error
	err := onPremDNSEventually(ctx, onPremDNSAPITimeout, fmt.Sprintf("see dnsRecordsType=%s loadBalancer=%q on platformStatus.%s rejected", dnsRecordsType, lbType, key), func(ctx context.Context) error {
		_, err := dryRunOnPremDNSStatusPatch(ctx, oc, key, dnsRecordsType, lbType)
		if err == nil {
			return fmt.Errorf("the patch was accepted")
		}
		if !apierrors.IsInvalid(err) {
			return err
		}
		invalid = err
		return nil
	})
	if err != nil {
		return err
	}
	if !strings.Contains(invalid.Error(), want) {
		return fmt.Errorf("platformStatus.%s: expected the rejection to contain %q, got: %v", key, want, invalid)
	}
	return nil
}

func dnsRecordsTypeForStatusKey(ps *configv1.PlatformStatus, key string) configv1.DNSRecordsType {
	if ps == nil {
		return ""
	}
	switch key {
	case "baremetal":
		if ps.BareMetal != nil {
			return ps.BareMetal.DNSRecordsType
		}
	case "openstack":
		if ps.OpenStack != nil {
			return ps.OpenStack.DNSRecordsType
		}
	case "vsphere":
		if ps.VSphere != nil {
			return ps.VSphere.DNSRecordsType
		}
	case "nutanix":
		if ps.Nutanix != nil {
			return ps.Nutanix.DNSRecordsType
		}
	case "ovirt":
		if ps.Ovirt != nil {
			return ps.Ovirt.DNSRecordsType
		}
	}
	return ""
}

// onPremDNSValidationKeys returns the platformStatus members the API
// validation tests patch. The cluster's own platform member is excluded: it
// already holds a load balancer type, which is immutable once set.
func onPremDNSValidationKeys(ps *configv1.PlatformStatus) []string {
	var own string
	if ps != nil {
		own = onPremDNSStatusKeys[ps.Type]
	}
	var keys []string
	for _, key := range []string{"baremetal", "openstack", "vsphere", "nutanix"} {
		if key != own {
			keys = append(keys, key)
		}
	}
	return keys
}

// onPremDNSCorefileRecords reports which cluster records a CoreDNS Corefile
// (or MCO's Corefile template) answers itself through template blocks.
type onPremDNSCorefileRecords struct {
	API    bool
	APIInt bool
	Apps   bool
}

func (r onPremDNSCorefileRecords) Any() bool { return r.API || r.APIInt || r.Apps }
func (r onPremDNSCorefileRecords) All() bool { return r.API && r.APIInt && r.Apps }
func (r onPremDNSCorefileRecords) String() string {
	return fmt.Sprintf("api=%t api-int=%t *.apps=%t", r.API, r.APIInt, r.Apps)
}

// parseOnPremDNSCorefile finds the template "match" lines MCO renders for the
// api, api-int and *.apps records when dnsRecordsType is Internal. hasHosts
// reports the node hosts block, which is rendered in both modes.
func parseOnPremDNSCorefile(corefile string) (records onPremDNSCorefileRecords, hasHosts bool) {
	for _, line := range strings.Split(corefile, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "hosts":
			hasHosts = true
		case fields[0] == "match" && len(fields) > 1:
			switch re := fields[1]; {
			case strings.HasPrefix(re, "^api-int."):
				records.APIInt = true
			case strings.HasPrefix(re, "^api."):
				records.API = true
			case strings.HasPrefix(re, ".*[.]apps."):
				records.Apps = true
			}
		}
	}
	return records, hasHosts
}

// checkOnPremDNSCorefile checks the Corefile answers the cluster records
// itself exactly when dnsRecordsType is Internal.
func checkOnPremDNSCorefile(corefile string, dnsRecordsType configv1.DNSRecordsType) error {
	records, hasHosts := parseOnPremDNSCorefile(corefile)
	if !hasHosts {
		return fmt.Errorf("the Corefile has no hosts block for the node records")
	}
	switch dnsRecordsType {
	case configv1.DNSRecordsTypeExternal:
		if records.Any() {
			return fmt.Errorf("dnsRecordsType is External but the Corefile still answers cluster records (%s)", records)
		}
	default:
		if !records.All() {
			return fmt.Errorf("dnsRecordsType is %s but the Corefile does not answer every cluster record (%s)", dnsRecordsType, records)
		}
	}
	return nil
}

// corefileTemplateFromRenderedMachineConfig returns the CoreDNS Corefile
// template from a rendered MachineConfig. found is false when the
// MachineConfig does not carry it.
func corefileTemplateFromRenderedMachineConfig(mc *unstructured.Unstructured) (corefile string, found bool, err error) {
	files, _, err := unstructured.NestedSlice(mc.Object, "spec", "config", "storage", "files")
	if err != nil {
		return "", false, fmt.Errorf("cannot read spec.config.storage.files of MachineConfig %s: %w", mc.GetName(), err)
	}
	for _, f := range files {
		file, ok := f.(map[string]interface{})
		if !ok || stringField(file, "path") != onPremDNSCorefileTemplatePath {
			continue
		}
		contents, _ := file["contents"].(map[string]interface{})
		data, err := decodeIgnitionSource(stringField(contents, "source"), stringField(contents, "compression"))
		if err != nil {
			return "", true, fmt.Errorf("cannot decode %s in MachineConfig %s: %w", onPremDNSCorefileTemplatePath, mc.GetName(), err)
		}
		return string(data), true, nil
	}
	return "", false, nil
}

// parseGetentAhosts returns the unique addresses in `getent ahosts` output.
func parseGetentAhosts(out string) []string {
	seen := sets.New[string]()
	var addrs []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || net.ParseIP(fields[0]) == nil || seen.Has(fields[0]) {
			continue
		}
		seen.Insert(fields[0])
		addrs = append(addrs, fields[0])
	}
	return addrs
}

// checkOnPremDNSResolution checks a name resolved, and when vips is not empty
// that every resolved address is one of them.
func checkOnPremDNSResolution(name string, addrs, vips []string) error {
	if len(addrs) == 0 {
		return fmt.Errorf("%s does not resolve", name)
	}
	if len(vips) == 0 {
		return nil
	}
	want := sets.New[string]()
	for _, vip := range vips {
		want.Insert(net.ParseIP(vip).String())
	}
	for _, a := range addrs {
		if !want.Has(net.ParseIP(a).String()) {
			return fmt.Errorf("%s resolves to %v, expected only the VIPs %v", name, addrs, vips)
		}
	}
	return nil
}
