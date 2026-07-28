package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/router/certgen"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	utilnet "k8s.io/utils/net"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// gatewayIngressNamespace is the namespace where the Gateway API/Istio
// operands (istiod, per-Gateway Envoy deployments and their fronting
// Services) run. Duplicated from test/extended/router/gatewayapicontroller.go's
// unexported "ingressNamespace" const since test/extended packages don't
// import each other.
const gatewayIngressNamespace = "openshift-ingress"

// gatewayClassControllerName is the Gateway API controller implemented by
// cluster-ingress-operator/sail-operator. Matches gatewayClassControllerName
// in test/extended/router/gatewayapicontroller.go.
const gatewayClassControllerName = "openshift.io/gateway-controller/v1"

// tlsAdherenceFeatureGateName is the FeatureGate that guards
// APIServer.spec.tlsAdherence (see vendor/github.com/openshift/api/features).
// GatewayAPIWithoutOLM is now GA, so unlike gatewayapicontroller.go's
// shouldSkipGatewayAPITests, no OLM-capability/feature-gate check is needed
// here - only TLSAdherence, which is still TechPreview/DevPreview-only.
const tlsAdherenceFeatureGateName = "TLSAdherence"

// isFeatureGateEnabled checks whether the named FeatureGate is enabled for
// the cluster's current desired version. Mirrors the version-aware pattern
// used by isNoOLMFeatureGateEnabled in gatewayapicontroller.go.
func isFeatureGateEnabled(oc *exutil.CLI, ctx context.Context, name string) (bool, error) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get ClusterVersion: %w", err)
	}
	currentVersion := cv.Status.Desired.Version

	fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get cluster FeatureGates: %w", err)
	}
	for _, fg := range fgs.Status.FeatureGates {
		if fg.Version != currentVersion {
			continue
		}
		for _, enabledFG := range fg.Enabled {
			if string(enabledFG.Name) == name {
				return true, nil
			}
		}
	}
	return false, nil
}

// canRunGatewayAPITLSTest determines whether the cluster can support the
// Gateway API wire-level TLS-profile-enforcement check. It returns false
// with a human-readable reason (not an error) when the check should simply
// be skipped, and a non-nil error only when the determination itself fails.
//
// Gateway API itself is GA (GatewayAPIWithoutOLM is stable), so this only
// needs to check platform/networking support for reaching a real
// LoadBalancer, plus the still-gated TLSAdherence field that Gateway TLS
// enforcement depends on (see PR openshift/cluster-ingress-operator#1480).
func canRunGatewayAPITLSTest(oc *exutil.CLI, ctx context.Context) (bool, string, error) {
	adherenceEnabled, err := isFeatureGateEnabled(oc, ctx, tlsAdherenceFeatureGateName)
	if err != nil {
		return false, "", fmt.Errorf("failed to check %s FeatureGate: %w", tlsAdherenceFeatureGateName, err)
	}
	if !adherenceEnabled {
		return false, fmt.Sprintf("%s FeatureGate is not enabled (requires TechPreviewNoUpgrade/DevPreviewNoUpgrade)", tlsAdherenceFeatureGateName), nil
	}

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to get infrastructure: %w", err)
	}
	if infra.Status.PlatformStatus == nil {
		return false, "", fmt.Errorf("infrastructure PlatformStatus is nil")
	}

	switch infra.Status.PlatformStatus.Type {
	case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType, configv1.IBMCloudPlatformType:
		// supported: these platforms provision a real LoadBalancer for the Gateway.
	default:
		return false, fmt.Sprintf("platform %q does not provision a LoadBalancer for Gateway API", infra.Status.PlatformStatus.Type), nil
	}

	networkConfig, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to get network config: %w", err)
	}
	for _, cidr := range networkConfig.Spec.ServiceNetwork {
		if utilnet.IsIPv6CIDRString(cidr) {
			return false, "cluster is IPv6/dual-stack", nil
		}
	}

	return true, "", nil
}

// gatewayListenerHostname is the SNI/certificate hostname on the Gateway
// HTTPS listener. Dial configs must set ServerName to this value because the
// target dials a LoadBalancer IP (or hostname) that does not match the cert.
const gatewayListenerHostname = "tls-observed-config-gw.example.com"

// gatewayLBTarget validates that a Gateway API Gateway's externally
// provisioned LoadBalancer enforces the cluster TLS profile at the wire
// level. Unlike endpointTarget, it dials the Gateway's real external
// address directly instead of port-forwarding to a pod.
//
// address is preferably a resolved IP so dials during reconciliation do not
// re-hit flaky cluster-DNS lookups of AWS ELB hostnames. serverName is the
// Gateway listener hostname used for SNI.
type gatewayLBTarget struct {
	address    string
	port       string
	serverName string
}

func (t gatewayLBTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
	if expected.tlsShouldWork == nil || expected.tlsShouldNotWork == nil {
		return fmt.Errorf("gateway LB %s: expected TLS configs are nil (minTLSVersion=%s)", t.address, expected.minTLSVersion)
	}

	hostPort := net.JoinHostPort(t.address, t.port)
	netDialer := &net.Dialer{Timeout: 10 * time.Second}

	// Clone so we can set ServerName without mutating shared expected configs.
	tlsShouldWork := expected.tlsShouldWork.Clone()
	tlsShouldNotWork := expected.tlsShouldNotWork.Clone()
	tlsShouldWork.ServerName = t.serverName
	tlsShouldNotWork.ServerName = t.serverName

	shouldWorkDialer := &tls.Dialer{NetDialer: netDialer, Config: tlsShouldWork}
	conn, err := shouldWorkDialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return fmt.Errorf("gateway LB %s: connection with min %s FAILED (expected success): %w",
			hostPort, tlsVersionName(tlsShouldWork.MinVersion), err)
	}
	conn.Close()

	shouldNotWorkDialer := &tls.Dialer{NetDialer: netDialer, Config: tlsShouldNotWork}
	conn, err = shouldNotWorkDialer.DialContext(ctx, "tcp", hostPort)
	if err == nil {
		conn.Close()
		return fmt.Errorf("gateway LB %s: connection with max %s should be REJECTED but succeeded",
			hostPort, tlsVersionName(tlsShouldNotWork.MaxVersion))
	}
	if !isTLSVersionRejectionError(err) {
		return fmt.Errorf("gateway LB %s: expected TLS version rejection for max %s, got: %w",
			hostPort, tlsVersionName(tlsShouldNotWork.MaxVersion), err)
	}

	e2e.Logf("gateway LB %s (SNI=%s): TLS PASS - accepts %s+, rejects %s",
		hostPort, t.serverName, tlsVersionName(tlsShouldWork.MinVersion), tlsVersionName(tlsShouldNotWork.MaxVersion))
	return nil
}

func (t gatewayLBTarget) key() string {
	return fmt.Sprintf("gatewayLB:%s", t.address)
}

// isTLSVersionRejectionError reports whether err looks like a TLS handshake
// rejection (or a connection close used in lieu of a TLS alert), rather than
// a network/DNS failure that must not count as version enforcement.
func isTLSVersionRejectionError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "protocol version") ||
		strings.Contains(errStr, "no supported versions") ||
		strings.Contains(errStr, "handshake failure") ||
		strings.Contains(errStr, "alert") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "connection reset by peer")
}

// gatewayTLSFixture tracks the resources created by setupGatewayTLSTarget so
// they can be torn down afterward.
type gatewayTLSFixture struct {
	oc                  *exutil.CLI
	gatewayName         string
	gatewayClassName    string
	secretName          string
	createdGatewayClass bool
}

// cleanup best-effort deletes everything this fixture created. It never
// fails the test - resource leaks here are logged, not fatal, since they
// don't affect the validity of a result that already passed or failed.
func (f *gatewayTLSFixture) cleanup(ctx context.Context) {
	if f.gatewayName != "" {
		if err := f.oc.AdminGatewayApiClient().GatewayV1().Gateways(gatewayIngressNamespace).Delete(ctx, f.gatewayName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			e2e.Logf("gatewayTLSFixture cleanup: failed to delete Gateway %s: %v", f.gatewayName, err)
		}
	}
	if f.secretName != "" {
		if err := f.oc.AdminKubeClient().CoreV1().Secrets(gatewayIngressNamespace).Delete(ctx, f.secretName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			e2e.Logf("gatewayTLSFixture cleanup: failed to delete Secret %s: %v", f.secretName, err)
		}
	}
	if f.createdGatewayClass && f.gatewayClassName != "" {
		if err := f.oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Delete(ctx, f.gatewayClassName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			e2e.Logf("gatewayTLSFixture cleanup: failed to delete GatewayClass %s: %v", f.gatewayClassName, err)
		}
	}
}

// ensureGatewayClass returns the name of a GatewayClass using our
// controller, reusing one if any already exists (e.g. from a previous run,
// another suite, or one auto-managed by cluster-ingress-operator) rather
// than assuming a fixed name. Only when none exists does it create one,
// using GenerateName so concurrent/repeated runs can never collide on a
// hardcoded name.
func ensureGatewayClass(oc *exutil.CLI, ctx context.Context) (name string, created bool, err error) {
	list, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to list GatewayClasses: %w", err)
	}
	for _, gwc := range list.Items {
		if string(gwc.Spec.ControllerName) == gatewayClassControllerName {
			e2e.Logf("reusing existing GatewayClass %s", gwc.Name)
			return gwc.Name, false, nil
		}
	}

	gatewayClass := &gatewayapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "tls-observed-config-gwclass-"},
		Spec: gatewayapiv1.GatewayClassSpec{
			ControllerName: gatewayapiv1.GatewayController(gatewayClassControllerName),
		},
	}
	created2, err := oc.AdminGatewayApiClient().GatewayV1().GatewayClasses().Create(ctx, gatewayClass, metav1.CreateOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to create GatewayClass: %w", err)
	}
	return created2.Name, true, nil
}

// setupGatewayTLSTarget provisions a GatewayClass (reusing one if it already
// exists), a Gateway with an HTTPS listener backed by a self-signed
// certificate, and waits for its LoadBalancer Service to be assigned an
// external address. It returns a tlsTarget that dials that address directly,
// plus a fixture whose cleanup() should be deferred by the caller.
func setupGatewayTLSTarget(oc *exutil.CLI, ctx context.Context) (gatewayLBTarget, *gatewayTLSFixture, error) {
	fixture := &gatewayTLSFixture{oc: oc}

	g.By("ensuring a GatewayClass exists for the Gateway TLS check")
	gatewayClassName, createdGatewayClass, err := ensureGatewayClass(oc, ctx)
	if err != nil {
		return gatewayLBTarget{}, fixture, err
	}
	fixture.gatewayClassName = gatewayClassName
	fixture.createdGatewayClass = createdGatewayClass

	g.By("generating a self-signed certificate for the Gateway HTTPS listener")
	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)
	hostname := gatewayListenerHostname

	// privateKey/pemKey hold key material only for as long as it takes to
	// populate the Secret below; zero/drop them on the way out (including
	// error returns) rather than leaving them for the GC to collect on its
	// own schedule.
	var privateKey *ecdsa.PrivateKey
	var pemKey string
	defer func() {
		if privateKey != nil && privateKey.D != nil {
			privateKey.D.SetInt64(0)
		}
		pemKey = ""
	}()

	var certDER []byte
	_, certDER, privateKey, err = certgen.GenerateKeyPair(hostname, notBefore, notAfter, hostname)
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to generate certificate: %w", err)
	}
	pemKey, err = certgen.MarshalPrivateKeyToPEMString(privateKey)
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to marshal private key: %w", err)
	}
	pemCrt, err := certgen.MarshalCertToPEMString(certDER)
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to marshal certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "tls-observed-config-gw-cert-", Namespace: gatewayIngressNamespace},
		StringData: map[string]string{"tls.crt": pemCrt, "tls.key": pemKey},
		Type:       corev1.SecretTypeTLS,
	}
	createdSecret, err := oc.AdminKubeClient().CoreV1().Secrets(gatewayIngressNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to create TLS Secret: %w", err)
	}
	fixture.secretName = createdSecret.Name

	g.By("creating Gateway with an HTTPS listener")
	tlsMode := gatewayapiv1.TLSModeTerminate
	gwHostname := gatewayapiv1.Hostname(hostname)
	gateway := &gatewayapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "tls-observed-config-gw-", Namespace: gatewayIngressNamespace},
		Spec: gatewayapiv1.GatewaySpec{
			GatewayClassName: gatewayapiv1.ObjectName(gatewayClassName),
			Listeners: []gatewayapiv1.Listener{
				{
					Name:     "https",
					Hostname: &gwHostname,
					Port:     443,
					Protocol: gatewayapiv1.HTTPSProtocolType,
					TLS: &gatewayapiv1.ListenerTLSConfig{
						Mode:            &tlsMode,
						CertificateRefs: []gatewayapiv1.SecretObjectReference{{Name: gatewayapiv1.ObjectName(createdSecret.Name)}},
					},
				},
			},
		},
	}
	createdGateway, err := oc.AdminGatewayApiClient().GatewayV1().Gateways(gatewayIngressNamespace).Create(ctx, gateway, metav1.CreateOptions{})
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to create Gateway: %w", err)
	}
	fixture.gatewayName = createdGateway.Name

	g.By(fmt.Sprintf("waiting for Gateway %s to be Programmed", createdGateway.Name))
	if err := waitForGatewayProgrammed(oc, ctx, createdGateway.Name); err != nil {
		return gatewayLBTarget{}, fixture, err
	}

	g.By(fmt.Sprintf("waiting for Gateway %s LoadBalancer Service to get an external address", createdGateway.Name))
	address, err := waitForGatewayLoadBalancerAddress(oc, ctx, createdGateway.Name)
	if err != nil {
		return gatewayLBTarget{}, fixture, err
	}

	return gatewayLBTarget{address: address, port: "443", serverName: hostname}, fixture, nil
}

// waitForGatewayProgrammed polls the Gateway's status conditions until
// Programmed=True. Mirrors checkGatewayStatus in gatewayapicontroller.go,
// restricted to the Programmed condition since canRunGatewayAPITLSTest
// already requires LoadBalancer-supporting platforms.
func waitForGatewayProgrammed(oc *exutil.CLI, ctx context.Context, gatewayName string) error {
	const timeout = 20 * time.Minute
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true,
		func(ctx context.Context) (bool, error) {
			gateway, err := oc.AdminGatewayApiClient().GatewayV1().Gateways(gatewayIngressNamespace).Get(ctx, gatewayName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("waiting for Gateway %s: %v", gatewayName, err)
				return false, nil
			}
			for _, condition := range gateway.Status.Conditions {
				if condition.Type == string(gatewayapiv1.GatewayConditionProgrammed) && condition.Status == metav1.ConditionTrue {
					return true, nil
				}
			}
			return false, nil
		})
}

// gatewayNameLabelKey is the label Istio adds to the per-Gateway LoadBalancer
// Service (and that cluster-ingress-operator uses to associate DNSRecords).
// Discovering the Service by this label avoids hardcoding the
// "<gateway>-<gatewayclass>" naming convention.
const gatewayNameLabelKey = "gateway.networking.k8s.io/gateway-name"

// waitForGatewayLoadBalancerAddress finds the LoadBalancer Service for the
// given Gateway via label selector and polls until it reports an external
// hostname or IP. Mirrors assertGatewayLoadbalancerReady in
// gatewayapicontroller.go, but without depending on Service name conventions.
//
// When the LoadBalancer publishes a hostname (as on AWS), the Service status
// can report it before it's actually resolvable in public DNS. This waits for
// resolution and returns a pinned IP so later dials during reconciliation do
// not re-resolve through flaky cluster DNS (which previously caused
// "lookup ... i/o timeout" false failures).
func waitForGatewayLoadBalancerAddress(oc *exutil.CLI, ctx context.Context, gatewayName string) (string, error) {
	const timeout = 10 * time.Minute
	selector := gatewayNameLabelKey + "=" + gatewayName
	var address string
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true,
		func(ctx context.Context) (bool, error) {
			svcs, err := oc.AdminKubeClient().CoreV1().Services(gatewayIngressNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: selector,
			})
			if err != nil {
				e2e.Logf("waiting for Gateway %s Service (selector %s): %v", gatewayName, selector, err)
				return false, nil
			}
			if len(svcs.Items) == 0 {
				e2e.Logf("waiting for Gateway %s Service (selector %s): not found yet", gatewayName, selector)
				return false, nil
			}
			if len(svcs.Items) > 1 {
				e2e.Logf("waiting for Gateway %s Service: found %d matches for %s, using first", gatewayName, len(svcs.Items), selector)
			}
			svc := svcs.Items[0]
			if len(svc.Status.LoadBalancer.Ingress) == 0 {
				e2e.Logf("waiting for Service %s: no LoadBalancer ingress yet", svc.Name)
				return false, nil
			}
			ingress := svc.Status.LoadBalancer.Ingress[0]
			if ingress.Hostname == "" {
				address = ingress.IP
				return address != "", nil
			}
			ips, err := net.DefaultResolver.LookupHost(ctx, ingress.Hostname)
			if err != nil || len(ips) == 0 {
				e2e.Logf("waiting for Service %s: hostname %s not yet resolvable: %v", svc.Name, ingress.Hostname, err)
				return false, nil
			}
			// Pin the first resolved IP so dials skip DNS.
			address = ips[0]
			e2e.Logf("Service %s hostname %s resolved to %s (pinned for dials)", svc.Name, ingress.Hostname, address)
			return true, nil
		})
	if err != nil {
		return "", fmt.Errorf("timed out waiting for Gateway %s Service (selector %s) to get a LoadBalancer address: %w", gatewayName, selector, err)
	}
	return address, nil
}
