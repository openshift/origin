package util

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CertInfo holds the Subject and Issuer DN strings from a TLS leaf certificate.
type CertInfo struct {
	Subject string
	Issuer  string
}

// ForwardPortAndExecute forwards a port to a service or pod and executes a function with the local port.
func ForwardPortAndExecute(resourceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	resource := resourceName
	port := remotePort

	if !strings.Contains(resourceName, "/") {
		resource = fmt.Sprintf("svc/%s", resourceName)
		if ClusterDegraded {
			resolveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			podResource, podPort, resolveErr := portForwardTargetOnReadyNode(resolveCtx, resourceName, namespace, remotePort)
			if resolveErr != nil {
				e2e.Logf("degraded cluster: unable to select Ready-node pod for %s/%s, falling back to service: %v", namespace, resourceName, resolveErr)
			} else {
				resource, port = podResource, podPort
				e2e.Logf("degraded cluster: port-forwarding to %s port %s instead of svc/%s", resource, port, resourceName)
			}
		}
	}
	var err error
	for i := 0; i < 3; i++ {
		if err = func() error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			localPort := rand.Intn(65534-1025) + 1025
			args := []string{"port-forward", fmt.Sprintf("svc/%s", serviceName), fmt.Sprintf("%d:%s", localPort, remotePort), "-n", namespace}

			cmd := exec.CommandContext(ctx, "oc", args...)
			stdout, stderr, err := e2e.StartCmdAndStreamOutput(cmd)
			if err != nil {
				return err
			}
			defer stdout.Close()
			defer stderr.Close()
			defer e2e.TryKill(cmd)

			_ = ReadPartialFrom(stdout, 1024)

			if err := waitForLocalPort(ctx, localPort, 5*time.Second); err != nil {
				return fmt.Errorf("port-forward failed to become ready: %w", err)
			}

			return toExecute(localPort)
		}(); err == nil {
			return nil
		} else {
			e2e.Logf("failed to start oc port-forward command or test: %v", err)
			time.Sleep(2 * time.Second)
		}
	}
	return err
}

// waitForLocalPort waits until the forwarded local port accepts TCP connections.
func waitForLocalPort(ctx context.Context, localPort int, timeout time.Duration) error {
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, timeout, true,
		func(ctx context.Context) (bool, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
			if err != nil {
				return false, nil
			}
			conn.Close()
			return true, nil
		})
}

// portForwardTargetOnReadyNode returns pod/<name> and the service's target port for a
// Ready Running pod backing serviceName that is scheduled on a Ready node.
func portForwardTargetOnReadyNode(ctx context.Context, serviceName, namespace, remotePort string) (string, string, error) {
	kubeClient, err := e2e.LoadClientset(true)
	if err != nil {
		return "", "", err
	}

	svc, err := kubeClient.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	if len(svc.Spec.Selector) == 0 {
		return "", "", fmt.Errorf("service %s/%s has no selector", namespace, serviceName)
	}

	targetPort, err := servicePortToTargetPort(svc, remotePort)
	if err != nil {
		return "", "", err
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	readyNodes := map[string]struct{}{}
	for i := range nodes.Items {
		if isNodeReadyConditionTrue(&nodes.Items[i]) {
			readyNodes[nodes.Items[i].Name] = struct{}{}
		}
	}
	if len(readyNodes) == 0 {
		return "", "", fmt.Errorf("no Ready nodes found")
	}

	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
	})
	if err != nil {
		return "", "", err
	}
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || !isPodReadyConditionTrue(&pod) {
			continue
		}
		if _, ok := readyNodes[pod.Spec.NodeName]; !ok {
			continue
		}
		return "pod/" + pod.Name, targetPort, nil
	}
	return "", "", fmt.Errorf("no Ready Running pods for service %s/%s on Ready nodes", namespace, serviceName)
}

func servicePortToTargetPort(svc *corev1.Service, remotePort string) (string, error) {
	for _, p := range svc.Spec.Ports {
		if strconv.Itoa(int(p.Port)) != remotePort && p.Name != remotePort {
			continue
		}
		switch p.TargetPort.Type {
		case intstr.String:
			if p.TargetPort.StrVal != "" {
				return p.TargetPort.StrVal, nil
			}
		default:
			if p.TargetPort.IntVal != 0 {
				return strconv.Itoa(int(p.TargetPort.IntVal)), nil
			}
		}
		return strconv.Itoa(int(p.Port)), nil
	}
	return "", fmt.Errorf("port %q not found on service %s/%s", remotePort, svc.Namespace, svc.Name)
}

func isNodeReadyConditionTrue(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isPodReadyConditionTrue(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
// ReadPartialFrom reads up to maxBytes from a reader and returns the content as a string.
func ReadPartialFrom(r io.Reader, maxBytes int) string {
	buf := make([]byte, maxBytes)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Sprintf("error reading: %v", err)
	}
	return string(buf[:n])
}

// CheckTLSConnection verifies that a TLS connection works with the expected config and fails with the other.
func CheckTLSConnection(port int, tlsShouldWork, tlsShouldNotWork *tls.Config) error {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: 10 * time.Second,
		},
		Config: tlsShouldWork,
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("should work: %w", err)
	}
	err = conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	if tlsShouldNotWork == nil {
		return nil
	}

	negDialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: 10 * time.Second,
		},
		Config: tlsShouldNotWork,
	}

	conn, err = negDialer.DialContext(context.Background(), "tcp", fmt.Sprintf("localhost:%d", port))
	if err == nil {
		return fmt.Errorf("should not work: connection unexpectedly succeeded, closing conn status: %v", conn.Close())
	}
	if !strings.Contains(err.Error(), "protocol version") &&
		!strings.Contains(err.Error(), "no supported versions satisfy") &&
		!strings.Contains(err.Error(), "handshake failure") {
		return fmt.Errorf("should not work: got error, but not a TLS version mismatch: %w", err)
	}
	return nil
}

// GetServerCertInfo opens a TLS connection to fqdn:port using caPath as the trusted root CA
// and returns the leaf certificate's Subject and Issuer DNs.
func GetServerCertInfo(ctx context.Context, fqdn, port, caPath string) (*CertInfo, error) {
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file %s: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse any valid certificates from CA file %s", caPath)
	}

	dialer := &tls.Dialer{
		Config: &tls.Config{RootCAs: pool, ServerName: fqdn},
	}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", fqdn, port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("connection is not a TLS connection")
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no peer certificates returned by %s:%s", fqdn, port)
	}
	return &CertInfo{
		Subject: certs[0].Subject.String(),
		Issuer:  certs[0].Issuer.String(),
	}, nil
}

// GetAPIServerFQDNAndPort returns the external API server hostname and port from the
// cluster's infrastructure config.
func GetAPIServerFQDNAndPort(ctx context.Context, oc *CLI) (string, string) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
		ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		e2e.Failf("failed to get infrastructure: %v", err)
	}

	rawURL := infra.Status.APIServerURL
	u, err := url.Parse(rawURL)
	if err != nil {
		e2e.Failf("failed to parse API server URL %q: %v", rawURL, err)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return host, port
}

// WaitCoBecomes polls the named ClusterOperator until all entries in conditions match
// (e.g. {"Available": "True", "Progressing": "False"}) or the timeout elapses.
func WaitCoBecomes(ctx context.Context, oc *CLI, coName string, timeoutSec int, conditions map[string]string) error {
	lastLogTime := time.Time{}
	logInterval := 30 * time.Second
	return wait.PollUntilContextTimeout(
		ctx, 5*time.Second, time.Duration(timeoutSec)*time.Second, false,
		func(pollCtx context.Context) (bool, error) {
			co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(
				pollCtx, coName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("failed to get ClusterOperator %s: %v — retrying", coName, err)
				return false, nil
			}

			// Build a map of observed conditions
			observed := make(map[string]string)
			for _, cond := range co.Status.Conditions {
				observed[string(cond.Type)] = string(cond.Status)
			}

			// Verify all requested conditions are present and match
			for condType, want := range conditions {
				got, found := observed[condType]
				if !found || got != want {
					// Only log every 30 seconds to reduce verbosity
					now := time.Now()
					if now.Sub(lastLogTime) >= logInterval {
						if !found {
							e2e.Logf("ClusterOperator %s: condition %s not found (want %s)", coName, condType, want)
						} else {
							e2e.Logf("ClusterOperator %s: condition %s=%s (want %s)", coName, condType, got, want)
						}
						lastLogTime = now
					}
					return false, nil
				}
			}
			return true, nil
		})
}

// GetEncryptionPrefix returns the first 30 bytes (as a string) of the etcd value stored at
// etcdPath. For an encrypted cluster this prefix will contain the encryption scheme identifier,
// e.g. "k8s:enc:aescbc:v1:1:".
func GetEncryptionPrefix(ctx context.Context, oc *CLI, etcdPath string) (string, error) {
	pods, err := oc.AsAdmin().KubeClient().CoreV1().Pods("openshift-etcd").List(
		ctx, metav1.ListOptions{LabelSelector: "app=etcd"})
	if err != nil {
		return "", fmt.Errorf("failed to list etcd pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no etcd pods found in openshift-etcd")
	}
	podName := pods.Items[0].Name

	out, err := oc.AsAdmin().Run("exec").Args(
		"-n", "openshift-etcd", podName, "-c", "etcdctl", "--",
		"etcdctl", "get", etcdPath, "--prefix", "--limit=1", "--print-value-only",
	).Output()
	if err != nil {
		return "", fmt.Errorf("etcdctl get %s failed: %w", etcdPath, err)
	}
	// Trim to the ASCII encryption prefix (at most 30 chars) so the output is safe to compare.
	if len(out) > 30 {
		out = out[:30]
	}
	return out, nil
}

// GetEncryptionKeyNumber lists secrets in openshift-config-managed whose names match
// pattern and returns the highest numeric suffix found.  For example, if secrets
// "encryption-key-openshift-apiserver-3" and "-4" exist, it returns 4.
func GetEncryptionKeyNumber(oc *CLI, pattern string) (int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return GetEncryptionKeyNumberWithRegex(oc, re)
}

// GetEncryptionKeyNumberWithRegex is like GetEncryptionKeyNumber but accepts a pre-compiled
// regex for efficiency when called repeatedly in polling loops.
func GetEncryptionKeyNumberWithRegex(oc *CLI, re *regexp.Regexp) (int, error) {
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"secrets", "-n", "openshift-config-managed",
		"-o=jsonpath={.items[*].metadata.name}",
	).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list secrets in openshift-config-managed: %w", err)
	}

	maxNum := 0
	found := false
	for _, name := range strings.Fields(out) {
		if !re.MatchString(name) {
			continue
		}
		parts := strings.Split(name, "-")
		if len(parts) == 0 {
			continue
		}
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			found = true
			if n > maxNum {
				maxNum = n
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("no encryption key secrets matching pattern %q found", re.String())
	}
	return maxNum, nil
}

// WaitEncryptionKeyMigration polls the named secret in openshift-config-managed until the
// "encryption.apiserver.operator.openshift.io/migrated-resources" annotation is non-empty,
// indicating the encryption key migration has completed (up to 30 minutes).
func WaitEncryptionKeyMigration(ctx context.Context, oc *CLI, secretName string) (bool, error) {
	const migrationAnnotation = "encryption.apiserver.operator.openshift.io/migrated-resources"
	err := wait.PollUntilContextTimeout(
		ctx, 30*time.Second, 30*time.Minute, false,
		func(pollCtx context.Context) (bool, error) {
			secret, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("openshift-config-managed").Get(
				pollCtx, secretName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("failed to get secret %s: %v — retrying", secretName, err)
				return false, nil
			}
			if v := secret.Annotations[migrationAnnotation]; v != "" {
				e2e.Logf("encryption migration complete for %s: %s", secretName, v)
				return true, nil
			}
			e2e.Logf("waiting for migration to complete for secret %s", secretName)
			return false, nil
		})
	if err != nil {
		return false, err
	}
	return true, nil
}

// EnsureEncryptionEnabled checks if etcd encryption is enabled, and if not, enables it with aescbc.
// Returns the encryption type and a cleanup function that should be deferred.
// The cleanup function will restore encryption to identity (disabled) if it was originally disabled.
func EnsureEncryptionEnabled(ctx context.Context, oc *CLI) (encryptionType string, cleanup func(), err error) {
	currentType, err := oc.WithoutNamespace().Run("get").Args(
		"apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get encryption type: %w", err)
	}

	wasEnabled := (currentType == "aescbc" || currentType == "aesgcm")
	if wasEnabled {
		e2e.Logf("etcd encryption already enabled with type: %s", currentType)
		return currentType, func() {}, nil
	}

	// Encryption not enabled - enable it
	e2e.Logf("etcd encryption is not enabled (current type: %s), enabling aescbc encryption", currentType)

	err = oc.WithoutNamespace().Run("patch").Args(
		"apiserver", "cluster", "--type=merge",
		"-p", `{"spec":{"encryption":{"type":"aescbc"}}}`).Execute()
	if err != nil {
		return "", nil, fmt.Errorf("failed to enable encryption: %w", err)
	}

	// Wait for kube-apiserver operator to start progressing
	err = WaitCoBecomes(ctx, oc, "kube-apiserver", 300, map[string]string{"Progressing": "True"})
	if err != nil {
		return "", nil, fmt.Errorf("kube-apiserver operator did not start progressing within 300s: %w", err)
	}

	// Wait for kube-apiserver to stabilize
	kubeAPIServerTimeout := 1800
	e2e.Logf("waiting for kube-apiserver operator to stabilize (timeout: %v)", time.Duration(kubeAPIServerTimeout)*time.Second)
	err = WaitCoBecomes(ctx, oc, "kube-apiserver", kubeAPIServerTimeout, map[string]string{
		"Available":   "True",
		"Progressing": "False",
		"Degraded":    "False",
	})
	if err != nil {
		return "", nil, fmt.Errorf("kube-apiserver operator did not stabilize within 1800s: %w", err)
	}

	// Wait for openshift-apiserver to stabilize
	openshiftAPIServerTimeout := 1800
	e2e.Logf("waiting for openshift-apiserver operator to stabilize (timeout: %v)", time.Duration(openshiftAPIServerTimeout)*time.Second)
	err = WaitCoBecomes(ctx, oc, "openshift-apiserver", openshiftAPIServerTimeout, map[string]string{
		"Available":   "True",
		"Progressing": "False",
		"Degraded":    "False",
	})
	if err != nil {
		return "", nil, fmt.Errorf("openshift-apiserver operator did not stabilize within 1800s: %w", err)
	}

	e2e.Logf("etcd encryption successfully enabled with type: aescbc")

	// Return cleanup function that restores to identity
	cleanupFunc := func() {
		e2e.Logf("restoring encryption to identity (disabled)")
		// Use detached context to ensure cleanup runs even if spec context is cancelled
		_, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		patchErr := oc.WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster", "--type=merge",
			"-p", `{"spec":{"encryption":{"type":"identity"}}}`).Execute()
		if patchErr != nil {
			e2e.Logf("error restoring encryption to identity: %v", patchErr)
		}
	}

	return "aescbc", cleanupFunc, nil
}

// BuildEncryptionKeySecretName constructs the encryption key secret name from prefix and number.
func BuildEncryptionKeySecretName(prefix string, keyNumber int) string {
	return prefix + strconv.Itoa(keyNumber)
}
