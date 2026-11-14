package authproxy

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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/openshift/origin/test/extended/util/image"
)

var (
	// ErrEmptyCLI reports a nil CLI.
	ErrEmptyCLI = errors.New("empty CLI")
	// ErrProxyCertificateRotationIncomplete reports that the TLS Secret was already
	// updated, so callers can tell a half-applied rotation from a no-op.
	ErrProxyCertificateRotationIncomplete = errors.New("proxy certificate rotation incomplete: TLS Secret was updated")
)

// AuthenticationProxyComponent is the result of DeployAuthProxyComponent. A partial
// component from a failed deployment can still be deleted, since Delete is a no-op
// without a namespace. Copies share URL pointers and CA bytes, so rotation updates
// only the receiver's CACertPEM.
type AuthenticationProxyComponent struct {
	oc        *exutil.CLI
	namespace string

	HttpUrl, HttpsUrl *url.URL
	CACertPEM         []byte
}

// DeployAuthProxyComponent requires an initialized oc. On failure after namespace
// creation the returned component still owns the namespace, so the caller must
// Delete it with an active context.
func DeployAuthProxyComponent(ctx context.Context, oc *exutil.CLI) (authProxy AuthenticationProxyComponent, err error) {
	if oc == nil {
		return authProxy, ErrEmptyCLI
	}
	if err := ctx.Err(); err != nil {
		return authProxy, err
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
	authProxy.namespace = namespace.Name

	certs, err := generateCertificates(namespace.Name)
	if err != nil {
		return authProxy, fmt.Errorf("error on generating proxy certificates: %w", err)
	}

	authProxy.CACertPEM = certs.caCertPEM

	// Field order is fixed: proxyTrafficMatches parses method url sourceIP squidStatus httpCode.
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

	_, err = kubeClient.CoreV1().
		Secrets(authProxy.namespace).
		Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "squid-config"},
			StringData: map[string]string{"squid.conf": squidConfig},
		}, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy config: %w", err)
	}

	_, err = kubeClient.CoreV1().
		Secrets(authProxy.namespace).
		Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "squid-tls"},
			Data: map[string][]byte{
				"tls.crt": certs.serverCertPEM,
				"tls.key": certs.serverKeyPEM,
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

	_, err = kubeClient.AppsV1().
		Deployments(authProxy.namespace).
		Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return authProxy, fmt.Errorf("error creating authentication proxy deployment: %w", err)
	}

	_, err = kubeClient.CoreV1().
		Services(authProxy.namespace).
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
			kubeClient.AppsV1().RESTClient(), "deployments", authProxy.namespace,
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
		return authProxy, fmt.Errorf("authentication proxy deployment in namespace %q did not become ready: %w", authProxy.namespace, err)
	}

	authProxy.HttpUrl, err = url.Parse("http://" + net.JoinHostPort(certs.serviceHost, strconv.Itoa(int(squidHTTPPort))))
	if err != nil {
		return authProxy, fmt.Errorf("error parsing http proxy url: %w", err)
	}

	authProxy.HttpsUrl, err = url.Parse("https://" + net.JoinHostPort(certs.serviceHost, strconv.Itoa(int(squidHTTPSPort))))
	if err != nil {
		return authProxy, fmt.Errorf("error parsing https proxy url: %w", err)
	}
	g.GinkgoWriter.Printf("Authentication proxy deployed: %q\n", authProxy)
	return authProxy, nil
}

// String omits credentials and CA contents, so the value is safe to log.
func (aP AuthenticationProxyComponent) String() string {
	return fmt.Sprintf("AuthenticationProxy[Namespace=%q HttpProxy=%q HttpsProxy=%q CACertLen=%d]",
		aP.namespace,
		stripCredentials(aP.HttpUrl),
		stripCredentials(aP.HttpsUrl),
		len(aP.CACertPEM),
	)
}

// Delete treats an absent namespace and a component without a namespace as success,
// so callers can register it unconditionally. The timeout bounds both the request and
// the wait. Failures report unconfirmed deletion and preserve the context and last
// read error. Fields are retained so Delete can be retried after a failed wait.
func (aP AuthenticationProxyComponent) Delete(ctx context.Context, timeout time.Duration) error {
	if aP.namespace == "" {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	deleteCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	kubeClient := aP.oc.AdminKubeClient()
	return deleteProxyNamespace(deleteCtx, kubeClient.CoreV1().Namespaces(), aP.namespace, 5*time.Second)
}

// deleteProxyNamespace uses foreground propagation so dependent resources are gone
// before the namespace disappears; the caller must bound the wait with a deadline.
func deleteProxyNamespace(ctx context.Context, namespaces coreclientv1.NamespaceInterface, namespace string, interval time.Duration) error {
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

// logReader returns a reader that reports all available logs when since is zero.
// The reader does not retry failures, so callers can accumulate them across polls.
func (aP AuthenticationProxyComponent) logReader(
	since time.Time,
) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kubeClient := aP.oc.AdminKubeClient()

		pods, err := kubeClient.CoreV1().
			Pods(aP.namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", squidServiceName),
			})
		if err != nil {
			return "", fmt.Errorf("error listing auth proxy pods in namespace %q: %w", aP.namespace, err)
		}
		if len(pods.Items) == 0 {
			return "", fmt.Errorf("no auth proxy pods found in namespace %q", aP.namespace)
		}

		logOpts := &corev1.PodLogOptions{Container: "squid"}
		if !since.IsZero() {
			t := metav1.NewTime(since)
			logOpts.SinceTime = &t
		}
		logBytes, err := kubeClient.CoreV1().
			Pods(aP.namespace).
			GetLogs(pods.Items[0].Name, logOpts).DoRaw(ctx)
		if err != nil {
			return "", fmt.Errorf("error getting logs from auth proxy container in namespace %q: %w", aP.namespace, err)
		}

		return string(logBytes), nil
	}
}

// WaitForProxyTrafficFrom matches sourceIP literally, so callers must use the logged
// address format. Read failures are retried but preserved if the wait fails, so a
// flake cannot mask the real timeout.
func (aP AuthenticationProxyComponent) WaitForProxyTrafficFrom(
	ctx context.Context,
	sourceIP string,
	since time.Time,
	timeout time.Duration,
) error {
	g.GinkgoHelper()
	sourceIPs := []string{sourceIP}
	if err := ctx.Err(); err != nil {
		return err
	}
	g.GinkgoWriter.Printf("waiting up to %s for proxy traffic from %s\n", timeout, sourceIP)
	return waitForProxyTraffic(ctx, timeout, 10*time.Second,
		aP.logReader(since),
		func(logs string) bool { return proxyTrafficMatches(logs, sourceIPs, "") })
}

// WaitForProxyTrafficFromTo matches IPs literally, so callers must use the logged
// format. An empty destHost accepts any destination, and hostnames compare in full,
// ignoring DNS case and a trailing dot. Read failures are retried but preserved.
func (aP AuthenticationProxyComponent) WaitForProxyTrafficFromTo(
	ctx context.Context,
	sourceIPs []string,
	destHost string,
	since time.Time,
	timeout time.Duration,
) error {
	g.GinkgoHelper()
	if err := ctx.Err(); err != nil {
		return err
	}

	g.GinkgoWriter.Printf("waiting up to %s for proxy traffic from %v to %s\n", timeout, sourceIPs, destHost)
	return waitForProxyTraffic(ctx, timeout, 10*time.Second,
		aP.logReader(since),
		func(logs string) bool { return proxyTrafficMatches(logs, sourceIPs, destHost) })
}

// proxyTrafficMatches compares source IPs literally; hosts ignore DNS case and a trailing dot. An empty
// destHost accepts any destination. Malformed entries are skipped.
func proxyTrafficMatches(logs string, sourceIPs []string, destHost string) bool {
	allowedIPs := sets.New(sourceIPs...)

	for line := range strings.SplitSeq(logs, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "CONNECT" || parts[3] != "TCP_TUNNEL" {
			continue
		}
		sourceIP := parts[2]
		if !allowedIPs.Has(sourceIP) {
			continue
		}
		host, _, err := net.SplitHostPort(parts[1])
		if err != nil {
			continue
		}
		if destHost != "" && !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(destHost, ".")) {
			continue
		}
		g.GinkgoWriter.Printf("confirmed proxy traffic: %s\n", line)
		return true
	}
	return false
}

// waitForProxyTraffic joins the last read failure with the termination error, so a log flake stays
// visible in the timeout message.
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

// certificates carries serviceHost because the proxy URLs must use the server
// certificate's only SAN for TLS verification against caCertPEM to succeed.
type certificates struct {
	serviceHost                 string
	caCertPEM                   []byte
	serverCertPEM, serverKeyPEM []byte
}

// generateCertificates returns a fresh CA on every call, so rotating discards trust
// in the previous one. The CA private key is dropped after signing.
func generateCertificates(namespace string) (certificates, error) {
	caConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	if err != nil {
		return certificates{}, fmt.Errorf("creating CA certificate: %w", err)
	}

	caCertPEM, _, err := caConfig.GetPEMBytes()
	if err != nil {
		return certificates{}, fmt.Errorf("encoding CA certificate: %w", err)
	}

	ca := &libcrypto.CA{
		Config:          caConfig,
		SerialGenerator: &libcrypto.RandomSerialGenerator{},
	}
	serviceHost := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, namespace)
	serverCertConfig, err := ca.MakeServerCert(sets.New(serviceHost), 2*time.Hour)
	if err != nil {
		return certificates{}, fmt.Errorf("creating server certificate: %w", err)
	}

	serverCertPEM, serverKeyPEM, err := serverCertConfig.GetPEMBytes()
	if err != nil {
		return certificates{}, fmt.Errorf("encoding server certificate: %w", err)
	}

	return certificates{
		serviceHost:   serviceHost,
		caCertPEM:     caCertPEM,
		serverCertPEM: serverCertPEM,
		serverKeyPEM:  serverKeyPEM,
	}, nil
}

// RotateProxyCertificates leaves CACertPEM unchanged until the proxy reconfigures,
// and the caller must distribute the returned CA to consumers. On
// ErrProxyCertificateRotationIncomplete the Secret is already updated and nothing is
// rolled back, so callers must treat the proxy as half-rotated.
func (aP *AuthenticationProxyComponent) RotateProxyCertificates(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	kubeClient := aP.oc.AdminKubeClient()
	adminClient := aP.oc.AsAdmin()

	tlsSecret, squidPod, err := aP.certificateRotationResources(ctx, kubeClient.CoreV1())
	if err != nil {
		return nil, err
	}

	certs, err := generateCertificates(aP.namespace)
	if err != nil {
		return nil, fmt.Errorf("error generating authentication proxy component certificates: %w", err)
	}

	g.GinkgoWriter.Println("Updating authentication proxy TLS Secret with rotated certificate")
	tlsSecret.Data[corev1.TLSCertKey] = certs.serverCertPEM
	tlsSecret.Data[corev1.TLSPrivateKeyKey] = certs.serverKeyPEM
	_, err = kubeClient.CoreV1().
		Secrets(aP.namespace).
		Update(ctx, tlsSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error updating authentication proxy component TLS configuration: %w", err)
	}

	g.GinkgoWriter.Println("Waiting for authentication proxy TLS Secret to propagate to pod volume")
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		return aP.podFileContentMatches(ctx, *squidPod, "squid", "/etc/squid/tls/tls.crt", certs.serverCertPEM)
	})
	if err != nil {
		return nil, fmt.Errorf("%w: certificate propagation failed: %w", ErrProxyCertificateRotationIncomplete, err)
	}

	g.GinkgoWriter.Println("Reconfiguring authentication proxy to pick up the new certificate")
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: before authentication proxy reconfiguration: %w", ErrProxyCertificateRotationIncomplete, err)
	}
	output, err := adminClient.Run("exec").Args(
		"-n", aP.namespace,
		squidPod.Name,
		"-c", "squid",
		"--", "/usr/sbin/squid", "-k", "reconfigure",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("%w: authentication proxy reconfiguration failed (%q): %w", ErrProxyCertificateRotationIncomplete, string(output), err)
	}

	aP.CACertPEM = certs.caCertPEM
	return aP.CACertPEM, nil
}

// certificateRotationResources performs no writes and does not validate PEM syntax,
// so it cannot repair an unusable Secret.
func (aP AuthenticationProxyComponent) certificateRotationResources(ctx context.Context, coreClient coreclientv1.CoreV1Interface) (*corev1.Secret, *corev1.Pod, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	tlsSecret, err := coreClient.
		Secrets(aP.namespace).
		Get(ctx, "squid-tls", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("error getting authentication proxy component TLS configuration: %w", err)
	}
	if len(tlsSecret.Data[corev1.TLSCertKey]) == 0 || len(tlsSecret.Data[corev1.TLSPrivateKeyKey]) == 0 {
		return nil, nil, fmt.Errorf("authentication proxy TLS Secret in namespace %q must contain non-empty %s and %s", aP.namespace, corev1.TLSCertKey, corev1.TLSPrivateKeyKey)
	}

	squidPods, err := coreClient.Pods(aP.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", squidServiceName),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error listing authentication proxy component pods: %w", err)
	}
	if len(squidPods.Items) == 0 {
		return nil, nil, fmt.Errorf("no authentication proxy component pods found in namespace %q", aP.namespace)
	}
	return tlsSecret, &squidPods.Items[0], nil
}

// podFileContentMatches verifies if the container's pod CA certificate
// file matches the expected PEM
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
