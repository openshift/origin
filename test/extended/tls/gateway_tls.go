package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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

// gatewayLBTarget validates that a Gateway API Gateway's externally
// provisioned LoadBalancer enforces the cluster TLS profile at the wire
// level. Unlike endpointTarget, it dials the Gateway's real external
// address directly instead of port-forwarding to a pod.
type gatewayLBTarget struct {
	address string
	port    string
}

func (t gatewayLBTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
	hostPort := fmt.Sprintf("%s:%s", t.address, t.port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	conn, err := tls.DialWithDialer(dialer, "tcp", hostPort, expected.tlsShouldWork)
	if err != nil {
		return fmt.Errorf("gateway LB %s: connection with min %s FAILED (expected success): %w",
			hostPort, tlsVersionName(expected.tlsShouldWork.MinVersion), err)
	}
	conn.Close()

	conn, err = tls.DialWithDialer(dialer, "tcp", hostPort, expected.tlsShouldNotWork)
	if err == nil {
		conn.Close()
		return fmt.Errorf("gateway LB %s: connection with max %s should be REJECTED but succeeded",
			hostPort, tlsVersionName(expected.tlsShouldNotWork.MaxVersion))
	}

	e2e.Logf("gateway LB %s: TLS PASS - accepts %s+, rejects %s",
		hostPort, tlsVersionName(expected.tlsShouldWork.MinVersion), tlsVersionName(expected.tlsShouldNotWork.MaxVersion))
	return nil
}

func (t gatewayLBTarget) key() string {
	return fmt.Sprintf("gatewayLB:%s", t.address)
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
	hostname := "tls-observed-config-gw.example.com"
	_, certDER, privateKey, err := certgen.GenerateKeyPair(hostname, notBefore, notAfter, hostname)
	if err != nil {
		return gatewayLBTarget{}, fixture, fmt.Errorf("failed to generate certificate: %w", err)
	}
	pemKey, err := certgen.MarshalPrivateKeyToPEMString(privateKey)
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

	serviceName := createdGateway.Name + "-" + gatewayClassName
	g.By(fmt.Sprintf("waiting for Gateway LoadBalancer Service %s to get an external address", serviceName))
	address, err := waitForLoadBalancerAddress(oc, ctx, serviceName)
	if err != nil {
		return gatewayLBTarget{}, fixture, err
	}

	return gatewayLBTarget{address: address, port: "443"}, fixture, nil
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

// waitForLoadBalancerAddress polls the given Service until its LoadBalancer
// status reports an external hostname or IP, and returns it. Mirrors
// assertGatewayLoadbalancerReady in gatewayapicontroller.go.
func waitForLoadBalancerAddress(oc *exutil.CLI, ctx context.Context, serviceName string) (string, error) {
	const timeout = 10 * time.Minute
	var address string
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true,
		func(ctx context.Context) (bool, error) {
			svc, err := oc.AdminKubeClient().CoreV1().Services(gatewayIngressNamespace).Get(ctx, serviceName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("waiting for Service %s: %v", serviceName, err)
				return false, nil
			}
			if len(svc.Status.LoadBalancer.Ingress) == 0 {
				return false, nil
			}
			ingress := svc.Status.LoadBalancer.Ingress[0]
			if ingress.Hostname != "" {
				address = ingress.Hostname
			} else {
				address = ingress.IP
			}
			return address != "", nil
		})
	if err != nil {
		return "", fmt.Errorf("timed out waiting for Service %s to get a LoadBalancer address: %w", serviceName, err)
	}
	return address, nil
}
