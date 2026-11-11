package component_proxy

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	ComponentProxyCAConfigMapName = "v4-0-config-system-auth-proxy-ca"

	defaultPollingInterval = 10 * time.Second
	defaultPollingTimeout  = 5 * time.Minute
)

type ProxyVerifier struct {
	Namespace, Name string
	CLI             *exutil.CLI

	// PollingInterval and PollingTimeout fall back to the package defaults when unset.
	PollingInterval, PollingTimeout time.Duration
}

// AssertProxyConfig polls the target deployment until its HTTP_PROXY, HTTPS_PROXY, and
// NO_PROXY env vars and trustedCA volume/mount state match the expected values. NO_PROXY
// is compared with superset semantics because the operator appends entries (e.g. the
// apiserver IP) beyond what is configured. Presence of trustedCA requires both the
// volume and a mount; absence requires neither, so a volume left without a mount
// satisfies neither check and fails AssertNoConfig.
func (pv ProxyVerifier) AssertProxyConfig(ctx context.Context, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectPresenceForTrustedCAVolume bool) error {
	if pv.PollingInterval <= 0 {
		pv.PollingInterval = defaultPollingInterval
	}
	if pv.PollingTimeout <= 0 {
		pv.PollingTimeout = defaultPollingTimeout
	}
	kubeClient := pv.CLI.AdminKubeClient()

	return wait.PollUntilContextTimeout(ctx, pv.PollingInterval, pv.PollingTimeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := kubeClient.AppsV1().
			Deployments(pv.Namespace).
			Get(ctx, pv.Name, metav1.GetOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to get %s/%s deployment: %v\n", pv.Namespace, pv.Name, err)
			return false, nil
		}

		envVars := make(map[string]string)
		for _, container := range deployment.Spec.Template.Spec.Containers {
			for _, env := range container.Env {
				switch env.Name {
				case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
					envVars[env.Name] = env.Value
				}
			}
		}

		if envVars["HTTP_PROXY"] != expectedHTTPProxy || envVars["HTTPS_PROXY"] != expectedHTTPSProxy {
			g.GinkgoWriter.Printf("proxy env mismatch for %s/%s deployment: HTTP_PROXY=%q (want %q), HTTPS_PROXY=%q (want %q)\n",
				pv.Namespace, pv.Name,
				envVars["HTTP_PROXY"], expectedHTTPProxy, envVars["HTTPS_PROXY"], expectedHTTPSProxy)
			return false, nil
		}

		if expectedNoProxy == "" {
			if envVars["NO_PROXY"] != "" {
				g.GinkgoWriter.Printf("proxy env mismatch for %s/%s deployment: NO_PROXY=%q (want empty)\n",
					pv.Namespace, pv.Name,
					envVars["NO_PROXY"])
				return false, nil
			}
		} else {
			// Use IsSuperset rather than exact match because the operator appends
			// the apiserver IP to NO_PROXY beyond the entries we configure.
			actualNoProxy := sets.New(strings.Split(envVars["NO_PROXY"], ",")...)
			expectedNoProxyEntries := sets.New(strings.Split(expectedNoProxy, ",")...)
			if !actualNoProxy.IsSuperset(expectedNoProxyEntries) {
				g.GinkgoWriter.Printf("proxy env mismatch for %s/%s deployment: NO_PROXY=%q does not contain all of %q\n",
					pv.Namespace, pv.Name,
					envVars["NO_PROXY"], expectedNoProxy)
				return false, nil
			}
		}

		foundVolume, foundMount := trustedCAVolumeState(deployment)
		if !trustedCAStateMatches(expectPresenceForTrustedCAVolume, foundVolume, foundMount) {
			g.GinkgoWriter.Printf("trustedCA for %s/%s deployment: volume=%v mount=%v (want present=%v)\n",
				pv.Namespace, pv.Name,
				foundVolume, foundMount, expectPresenceForTrustedCAVolume)
			return false, nil
		}
		return true, nil
	})
}

// AssertNoConfig asserts the target deployment has no proxy env vars and no trustedCA
// volume/mount configured.
func (pv ProxyVerifier) AssertNoConfig(ctx context.Context) error {
	return pv.AssertProxyConfig(ctx, "", "", "", false)
}

// trustedCAStateMatches reports whether the observed volume/mount state matches the
// expectation. Presence requires both the volume and a mount; absence requires neither,
// so a volume left without a mount satisfies neither and fails an AssertNoConfig check.
func trustedCAStateMatches(expectPresence, foundVolume, foundMount bool) bool {
	if expectPresence {
		return foundVolume && foundMount
	}
	return !(foundVolume || foundMount)
}

func trustedCAVolumeState(deployment *appsv1.Deployment) (foundVolume, foundMount bool) {
	var volumeName string
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == ComponentProxyCAConfigMapName {
			foundVolume = true
			volumeName = vol.Name
			break
		}
	}
	if !foundVolume {
		return false, false
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName {
				return true, true
			}
		}
	}

	return true, false
}
