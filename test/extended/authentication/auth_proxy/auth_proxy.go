package auth_proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	libcrypto "github.com/openshift/library-go/pkg/crypto"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/openshift/origin/test/extended/util/image"
)

var (
	// ErrInvalidCLI indicates a missing CLI or a nil component receiver.
	ErrInvalidCLI = errors.New("invalid CLI")
	// ErrMissingNamespace indicates a component without a namespace to operate on.
	ErrMissingNamespace = errors.New("authentication proxy component doesn't hold namespace information")
	// ErrProxyCertificateRotationIncomplete marks failures after the TLS Secret was updated.
	ErrProxyCertificateRotationIncomplete = errors.New("proxy certificate rotation incomplete: TLS Secret was updated")
)

// AuthenticationProxyConfig controls a forward proxy deployment. A nil ClusterProxy means
// direct connections; a non-nil configuration must specify at least one proxy URL.
type AuthenticationProxyConfig struct {
	ClusterProxy *ProxyConfig
}

// squidProxyUpstreamConfig contains peer directives and the optional trust bundle.
type squidProxyUpstreamConfig struct {
	squidConfig       string
	upstreamCACertPEM []byte
}

// AuthenticationProxyComponent identifies an allocated proxy namespace, its URLs,
// and its current CA. A partial component returned on deployment failure can be
// closed when Namespace is non-empty. Copies share URL pointers and CA bytes;
// certificate rotation updates CACertPEM only on the receiver being rotated.
type AuthenticationProxyComponent struct {
	oc        *exutil.CLI
	Namespace string

	config            AuthenticationProxyConfig
	HttpUrl, HttpsUrl *url.URL
	CACertPEM         []byte
}

// upstreamProxyConfig builds Squid peer and bypass directives without cluster access.
// A nil upstream produces no directives. An invalid upstream returns an empty result.
func (ac AuthenticationProxyConfig) upstreamProxyConfig() (squidProxyUpstreamConfig, error) {
	g.GinkgoHelper()
	var squidAdditionalConfig strings.Builder
	if ac.ClusterProxy != nil {
		if !ac.ClusterProxy.HasProxy() {
			return squidProxyUpstreamConfig{}, fmt.Errorf("either HttpsProxy or HttpProxy in ClusterProxy must be non-empty")
		}

		httpsProxy, httpProxy := ac.ClusterProxy.HttpsProxy, ac.ClusterProxy.HttpProxy
		squidAdditionalConfig.WriteString("acl upstream_connect method CONNECT\n")
		peers := []struct {
			url       *url.URL
			name, acl string
		}{
			{
				name: "upstream_http",
				url:  httpProxy,
				acl:  "!upstream_connect",
			},
			{
				name: "upstream_https",
				url:  httpsProxy,
				acl:  "upstream_connect",
			},
		}

		if proxyURLsEqual(httpProxy, httpsProxy) {
			peers = peers[:1]
			peers[0].acl = "all"
		}

		for _, peer := range peers {
			if peer.url == nil {
				fmt.Fprintf(&squidAdditionalConfig, "always_direct allow %s\n", peer.acl)
				continue
			}

			if peer.url.Hostname() == "" ||
				(peer.url.Scheme != "http" && peer.url.Scheme != "https") {
				return squidProxyUpstreamConfig{}, fmt.Errorf("%s must be a valid HTTP or HTTPS proxy URL", peer.name)
			}
			port := peer.url.Port()
			if port == "" {
				port = "80"
				if peer.url.Scheme == "https" {
					port = "443"
				}
			}
			peerOptions := "no-query name=" + peer.name
			if peer.url.Scheme == "https" {
				peerOptions += " tls"
				if len(ac.ClusterProxy.UpstreamCACertPEM) > 0 {
					peerOptions += " tls-cafile=" + upstreamCACertPath
				}
			}
			if peer.url.User != nil {
				proxyUsername := peer.url.User.Username()
				proxyPassword, _ := peer.url.User.Password()

				// Squid accepts URL escapes in login credentials, but not spaces.
				username := strings.ReplaceAll(url.QueryEscape(proxyUsername), "+", "%20")
				proxyPassword = strings.ReplaceAll(url.QueryEscape(proxyPassword), "+", "%20")
				peerOptions += " login=" + username + ":" + proxyPassword
			}
			fmt.Fprintf(&squidAdditionalConfig, "cache_peer %s parent %s 0 %s\n", peer.url.Hostname(), port, peerOptions)
			fmt.Fprintf(&squidAdditionalConfig, "cache_peer_access %s allow %s\ncache_peer_access %s deny all\n", peer.name, peer.acl, peer.name)
		}

		// Internal destinations must remain reachable without the global proxy.
		for i, entry := range ac.ClusterProxy.NoProxy.UnsortedList() {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if entry == "*" {
				squidAdditionalConfig.WriteString("always_direct allow all\n")
				continue
			}
			aclType := "dstdomain -n"
			if _, _, cidrErr := net.ParseCIDR(entry); cidrErr == nil || net.ParseIP(entry) != nil {
				aclType = "dst"
			} else {
				// A bare noProxy domain also excludes its subdomains.
				entry = "." + strings.TrimPrefix(strings.TrimPrefix(entry, "*."), ".")
			}
			aclName := fmt.Sprintf("global_no_proxy_%d", i)
			fmt.Fprintf(&squidAdditionalConfig, "acl %s %s %s\nalways_direct allow %s\n", aclName, aclType, entry, aclName)
		}
		squidAdditionalConfig.WriteString("never_direct allow all\n")
	}

	result := squidProxyUpstreamConfig{
		squidConfig: squidAdditionalConfig.String(),
	}
	if ac.ClusterProxy != nil {
		result.upstreamCACertPEM = ac.ClusterProxy.UpstreamCACertPEM
	}
	return result, nil
}

// DeployAuthProxyComponent creates a namespace and deploys a proxy on HTTP port 3128
// and HTTPS port 3129, with a generated CA and serving certificate. It validates
// the configuration before cluster access and waits up to five minutes for readiness.
// It stores the proxy configuration in a Secret because upstream settings can contain credentials.
// oc must be initialized. Errors after namespace creation retain the namespace and
// CLI in the returned component; the caller must Close it with an active context.
func (ac AuthenticationProxyConfig) DeployAuthProxyComponent(ctx context.Context, oc *exutil.CLI) (authProxy AuthenticationProxyComponent, err error) {
	if oc == nil {
		return authProxy, ErrInvalidCLI
	}
	if err := ctx.Err(); err != nil {
		return authProxy, err
	}
	additionalUpstreamProxyConfig, err := ac.upstreamProxyConfig()
	if err != nil {
		return authProxy, fmt.Errorf("error creating configuration from cluster proxy: %w", err)
	}

	authProxy.oc = oc
	kubeClient := oc.AdminKubeClient()

	namespace, err := kubeClient.CoreV1().
		Namespaces().
		Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-proxy-",
				Labels:       ComponentProxyTestLabels(),
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error on creating auth proxy component namespace: %w", err)
	}
	authProxy.Namespace = namespace.Name

	caConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	if err != nil {
		return authProxy, fmt.Errorf("error on creating proxy CA certificate: %w", err)
	}
	ca := &libcrypto.CA{
		Config:          caConfig,
		SerialGenerator: &libcrypto.RandomSerialGenerator{},
	}

	serviceHost := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, namespace.Name)
	serverCertConfig, err := ca.MakeServerCert(sets.New(serviceHost), 2*time.Hour)
	if err != nil {
		return authProxy, fmt.Errorf("error creating proxy server cert: %w", err)
	}

	caCertPEM, _, err := caConfig.GetPEMBytes()
	if err != nil {
		return authProxy, fmt.Errorf("error encoding proxy CA cert: %w", err)
	}

	authProxy.CACertPEM = caCertPEM
	serverCertPEM, serverKeyPEM, err := serverCertConfig.GetPEMBytes()
	if err != nil {
		return authProxy, fmt.Errorf("error encoding proxy server cert: %w", err)
	}

	// logformat fields: method url sourceIP squidStatus httpCode — parsed by waitForProxyTrafficFrom
	squidConfig := fmt.Sprintf(`http_port %d
https_port %d tls-cert=/etc/squid/tls/tls.crt tls-key=/etc/squid/tls/tls.key
pid_filename /tmp/squid.pid
acl all src all
http_access allow all
logformat proxytest %%rm %%ru %%>a %%Ss %%>Hs
access_log stdio:/dev/stdout proxytest
cache_log stdio:/dev/stderr
cache deny all
buffered_logs off
`, squidHTTPPort, squidHTTPSPort)

	squidConfig += additionalUpstreamProxyConfig.squidConfig

	_, err = kubeClient.CoreV1().
		Secrets(authProxy.Namespace).
		Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "squid-config"},
			StringData: map[string]string{"squid.conf": squidConfig},
		}, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy config: %w", err)
	}

	_, err = kubeClient.CoreV1().
		Secrets(authProxy.Namespace).
		Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "squid-tls"},
			Data: map[string][]byte{
				"tls.crt": serverCertPEM,
				"tls.key": serverKeyPEM,
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy TLS Secret: %w", err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   squidServiceName,
			Labels: map[string]string{"app": squidServiceName},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": squidServiceName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": squidServiceName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "squid",
							Image: image.LocationFor(squidImage),
							Ports: []corev1.ContainerPort{
								{ContainerPort: squidHTTPPort, Protocol: corev1.ProtocolTCP},
								{ContainerPort: squidHTTPSPort, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "squid-config", MountPath: "/etc/squid/squid.conf", SubPath: "squid.conf"},
								{Name: "squid-tls", MountPath: "/etc/squid/tls", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(squidHTTPPort),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "squid-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "squid-config",
								},
							},
						},
						{
							Name: "squid-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: "squid-tls"},
							},
						},
					},
				},
			},
		},
	}

	pemData := additionalUpstreamProxyConfig.upstreamCACertPEM
	if len(pemData) > 0 {
		_, err = kubeClient.CoreV1().
			ConfigMaps(authProxy.Namespace).
			Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: upstreamCAConfigMap,
				},
				Data: map[string]string{
					upstreamCAFile: string(pemData),
				},
			}, metav1.CreateOptions{})
		if err != nil {
			return authProxy, fmt.Errorf("error creating authentication proxy upstream CA ConfigMap: %w", err)
		}

		deploymentSpec := &deployment.Spec.Template.Spec
		deploymentSpec.Containers[0].VolumeMounts = append(deploymentSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      upstreamCAConfigMap,
			MountPath: upstreamCACertPath,
			SubPath:   upstreamCAFile,
			ReadOnly:  true,
		})
		deploymentSpec.Volumes = append(deploymentSpec.Volumes, corev1.Volume{
			Name: upstreamCAConfigMap,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: upstreamCAConfigMap,
					},
				},
			},
		})
	}

	_, err = kubeClient.AppsV1().
		Deployments(authProxy.Namespace).
		Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy deployment: %w", err)
	}

	_, err = kubeClient.CoreV1().
		Services(authProxy.Namespace).
		Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   squidServiceName,
				Labels: map[string]string{"app": squidServiceName},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": squidServiceName},
				Ports: []corev1.ServicePort{
					{Name: "http", Port: squidHTTPPort, TargetPort: intstr.FromInt32(squidHTTPPort), Protocol: corev1.ProtocolTCP},
					{Name: "https", Port: squidHTTPSPort, TargetPort: intstr.FromInt32(squidHTTPSPort), Protocol: corev1.ProtocolTCP},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy service: %w", err)
	}

	g.GinkgoWriter.Printf("waiting for authentication proxy deployment in %s to be ready\n", namespace)
	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	_, err = watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			kubeClient.AppsV1().RESTClient(), "deployments", authProxy.Namespace,
			fields.OneTermEqualSelector("metadata.name", squidServiceName)),
		&appsv1.Deployment{},
		nil,
		func(event watch.Event) (bool, error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("authentication proxy deployment watch error: %v", event.Object)
			}
			if event.Type == watch.Bookmark {
				return false, nil
			}
			d, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				return false, nil
			}
			return d.Status.ReadyReplicas > 0, nil
		},
	)
	if err != nil {
		return authProxy, fmt.Errorf("authentication proxy deployment in namespace %q did not become ready: %w", authProxy.Namespace, err)
	}

	authProxy.HttpUrl, err = url.Parse("http://" + net.JoinHostPort(serviceHost, strconv.Itoa(int(squidHTTPPort))))
	if err != nil {
		return authProxy, fmt.Errorf("error parsing http proxy url: %w", err)
	}

	authProxy.HttpsUrl, err = url.Parse("https://" + net.JoinHostPort(serviceHost, strconv.Itoa(int(squidHTTPSPort))))
	if err != nil {
		return authProxy, fmt.Errorf("error parsing https proxy url: %w", err)
	}
	g.GinkgoWriter.Printf("Authentication proxy deployed: %q\n", authProxy)
	return authProxy, nil
}

// String describes the namespace, credential-free URLs, and CA length without
// changing the component. It also accepts a zero or partially initialized component.
func (aP AuthenticationProxyComponent) String() string {
	return fmt.Sprintf("AuthenticationProxy[Namespace=%q HttpProxy=%q HttpsProxy=%q CACertLen=%d]",
		aP.Namespace,
		stripCredentials(aP.HttpUrl),
		stripCredentials(aP.HttpsUrl),
		len(aP.CACertPEM),
	)
}

// Delete deletes the proxy namespace and waits for its removal. The timeout covers
// both the deletion request and the wait. An already absent namespace is a success.
// The component must have a CLI and namespace, and timeout must be positive.
// If the wait fails, the error reports that deletion was requested but not confirmed
// and preserves both the context error and the last read error, when present.
// Delete retains component fields and can be repeated after success or a failed wait.
func (aP AuthenticationProxyComponent) Delete(ctx context.Context, timeout time.Duration) error {
	if aP.oc == nil {
		return ErrInvalidCLI
	}
	if aP.Namespace == "" {
		return ErrMissingNamespace
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero: %s", timeout)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	closeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	kubeClient := aP.oc.AdminKubeClient()
	return closeProxyNamespace(closeCtx, kubeClient.CoreV1().Namespaces(), aP.Namespace, 5*time.Second)
}

// closeProxyNamespace requests foreground deletion and polls with the supplied
// client and interval. The caller must provide a context deadline to bound the wait.
func closeProxyNamespace(ctx context.Context, namespaces coreclientv1.NamespaceInterface, namespace string, interval time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.GinkgoWriter.Printf("attempting to remove authentication proxy component in namespace %q\n", namespace)
	err := namespaces.Delete(ctx, namespace, metav1.DeleteOptions{
		PropagationPolicy: new(metav1.DeletePropagationForeground),
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("requesting deletion of proxy namespace %q: %w", namespace, err)
	}

	g.GinkgoWriter.Printf("waiting for namespace %q to be fully deleted\n", namespace)
	var lastReadErr error
	err = wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		_, err := namespaces.Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			lastReadErr = err
			g.GinkgoWriter.Printf("error checking namespace %s: %v\n", namespace, err)
		}
		return false, nil
	})
	if err != nil {
		if lastReadErr != nil {
			err = errors.Join(err, fmt.Errorf("last namespace read failed: %w", lastReadErr))
		}
		return fmt.Errorf("proxy namespace %q deletion was requested but completion was not confirmed: %w", namespace, err)
	}
	return nil
}

// logsSince reads the squid container log from the first pod matching the proxy
// label. A zero since includes all available logs. Read failures are not retried here.
func (aP AuthenticationProxyComponent) logsSince(
	ctx context.Context,
	since time.Time,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	kubeClient := aP.oc.AdminKubeClient()

	pods, err := kubeClient.CoreV1().
		Pods(aP.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", squidServiceName),
		})
	if err != nil {
		return "", fmt.Errorf("error listing auth proxy pods in namespace %q: %w", aP.Namespace, err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no auth proxy pods found in namespace %q", aP.Namespace)
	}

	logOpts := &corev1.PodLogOptions{Container: "squid"}
	if !since.IsZero() {
		t := metav1.NewTime(since)
		logOpts.SinceTime = &t
	}
	logBytes, err := kubeClient.CoreV1().
		Pods(aP.Namespace).
		GetLogs(pods.Items[0].Name, logOpts).DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting logs from auth proxy container in namespace %q: %w", aP.Namespace, err)
	}

	return string(logBytes), nil
}

// WaitForProxyTrafficFrom waits for a CONNECT tunnel from sourceIP in the proxy
// access log. sourceIP must be an IPv4 or IPv6 address and timeout must be positive.
// The component must have a CLI and namespace. A zero since includes all available
// logs. Log read failures are retried; failure preserves the context error and the
// last read error, when present.
func (aP AuthenticationProxyComponent) WaitForProxyTrafficFrom(
	ctx context.Context,
	sourceIP string,
	since time.Time,
	timeout time.Duration,
) error {
	g.GinkgoHelper()
	sourceIPs := []string{sourceIP}
	if err := aP.validateProxyTrafficWait(ctx, sourceIPs, timeout); err != nil {
		return err
	}
	g.GinkgoWriter.Printf("waiting up to %s for proxy traffic from %s\n", timeout, sourceIP)
	return waitForProxyTraffic(ctx, timeout, 10*time.Second,
		func(ctx context.Context) (string, error) { return aP.logsSince(ctx, since) },
		func(logs string) bool { return proxyTrafficMatches(logs, sourceIPs, "") })
}

// WaitForProxyTrafficFromTo waits for a CONNECT tunnel from any sourceIPs to
// destHost. destHost must be a hostname or IP without a scheme, path, or port.
// sourceIPs must be non-empty and contain only IPv4 or IPv6 addresses; timeout
// must be positive. The component must have a CLI and namespace. Hostnames are
// matched in full, ignoring DNS case and a trailing dot; IPs compare by address.
// A zero since includes all logs. Read failures are retried and preserved with the
// context error if the wait fails.
func (aP AuthenticationProxyComponent) WaitForProxyTrafficFromTo(
	ctx context.Context,
	sourceIPs []string,
	destHost string,
	since time.Time,
	timeout time.Duration,
) error {
	g.GinkgoHelper()
	if err := aP.validateProxyTrafficWait(ctx, sourceIPs, timeout); err != nil {
		return err
	}
	if !validProxyHost(destHost) {
		return fmt.Errorf("destination must be a hostname or IP without a scheme, path, or port: %q", destHost)
	}
	g.GinkgoWriter.Printf("waiting up to %s for proxy traffic from %v to %s\n", timeout, sourceIPs, destHost)
	return waitForProxyTraffic(ctx, timeout, 10*time.Second,
		func(ctx context.Context) (string, error) { return aP.logsSince(ctx, since) },
		func(logs string) bool { return proxyTrafficMatches(logs, sourceIPs, destHost) })
}

// validateProxyTrafficWait checks component state and polling inputs before any I/O.
func (aP AuthenticationProxyComponent) validateProxyTrafficWait(ctx context.Context, sourceIPs []string, timeout time.Duration) error {
	if aP.oc == nil {
		return ErrInvalidCLI
	}
	if aP.Namespace == "" {
		return ErrMissingNamespace
	}
	if len(sourceIPs) == 0 {
		return fmt.Errorf("at least one source IP is required")
	}
	for _, ip := range sourceIPs {
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid source IP: %q", ip)
		}
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero: %s", timeout)
	}
	return ctx.Err()
}

// proxyTrafficMatches looks for a CONNECT TCP_TUNNEL entry from an allowed address.
// An empty destHost accepts any valid CONNECT destination. Malformed entries are skipped.
func proxyTrafficMatches(logs string, sourceIPs []string, destHost string) bool {
	allowedIPs := sets.New[string]()
	for _, sourceIP := range sourceIPs {
		if ip := net.ParseIP(sourceIP); ip != nil {
			allowedIPs.Insert(ip.String())
		}
	}
	for line := range strings.SplitSeq(logs, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "CONNECT" || parts[3] != "TCP_TUNNEL" {
			continue
		}
		sourceIP := net.ParseIP(parts[2])
		if sourceIP == nil || !allowedIPs.Has(sourceIP.String()) {
			continue
		}
		// CONNECT destinations use host:port, including brackets for IPv6.
		host, port, err := net.SplitHostPort(parts[1])
		if err != nil || !validProxyHost(host) {
			continue
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber <= 0 || portNumber > 65535 {
			continue
		}
		if destHost != "" {
			if destIP := net.ParseIP(destHost); destIP != nil {
				if !destIP.Equal(net.ParseIP(host)) {
					continue
				}
			} else if !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(destHost, ".")) {
				continue
			}
		}
		g.GinkgoWriter.Printf("confirmed proxy traffic: %s\n", line)
		return true
	}
	return false
}

// validProxyHost accepts an IP or DNS hostname, including a trailing DNS dot.
func validProxyHost(host string) bool {
	return net.ParseIP(host) != nil || len(validation.IsDNS1123Subdomain(strings.ToLower(strings.TrimSuffix(host, ".")))) == 0
}

// waitForProxyTraffic polls readLogs until matches succeeds or the context expires.
// Callbacks and durations must be valid. Read failures are retried; the last one
// is joined with the termination error. A later match returns success.
func waitForProxyTraffic(ctx context.Context, timeout, interval time.Duration, readLogs func(context.Context) (string, error), matches func(string) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var lastReadErr error
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		logs, err := readLogs(ctx)
		if err != nil {
			lastReadErr = err
			g.GinkgoWriter.Printf("failed to read auth proxy logs: %v\n", err)
			return false, nil
		}
		return matches(logs), nil
	})
	if err != nil && lastReadErr != nil {
		return errors.Join(err, fmt.Errorf("last proxy log read failed: %w", lastReadErr))
	}
	return err
}

// RotateProxyCertificates rotates the proxy serving certificates and updates CACertPEM.
// The caller must distribute the returned CA to consumers of the proxy.
// The receiver must have a CLI and namespace. Before writes, the method requires
// non-empty tls.crt and tls.key entries in the TLS Secret and at least one proxy pod.
// It waits up to three minutes for the selected pod to see the new certificate,
// then reconfigures the proxy. CACertPEM changes only after successful reconfiguration.
// ErrProxyCertificateRotationIncomplete means the Secret was updated but applying
// the certificates failed. CACertPEM is unchanged and no rollback is attempted.
func (aP *AuthenticationProxyComponent) RotateProxyCertificates(ctx context.Context) ([]byte, error) {
	if aP == nil || aP.oc == nil {
		return nil, ErrInvalidCLI
	}
	if aP.Namespace == "" {
		return nil, ErrMissingNamespace
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	kubeClient := aP.oc.AdminKubeClient()
	adminClient := aP.oc.AsAdmin()

	tlsSecret, squidPod, err := aP.certificateRotationResources(ctx, kubeClient.CoreV1())
	if err != nil {
		return nil, err
	}

	newCAConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("error creating new CA certificate: %w", err)
	}
	newCA := &libcrypto.CA{Config: newCAConfig, SerialGenerator: &libcrypto.RandomSerialGenerator{}}

	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, aP.Namespace)
	newServerCertConfig, err := newCA.MakeServerCert(sets.New(serviceDNS), 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("error creating authentication proxy component certificate: %w", err)
	}

	newCACertPEM, _, err := newCAConfig.GetPEMBytes()
	if err != nil {
		return nil, fmt.Errorf("error extracting CA cert bytes: %w", err)
	}
	newServerCertPEM, newServerKeyPEM, err := newServerCertConfig.GetPEMBytes()
	if err != nil {
		return nil, fmt.Errorf("error extracting authentication proxy component bytes: %w", err)
	}

	g.GinkgoWriter.Println("Updating authentication proxy TLS Secret with rotated certificate")
	tlsSecret.Data[corev1.TLSCertKey] = newServerCertPEM
	tlsSecret.Data[corev1.TLSPrivateKeyKey] = newServerKeyPEM
	_, err = kubeClient.CoreV1().
		Secrets(aP.Namespace).
		Update(ctx, tlsSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error updating authentication proxy component TLS configuration: %w", err)
	}

	g.GinkgoWriter.Println("Waiting for authentication proxy TLS Secret to propagate to pod volume")
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		return aP.podFileContentMatches(ctx, *squidPod, "squid", "/etc/squid/tls/tls.crt", newServerCertPEM)
	})
	if err != nil {
		return nil, fmt.Errorf("%w: certificate propagation failed: %w", ErrProxyCertificateRotationIncomplete, err)
	}

	g.GinkgoWriter.Println("Reconfiguring authentication proxy to pick up the new certificate")
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: before authentication proxy reconfiguration: %w", ErrProxyCertificateRotationIncomplete, err)
	}
	output, err := adminClient.Run("exec").Args(
		"-n", aP.Namespace,
		squidPod.Name,
		"-c", "squid",
		"--", "/usr/sbin/squid", "-k", "reconfigure",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("%w: authentication proxy reconfiguration failed (%q): %w", ErrProxyCertificateRotationIncomplete, string(output), err)
	}

	aP.CACertPEM = newCACertPEM
	return newCACertPEM, nil
}

// certificateRotationResources reads and validates the existing TLS Secret and
// selects the first proxy pod. It performs no writes and does not validate PEM syntax.
func (aP AuthenticationProxyComponent) certificateRotationResources(ctx context.Context, coreClient coreclientv1.CoreV1Interface) (*corev1.Secret, *corev1.Pod, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	tlsSecret, err := coreClient.Secrets(aP.Namespace).Get(ctx, "squid-tls", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("error getting authentication proxy component TLS configuration: %w", err)
	}
	if len(tlsSecret.Data[corev1.TLSCertKey]) == 0 || len(tlsSecret.Data[corev1.TLSPrivateKeyKey]) == 0 {
		return nil, nil, fmt.Errorf("authentication proxy TLS Secret in namespace %q must contain non-empty %s and %s", aP.Namespace, corev1.TLSCertKey, corev1.TLSPrivateKeyKey)
	}

	squidPods, err := coreClient.Pods(aP.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", squidServiceName),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error listing authentication proxy component pods: %w", err)
	}
	if len(squidPods.Items) == 0 {
		return nil, nil, fmt.Errorf("no authentication proxy component pods found in namespace %q", aP.Namespace)
	}
	return tlsSecret, &squidPods.Items[0], nil
}

// podFileContentMatches reads a file through oc exec and checks that its trimmed
// contents include the trimmed expected PEM bytes, allowing additional certificates.
func (aP AuthenticationProxyComponent) podFileContentMatches(ctx context.Context, pod v1.Pod, container, caFilePath string, newCACertPEM []byte) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	adminClient := aP.oc.AsAdmin()

	output, execErr := adminClient.
		Run("exec").Args(
		"-n", pod.Namespace,
		pod.Name,
		"-c", container,
		"--", "cat", caFilePath,
	).Output()
	if execErr != nil {
		g.GinkgoWriter.Printf("failed to read CA file from pod %s/%s: %v\n", pod.Namespace, pod.Name, execErr)
		return false, execErr
	}
	if !strings.Contains(strings.TrimSpace(output), strings.TrimSpace(string(newCACertPEM))) {
		return false, nil
	}
	return true, nil
}
