package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
)

// ForwardPortAndExecute sets up oc port-forward to a service and executes
// the given test function with the local port. Retries up to 5 times with
// exponential backoff to handle pods restarting after config changes.
func ForwardPortAndExecute(serviceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	const maxAttempts = 5
	var err error
	backoff := 2 * time.Second
	for i := 0; i < maxAttempts; i++ {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			localPort := rand.Intn(65534-1025) + 1025
			args := []string{
				"port-forward",
				fmt.Sprintf("svc/%s", serviceName),
				fmt.Sprintf("%d:%s", localPort, remotePort),
				"-n", namespace,
			}

			cmd := exec.CommandContext(ctx, "oc", args...)
			stdout, stderr, err := e2e.StartCmdAndStreamOutput(cmd)
			if err != nil {
				return fmt.Errorf("failed to start port-forward: %v", err)
			}
			defer stdout.Close()
			defer stderr.Close()
			defer e2e.TryKill(cmd)

			ready := false
			for j := 0; j < 20; j++ {
				output := ReadPartialFrom(stdout, 1024)
				if strings.Contains(output, "Forwarding from") {
					e2e.Logf("oc port-forward ready: %s", output)
					ready = true
					break
				}

				testConn, testErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 200*time.Millisecond)
				if testErr == nil {
					testConn.Close()
					e2e.Logf("oc port-forward ready (port accepting connections)")
					ready = true
					break
				}

				time.Sleep(500 * time.Millisecond)
			}

			if !ready {
				stderrOutput := ReadPartialFrom(stderr, 1024)
				return fmt.Errorf("port-forward did not become ready within timeout (stderr: %s)", stderrOutput)
			}

			return toExecute(localPort)
		}(); err == nil {
			return nil
		}
		e2e.Logf("port-forward attempt %d/%d failed: %v", i+1, maxAttempts, err)
		if i < maxAttempts-1 {
			isPodNotReady := strings.Contains(err.Error(), "not running") ||
				strings.Contains(err.Error(), "Pending") ||
				strings.Contains(err.Error(), "CrashLoopBackOff")
			if isPodNotReady {
				e2e.Logf("pod backing svc/%s is not ready, waiting %v before retry", serviceName, backoff)
			}
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return err
}

// ReadPartialFrom reads up to maxBytes from a reader.
func ReadPartialFrom(r io.Reader, maxBytes int) string {
	buf := make([]byte, maxBytes)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Sprintf("error reading: %v", err)
	}
	return string(buf[:n])
}

// TLSVersionName returns a human-readable name for a TLS version constant.
func TLSVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// CheckTLSConnection verifies that a local-forwarded port accepts the expected
// TLS version and rejects the one that should not work.
func CheckTLSConnection(localPort int, shouldWork, shouldNotWork *tls.Config, serviceName, namespace string) error {
	hosts := []string{
		fmt.Sprintf("127.0.0.1:%d", localPort),
		fmt.Sprintf("[::1]:%d", localPort),
	}

	expectedMinVersion := TLSVersionName(shouldWork.MinVersion)
	rejectedMaxVersion := TLSVersionName(shouldNotWork.MaxVersion)

	var testedHosts []string

	for _, host := range hosts {
		hostType := "IPv4"
		if strings.HasPrefix(host, "[") {
			hostType = "IPv6"
		}

		e2e.Logf("[%s] %s: Testing connection with min %s (should SUCCEED)",
			hostType, host, expectedMinVersion)

		dialer := &net.Dialer{Timeout: 10 * time.Second}

		conn, err := tls.DialWithDialer(dialer, "tcp", host, shouldWork)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "network is unreachable") ||
				strings.Contains(errStr, "no route to host") ||
				strings.Contains(errStr, "connect: cannot assign requested address") {
				e2e.Logf("[%s] %s: Host not available, skipping: %v", hostType, host, err)
				continue
			}
			return fmt.Errorf("svc/%s in %s [%s]: Connection with %s FAILED (expected success): %w",
				serviceName, namespace, hostType, expectedMinVersion, err)
		}

		negotiated := conn.ConnectionState().Version
		conn.Close()
		e2e.Logf("[%s] %s: SUCCESS - Negotiated %s (requested min %s)",
			hostType, host, TLSVersionName(negotiated), expectedMinVersion)

		e2e.Logf("[%s] %s: Testing connection with max %s (should be REJECTED)",
			hostType, host, rejectedMaxVersion)

		conn, err = tls.DialWithDialer(dialer, "tcp", host, shouldNotWork)
		if err == nil {
			negotiatedBad := conn.ConnectionState().Version
			conn.Close()
			return fmt.Errorf("svc/%s in %s [%s]: Connection with max %s should be REJECTED but succeeded (negotiated %s)",
				serviceName, namespace, hostType, rejectedMaxVersion, TLSVersionName(negotiatedBad))
		}

		errStr := err.Error()
		if !strings.Contains(errStr, "protocol version") &&
			!strings.Contains(errStr, "no supported versions") &&
			!strings.Contains(errStr, "handshake failure") &&
			!strings.Contains(errStr, "alert") &&
			!strings.Contains(errStr, "EOF") &&
			!strings.Contains(errStr, "connection reset by peer") {
			return fmt.Errorf("svc/%s in %s [%s]: Expected TLS version rejection error, got: %w",
				serviceName, namespace, hostType, err)
		}
		e2e.Logf("[%s] %s: REJECTED - %s correctly refused by server",
			hostType, host, rejectedMaxVersion)

		testedHosts = append(testedHosts, fmt.Sprintf("%s(%s)", hostType, host))
	}

	if len(testedHosts) == 0 {
		return fmt.Errorf("svc/%s in %s: No hosts available for testing (tried IPv4 and IPv6)",
			serviceName, namespace)
	}

	e2e.Logf("svc/%s in %s: TLS PASS - Verified on %d host(s): %v | Accepts: %s+ | Rejects: %s",
		serviceName, namespace, len(testedHosts), testedHosts, expectedMinVersion, rejectedMaxVersion)
	return nil
}

// WaitForDeploymentCompleteWithTimeout waits for a deployment to complete rollout
// with a configurable timeout.
func WaitForDeploymentCompleteWithTimeout(ctx context.Context, c clientset.Interface, d *appsv1.Deployment, timeout time.Duration) error {
	e2e.Logf("Waiting for deployment %s/%s to complete (timeout: %v)", d.Namespace, d.Name, timeout)
	start := time.Now()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true,
		func(ctx context.Context) (bool, error) {
			deployment, err := c.AppsV1().Deployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll[%v]: error getting deployment: %v", time.Since(start).Round(time.Second), err)
				return false, nil
			}

			replicas := int32(1)
			if deployment.Spec.Replicas != nil {
				replicas = *deployment.Spec.Replicas
			}

			ready := deployment.Status.ReadyReplicas
			updated := deployment.Status.UpdatedReplicas
			available := deployment.Status.AvailableReplicas
			unavailable := deployment.Status.UnavailableReplicas

			if updated == replicas && ready == replicas && available == replicas && unavailable == 0 {
				e2e.Logf("  poll[%v]: deployment %s/%s is complete (ready=%d/%d)",
					time.Since(start).Round(time.Second), d.Namespace, d.Name, ready, replicas)
				return true, nil
			}

			elapsed := time.Since(start)
			if elapsed.Seconds() > 0 && int(elapsed.Seconds())%30 == 0 {
				e2e.Logf("  poll[%v]: deployment %s/%s not ready (replicas=%d, ready=%d, updated=%d, unavailable=%d)",
					elapsed.Round(time.Second), d.Namespace, d.Name, replicas, ready, updated, unavailable)
			}

			return false, nil
		})
}

// EnvToMap converts a slice of container environment variables to a map.
func EnvToMap(envVars []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(envVars))
	for _, e := range envVars {
		m[e.Name] = e.Value
	}
	return m
}

// FindEnvAcrossContainers searches all containers in a pod spec for the
// given env var key and returns the env map of the container that has it.
func FindEnvAcrossContainers(containers []corev1.Container, key string) map[string]string {
	for _, c := range containers {
		m := EnvToMap(c.Env)
		if _, ok := m[key]; ok {
			return m
		}
	}
	if len(containers) > 0 {
		return EnvToMap(containers[0].Env)
	}
	return map[string]string{}
}

// LogEnvVars logs TLS-related environment variables from the given map.
func LogEnvVars(envMap map[string]string, primaryKey string) {
	tlsPatterns := []string{"TLS", "CIPHER", "SSL"}
	e2e.Logf("TLS-related environment variables:")
	for key, val := range envMap {
		for _, pattern := range tlsPatterns {
			if strings.Contains(strings.ToUpper(key), pattern) {
				display := val
				if len(display) > 120 {
					display = display[:120] + "..."
				}
				e2e.Logf("  %s=%s", key, display)
				break
			}
		}
	}
	if _, ok := envMap[primaryKey]; !ok {
		e2e.Logf("  WARNING: primary TLS env var %s not found", primaryKey)
	}
}

// WaitForClusterOperatorStable waits until the named ClusterOperator reaches
// Available=True, Progressing=False, Degraded=False.
func WaitForClusterOperatorStable(oc *CLI, ctx context.Context, name string) {
	e2e.Logf("Waiting for ClusterOperator %q to become stable", name)
	start := time.Now()

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 25*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll[%s]: error fetching ClusterOperator %s: %v",
					time.Since(start).Round(time.Second), name, err)
				return false, nil
			}

			isAvailable := false
			isProgressing := true
			isDegraded := false

			for _, c := range co.Status.Conditions {
				switch c.Type {
				case configv1.OperatorAvailable:
					isAvailable = c.Status == configv1.ConditionTrue
				case configv1.OperatorProgressing:
					isProgressing = c.Status == configv1.ConditionTrue
				case configv1.OperatorDegraded:
					isDegraded = c.Status == configv1.ConditionTrue
				}
			}

			if isDegraded {
				e2e.Logf("  poll[%s]: WARNING ClusterOperator %s is degraded", time.Since(start).Round(time.Second), name)
				for _, c := range co.Status.Conditions {
					e2e.Logf("    %s=%s reason=%s message=%q", c.Type, c.Status, c.Reason, c.Message)
				}
				return false, nil
			}

			if isAvailable && !isProgressing {
				e2e.Logf("  poll[%s]: ClusterOperator %s is stable", time.Since(start).Round(time.Second), name)
				return true, nil
			}

			e2e.Logf("  poll[%s]: ClusterOperator %s not stable (Available=%v, Progressing=%v)",
				time.Since(start).Round(time.Second), name, isAvailable, isProgressing)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("ClusterOperator %s did not reach stable state after %s",
			name, time.Since(start).Round(time.Second)))
}
