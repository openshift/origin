package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
)

// Helpers for components that consume the APIServer TLS security profile when
// TLSGroupPreferences is enabled. Profile updates, deployment env checks, and
// openssl s_client probes are shared. Each component supplies its own
// namespace, deployment, Service, and env var names.

const (
	TLS12Flag           = "-tls1_2"
	TLS13Flag           = "-tls1_3"
	TLS11Flag           = "-tls1_1"
	InvalidTLSGroupName = "not-a-tls-group"
)

// TLS12HTTP2Ciphers is the smallest TLS 1.2 cipher list APIServer admission accepts.
// A custom profile whose minimum version is below TLS 1.3 must include an HTTP/2 cipher.
var TLS12HTTP2Ciphers = []string{
	"ECDHE-ECDSA-AES128-GCM-SHA256",
	"ECDHE-RSA-AES128-GCM-SHA256",
}

// ErrDeploymentRolling is returned while a deployment has not finished rolling out
// or has no ready pods.
var ErrDeploymentRolling = errors.New("deployment rollout is not complete")

// ErrOpenSSLFlagUnsupported is returned when the local openssl cannot honor a TLS flag.
var ErrOpenSSLFlagUnsupported = errors.New("openssl does not support the requested TLS flag")

// APIServerTLSProfile is a JSON patch body. Empty fields are omitted so a case can
// send groups without also sending minTLSVersion or ciphers.
type APIServerTLSProfile struct {
	Type         string              `json:"type"`
	Custom       *APIServerCustomTLS `json:"custom,omitempty"`
	Intermediate *struct{}           `json:"intermediate,omitempty"`
	Modern       *struct{}           `json:"modern,omitempty"`
	Old          *struct{}           `json:"old,omitempty"`
}

// APIServerCustomTLS is the custom profile payload for APIServerTLSProfile.
type APIServerCustomTLS struct {
	MinTLSVersion string   `json:"minTLSVersion,omitempty"`
	Ciphers       []string `json:"ciphers,omitempty"`
	Groups        []string `json:"groups,omitempty"`
}

// TLSEndpoint is the Service an openssl s_client probe reaches through oc port-forward.
type TLSEndpoint struct {
	Namespace  string
	Service    string
	Port       string
	ServerName string
}

// DeploymentTLSTarget identifies a deployment and the env vars that carry its TLS settings.
type DeploymentTLSTarget struct {
	Namespace  string
	Deployment string
	MinEnv     string
	GroupsEnv  string
	CiphersEnv string
}

// ObservedTLSEnv is the TLS-related env observed on ready pods.
type ObservedTLSEnv struct {
	MinVersion    string
	MinFound      bool
	Groups        string
	GroupsFound   bool
	Ciphers       string
	CiphersFound  bool
	ReadyPodCount int
}

// TLSEnvExpectation is the env a component should reach after a profile change.
// GroupsSpecified requires Groups to be present and equal. Otherwise the groups
// env var must be absent or empty.
type TLSEnvExpectation struct {
	MinVersion      string
	AllowUnsetMin   bool
	Groups          []string
	GroupsSpecified bool
}

// NamedAPIServerTLSProfile returns an Old, Intermediate, or Modern profile patch.
func NamedAPIServerTLSProfile(profileType string) APIServerTLSProfile {
	profile := APIServerTLSProfile{Type: profileType}
	switch profileType {
	case string(configv1.TLSProfileIntermediateType):
		profile.Intermediate = &struct{}{}
	case string(configv1.TLSProfileModernType):
		profile.Modern = &struct{}{}
	case string(configv1.TLSProfileOldType):
		profile.Old = &struct{}{}
	}
	return profile
}

// CustomAPIServerTLSProfile returns a custom profile patch. Empty minTLSVersion,
// ciphers, or groups are omitted from the JSON body.
func CustomAPIServerTLSProfile(minTLSVersion string, ciphers, groups []string) APIServerTLSProfile {
	return APIServerTLSProfile{
		Type: string(configv1.TLSProfileCustomType),
		Custom: &APIServerCustomTLS{
			MinTLSVersion: minTLSVersion,
			Ciphers:       ciphers,
			Groups:        groups,
		},
	}
}

// DefaultTLSProfileGroups is the group list OpenShift applies for the Intermediate
// and Modern profiles. FIPS drops X25519 and X25519MLKEM768.
func DefaultTLSProfileGroups(fips bool) []string {
	if fips {
		return []string{
			string(configv1.TLSGroupSecP256r1),
			string(configv1.TLSGroupSecP384r1),
		}
	}
	return []string{
		string(configv1.TLSGroupX25519MLKEM768),
		string(configv1.TLSGroupX25519),
		string(configv1.TLSGroupSecP256r1),
		string(configv1.TLSGroupSecP384r1),
	}
}

// EffectiveMinTLSVersion is the minTLSVersion operators publish for profile.
// A nil profile uses the Intermediate profile. This matches the value the
// tls-observed-config suite compares against.
func EffectiveMinTLSVersion(profile *configv1.TLSSecurityProfile) string {
	profileType := configv1.TLSProfileIntermediateType
	if profile != nil && profile.Type != "" {
		profileType = profile.Type
	}

	var spec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile != nil && profile.Custom != nil {
			spec = &profile.Custom.TLSProfileSpec
		}
	} else {
		spec = configv1.TLSProfiles[profileType]
	}
	if spec == nil {
		spec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	}
	return string(spec.MinTLSVersion)
}

// TLSEnvForProfile is the deployment env a component should reach for profile.
// Named profiles publish the platform default groups. A custom profile with no
// groups leaves the groups env unset.
func TLSEnvForProfile(profile *configv1.TLSSecurityProfile, fips bool) TLSEnvExpectation {
	groups, specified := groupsForProfile(profile, fips)
	return TLSEnvExpectation{
		MinVersion:      EffectiveMinTLSVersion(profile),
		Groups:          groups,
		GroupsSpecified: specified,
	}
}

func groupsForProfile(profile *configv1.TLSSecurityProfile, fips bool) ([]string, bool) {
	if profile == nil || profile.Type == "" || profile.Type != configv1.TLSProfileCustomType {
		return DefaultTLSProfileGroups(fips), true
	}
	if profile.Custom == nil || len(profile.Custom.Groups) == 0 {
		return nil, false
	}
	groups := make([]string, 0, len(profile.Custom.Groups))
	for _, group := range profile.Custom.Groups {
		name := string(group)
		if fips && (name == string(configv1.TLSGroupX25519) || name == string(configv1.TLSGroupX25519MLKEM768)) {
			continue
		}
		groups = append(groups, name)
	}
	if len(groups) == 0 {
		return nil, false
	}
	return groups, true
}

// TLS12OrderedGroupOffer builds a TLS 1.2 client group list. X25519MLKEM768 is omitted
// because it is a TLS 1.3 group. X25519 is omitted on FIPS.
func TLS12OrderedGroupOffer(first, second string, fips bool) []string {
	offer := []string{first, second}
	if !fips {
		offer = append(offer, string(configv1.TLSGroupX25519))
	}
	return offer
}

// OtherTLSGroupsFirst returns a client offer that lists want last, so a server
// that ignores the client order cannot hide behind the first offered group.
func OtherTLSGroupsFirst(want string, fips bool) []string {
	candidates := []string{
		string(configv1.TLSGroupSecP256r1),
		string(configv1.TLSGroupSecP384r1),
		string(configv1.TLSGroupSecP521r1),
	}
	if !fips {
		candidates = append([]string{
			string(configv1.TLSGroupX25519MLKEM768),
			string(configv1.TLSGroupX25519),
		}, candidates...)
	}
	offer := make([]string, 0, len(candidates))
	for _, group := range candidates {
		if group != want {
			offer = append(offer, group)
		}
	}
	return append(offer, want)
}

// TLSGroupPreferencesEnabled reports whether the cluster FeatureGate status
// lists TLSGroupPreferences as enabled.
func TLSGroupPreferencesEnabled(ctx context.Context, oc *CLI) (bool, error) {
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, version := range featureGate.Status.FeatureGates {
		for _, enabled := range version.Enabled {
			if enabled.Name == features.FeatureGateTLSGroupPreferences {
				return true, nil
			}
		}
	}
	return false, nil
}

// PatchAPIServerTLSProfile JSON-patches /spec/tlsSecurityProfile. Use this instead
// of a typed update when fields must be omitted.
func PatchAPIServerTLSProfile(ctx context.Context, oc *CLI, profile APIServerTLSProfile) error {
	rawProfile, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		op := "replace"
		if current.Spec.TLSSecurityProfile == nil {
			op = "add"
		}
		patch := []byte(fmt.Sprintf(`[{"op":"%s","path":"/spec/tlsSecurityProfile","value":%s}]`, op, rawProfile))
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Patch(ctx, "cluster", types.JSONPatchType, patch, metav1.PatchOptions{})
		return err
	})
}

// RestoreAPIServerTLSProfile writes the saved profile back with a typed update.
// A nil profile clears spec.tlsSecurityProfile.
func RestoreAPIServerTLSProfile(ctx context.Context, oc *CLI, original *configv1.TLSSecurityProfile) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		current.Spec.TLSSecurityProfile = original
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
}

// ExpectAPIServerTLSProfileType checks the stored APIServer profile type.
func ExpectAPIServerTLSProfileType(ctx context.Context, oc *CLI, profileType configv1.TLSProfileType) {
	apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(apiServer.Spec.TLSSecurityProfile).NotTo(o.BeNil())
	o.Expect(apiServer.Spec.TLSSecurityProfile.Type).To(o.Equal(profileType))
}

// ExpectInvalidTLSGroupError asserts that admission rejected a profile because
// it names a TLS group that is not configurable.
func ExpectInvalidTLSGroupError(err error) {
	o.Expect(err).To(o.HaveOccurred())
	msg := strings.ToLower(err.Error())
	o.Expect(msg).To(o.ContainSubstring(InvalidTLSGroupName))
	o.Expect(msg).To(o.Or(
		o.ContainSubstring("unsupported"),
		o.ContainSubstring("invalid"),
		o.ContainSubstring("not configurable"),
	))
}

// ExpectMissingTLSCipherError asserts that admission rejected a custom profile
// whose minimum is below TLS 1.3 and that has no usable cipher suite.
func ExpectMissingTLSCipherError(err error) {
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(err.Error()).To(o.ContainSubstring("no supported cipher suite found"))
	o.Expect(err.Error()).To(o.ContainSubstring("AES_128_GCM_SHA256"))
}

// ExpectTLS13CipherError asserts that admission rejected cipher suites on TLS 1.3.
func ExpectTLS13CipherError(err error) {
	o.Expect(err).To(o.HaveOccurred())
	o.Expect(err.Error()).To(o.ContainSubstring("TLS 1.3"))
	o.Expect(strings.ToLower(err.Error())).To(o.ContainSubstring("not configurable"))
}

// ReadDeploymentTLSEnv returns TLS env from ready pods once the deployment has rolled out.
// Ready pods must agree on the minimum version and groups.
func ReadDeploymentTLSEnv(ctx context.Context, oc *CLI, target DeploymentTLSTarget) (ObservedTLSEnv, error) {
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(target.Namespace).Get(ctx, target.Deployment, metav1.GetOptions{})
	if err != nil {
		return ObservedTLSEnv{}, err
	}
	if !deploymentRolledOut(deployment) {
		return ObservedTLSEnv{}, ErrDeploymentRolling
	}

	selector := deployment.Spec.Selector.MatchLabels
	if len(selector) == 0 {
		selector = deployment.Spec.Template.Labels
	}
	pods, err := oc.AdminKubeClient().CoreV1().Pods(target.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector).String(),
	})
	if err != nil {
		return ObservedTLSEnv{}, err
	}

	var ready []ObservedTLSEnv
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !podReady(pod) {
			continue
		}
		envMap := envFromContainers(pod.Spec.Containers)
		observed := ObservedTLSEnv{ReadyPodCount: 1}
		observed.MinVersion, observed.MinFound = envMap[target.MinEnv]
		observed.Groups, observed.GroupsFound = envMap[target.GroupsEnv]
		observed.Ciphers, observed.CiphersFound = envMap[target.CiphersEnv]
		ready = append(ready, observed)
	}
	if len(ready) == 0 {
		return ObservedTLSEnv{}, ErrDeploymentRolling
	}
	first := ready[0]
	first.ReadyPodCount = len(ready)
	for _, other := range ready[1:] {
		if other.MinVersion != first.MinVersion || other.MinFound != first.MinFound ||
			other.Groups != first.Groups || other.GroupsFound != first.GroupsFound {
			return ObservedTLSEnv{}, fmt.Errorf("ready pods in %s/%s disagree on TLS env: %+v vs %+v", target.Namespace, target.Deployment, first, other)
		}
	}
	return first, nil
}

// WaitForDeploymentTLSEnv polls until the deployment's TLS env matches exp or the timeout elapses.
func WaitForDeploymentTLSEnv(ctx context.Context, oc *CLI, target DeploymentTLSTarget, exp TLSEnvExpectation, timeout time.Duration) error {
	var last ObservedTLSEnv
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		env, readErr := ReadDeploymentTLSEnv(ctx, oc, target)
		last = env
		lastErr = readErr
		if readErr != nil {
			if errors.Is(readErr, ErrDeploymentRolling) {
				e2e.Logf("%s/%s is still rolling out", target.Namespace, target.Deployment)
				return false, nil
			}
			e2e.Logf("waiting for %s/%s TLS env: %v", target.Namespace, target.Deployment, readErr)
			return false, nil
		}
		if TLSEnvMatches(env, exp) {
			e2e.Logf("%s/%s TLS env matches: min=%s groups=%s", target.Namespace, target.Deployment, env.MinVersion, env.Groups)
			return true, nil
		}
		e2e.Logf("%s/%s TLS env min=%q (found=%v) groups=%q (found=%v); waiting for %+v",
			target.Namespace, target.Deployment, env.MinVersion, env.MinFound, env.Groups, env.GroupsFound, exp)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("%s/%s TLS env did not converge: %w (last=%+v lastErr=%v)", target.Namespace, target.Deployment, err, last, lastErr)
	}
	return nil
}

// WaitForDeploymentRollout polls until the deployment is available. It does not check env values.
func WaitForDeploymentRollout(ctx context.Context, oc *CLI, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := ReadDeploymentTLSEnv(ctx, oc, DeploymentTLSTarget{Namespace: namespace, Deployment: name})
		if errors.Is(err, ErrDeploymentRolling) {
			return false, nil
		}
		return err == nil, err
	})
}

// TLSEnvMatches reports whether the observed env satisfies exp.
// An empty expected minimum accepts an unset value or VersionTLS12.
func TLSEnvMatches(env ObservedTLSEnv, exp TLSEnvExpectation) bool {
	if exp.GroupsSpecified {
		if !env.GroupsFound || normalizeGroupList(env.Groups) != normalizeGroupList(strings.Join(exp.Groups, ",")) {
			return false
		}
	} else if env.GroupsFound && normalizeGroupList(env.Groups) != "" {
		return false
	}

	min := strings.TrimSpace(env.MinVersion)
	if exp.MinVersion == "" {
		return !env.MinFound || min == "" || min == string(configv1.VersionTLS12)
	}
	if !env.MinFound || min == "" {
		return exp.AllowUnsetMin
	}
	return min == exp.MinVersion
}

// ExpectNegotiatedTLSGroup port-forwards to endpoint and requires openssl s_client
// to negotiate want with the given client group order.
func ExpectNegotiatedTLSGroup(ctx context.Context, endpoint TLSEndpoint, tlsFlag string, clientGroups []string, want string) {
	g.By(fmt.Sprintf("openssl s_client %s -groups %s", tlsFlag, strings.Join(clientGroups, ":")))
	protocol, got, output, err := ProbeServiceTLS(ctx, endpoint, tlsFlag, clientGroups)
	o.Expect(err).NotTo(o.HaveOccurred(), "openssl output:\n%s", truncateForLog(output))
	o.Expect(protocol).To(o.Equal(OpenSSLProtocolName(tlsFlag)), "openssl output:\n%s", truncateForLog(output))
	o.Expect(got).To(o.Equal(want), "openssl output:\n%s", truncateForLog(output))
}

// ExpectTLSGroupOneOf is ExpectNegotiatedTLSGroup when several negotiated groups are acceptable.
func ExpectTLSGroupOneOf(ctx context.Context, endpoint TLSEndpoint, tlsFlag string, clientGroups, allowed []string) {
	g.By(fmt.Sprintf("openssl s_client %s -groups %s", tlsFlag, strings.Join(clientGroups, ":")))
	protocol, got, output, err := ProbeServiceTLS(ctx, endpoint, tlsFlag, clientGroups)
	o.Expect(err).NotTo(o.HaveOccurred(), "openssl output:\n%s", truncateForLog(output))
	o.Expect(protocol).To(o.Equal(OpenSSLProtocolName(tlsFlag)), "openssl output:\n%s", truncateForLog(output))
	o.Expect(allowed).To(o.ContainElement(got), "openssl output:\n%s", truncateForLog(output))
}

// ExpectTLSVersionRejected requires the handshake for tlsFlag to fail.
// A missing openssl flag is logged and skipped.
func ExpectTLSVersionRejected(ctx context.Context, endpoint TLSEndpoint, tlsFlag string) {
	g.By(fmt.Sprintf("openssl s_client %s is rejected", tlsFlag))
	err := serviceRejectsTLS(ctx, endpoint, tlsFlag)
	if errors.Is(err, ErrOpenSSLFlagUnsupported) {
		e2e.Logf("openssl does not support %s; skipping the rejection check", tlsFlag)
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

// ProbeServiceTLS port-forwards to endpoint and returns the protocol and negotiated group.
func ProbeServiceTLS(ctx context.Context, endpoint TLSEndpoint, tlsFlag string, groups []string) (string, string, string, error) {
	var output, protocol, group string
	err := ForwardPortAndExecute(endpoint.Service, endpoint.Namespace, endpoint.Port, func(port int) error {
		var runErr error
		output, runErr = runOpenSSLClient(ctx, port, endpoint.ServerName, tlsFlag, groups)
		var parsed bool
		protocol, group, parsed = ParseOpenSSLHandshake(output)
		if parsed {
			return nil
		}
		if runErr != nil {
			return fmt.Errorf("openssl %s -groups %s: %w\n%s", tlsFlag, strings.Join(groups, ":"), runErr, truncateForLog(output))
		}
		return fmt.Errorf("openssl %s did not report a negotiated TLS group\n%s", tlsFlag, truncateForLog(output))
	})
	return protocol, group, output, err
}

// ParseOpenSSLHandshake extracts the protocol and negotiated group from openssl s_client output.
// The TLS 1.3 negotiated-group line wins over a peer or server temp key.
func ParseOpenSSLHandshake(output string) (protocol, group string, ok bool) {
	if match := openSSLProtocolPattern.FindStringSubmatch(output); len(match) > 1 {
		protocol = match[1]
	}
	for _, pattern := range openSSLGroupPatterns {
		if match := pattern.FindStringSubmatch(output); len(match) > 1 {
			group = NormalizeNegotiatedGroup(match[1])
			break
		}
	}
	return protocol, group, protocol != "" && group != ""
}

// NormalizeNegotiatedGroup maps an openssl group name onto the APIServer TLSGroup value.
func NormalizeNegotiatedGroup(raw string) string {
	value := strings.ToLower(raw)
	switch {
	case strings.Contains(value, "x25519mlkem768"):
		return string(configv1.TLSGroupX25519MLKEM768)
	case strings.Contains(value, "secp256r1mlkem768"):
		return string(configv1.TLSGroupSecP256r1MLKEM768)
	case strings.Contains(value, "secp384r1mlkem1024"):
		return string(configv1.TLSGroupSecP384r1MLKEM1024)
	case strings.Contains(value, "x25519"):
		return string(configv1.TLSGroupX25519)
	case strings.Contains(value, "p-521"), strings.Contains(value, "secp521r1"), strings.Contains(value, "p521"):
		return string(configv1.TLSGroupSecP521r1)
	case strings.Contains(value, "p-384"), strings.Contains(value, "secp384r1"):
		return string(configv1.TLSGroupSecP384r1)
	case strings.Contains(value, "p-256"), strings.Contains(value, "secp256r1"), strings.Contains(value, "prime256v1"):
		return string(configv1.TLSGroupSecP256r1)
	default:
		return strings.TrimSpace(raw)
	}
}

// OpenSSLProtocolName maps an openssl TLS flag to the Protocol line openssl prints.
func OpenSSLProtocolName(tlsFlag string) string {
	switch tlsFlag {
	case TLS13Flag:
		return "TLSv1.3"
	case TLS12Flag:
		return "TLSv1.2"
	case TLS11Flag:
		return "TLSv1.1"
	default:
		return ""
	}
}

func serviceRejectsTLS(ctx context.Context, endpoint TLSEndpoint, tlsFlag string) error {
	return ForwardPortAndExecute(endpoint.Service, endpoint.Namespace, endpoint.Port, func(port int) error {
		output, runErr := runOpenSSLClient(ctx, port, endpoint.ServerName, tlsFlag, nil)
		if opensslFlagUnsupported(output) {
			return ErrOpenSSLFlagUnsupported
		}
		protocol, _, parsed := ParseOpenSSLHandshake(output)
		if parsed && protocol == OpenSSLProtocolName(tlsFlag) {
			return fmt.Errorf("handshake succeeded with %s (%s)\n%s", tlsFlag, protocol, truncateForLog(output))
		}
		if strings.TrimSpace(output) == "" && runErr != nil {
			return fmt.Errorf("openssl produced no output: %w", runErr)
		}
		e2e.Logf("openssl %s rejected the handshake", tlsFlag)
		return nil
	})
}

func runOpenSSLClient(ctx context.Context, port int, serverName, tlsFlag string, groups []string) (string, error) {
	args := []string{
		"s_client",
		"-connect", fmt.Sprintf("127.0.0.1:%d", port),
		"-servername", serverName,
		tlsFlag,
	}
	if len(groups) > 0 {
		args = append(args, "-groups", strings.Join(groups, ":"))
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "openssl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func deploymentRolledOut(deployment *appsv1.Deployment) bool {
	if deployment.Spec.Replicas == nil || deployment.Status.ObservedGeneration < deployment.Generation {
		return false
	}
	replicas := *deployment.Spec.Replicas
	return replicas > 0 &&
		deployment.Status.UpdatedReplicas == replicas &&
		deployment.Status.ReadyReplicas == replicas &&
		deployment.Status.AvailableReplicas == replicas &&
		deployment.Status.UnavailableReplicas == 0
}

func podReady(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func envFromContainers(containers []corev1.Container) map[string]string {
	values := map[string]string{}
	for _, container := range containers {
		for _, env := range container.Env {
			if _, exists := values[env.Name]; !exists {
				values[env.Name] = env.Value
			}
		}
	}
	return values
}

func normalizeGroupList(value string) string {
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, ",")
}

var (
	openSSLProtocolPattern = regexp.MustCompile(`(?i)Protocol\s*:\s*(TLSv1\.[0-3])`)
	openSSLGroupPatterns   = []*regexp.Regexp{
		regexp.MustCompile(`(?im)^[ \t]*Negotiated TLS1\.3 group\s*:\s*(.+?)\s*$`),
		regexp.MustCompile(`(?im)^[ \t]*Server Temp Key\s*:\s*(.+?)\s*$`),
		regexp.MustCompile(`(?im)^[ \t]*Peer Temp Key\s*:\s*(.+?)\s*$`),
	}
)

func opensslFlagUnsupported(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "unknown option") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "bad option") ||
		strings.Contains(lower, "option unknown")
}

func truncateForLog(value string) string {
	const limit = 2000
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... truncated"
}

const restoredTLSStableFor = 2 * time.Minute

// apiServerTLSObservers are the operator configs and ConfigMaps that the
// tls-observed-config suite reads. A disruptive test that changes the APIServer
// profile has to wait for this set, not only for its own operand.
var apiServerTLSObservers = struct {
	observed  []observedTLSMinTarget
	configMap []configMapTLSMinTarget
}{
	observed: []observedTLSMinTarget{
		{key: "configs/cluster", gvr: schema.GroupVersionResource{Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "configs"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "openshiftcontrollermanagers/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftcontrollermanagers"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "kubeapiservers/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "kubeapiservers"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "openshiftapiservers/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftapiservers"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "etcds/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "etcds"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "kubecontrollermanagers/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "kubecontrollermanagers"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "kubeschedulers/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "kubeschedulers"}, name: "cluster", path: []string{"servingInfo"}},
		{key: "authentications/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "authentications"}, name: "cluster", path: []string{"oauthServer", "servingInfo"}},
		{key: "servicecas/cluster", gvr: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "servicecas"}, name: "cluster", path: []string{"servingInfo"}},
	},
	configMap: []configMapTLSMinTarget{
		{namespace: "openshift-image-registry", name: "image-registry-operator-config", key: "config.yaml"},
		{namespace: "openshift-controller-manager-operator", name: "openshift-controller-manager-operator-config", key: "config.yaml"},
		{namespace: "openshift-kube-apiserver-operator", name: "kube-apiserver-operator-config", key: "config.yaml"},
		{namespace: "openshift-apiserver-operator", name: "openshift-apiserver-operator-config", key: "config.yaml"},
		{namespace: "openshift-etcd-operator", name: "etcd-operator-config", key: "config.yaml"},
		{namespace: "openshift-kube-controller-manager-operator", name: "kube-controller-manager-operator-config", key: "config.yaml"},
		{namespace: "openshift-kube-scheduler-operator", name: "openshift-kube-scheduler-operator-config", key: "config.yaml"},
		{namespace: "openshift-cluster-samples-operator", name: "samples-operator-config", key: "config.yaml"},
		{namespace: "openshift-authentication-operator", name: "authentication-operator-config", key: "operator-config.yaml"},
		{namespace: "openshift-service-ca", name: "service-ca-controller-config", key: "controller-config.yaml"},
	},
}

var tlsConsumingOperators = []string{
	"authentication",
	"etcd",
	"image-registry",
	"kube-apiserver",
	"kube-controller-manager",
	"kube-scheduler",
	"openshift-apiserver",
	"openshift-controller-manager",
	"openshift-samples",
	"service-ca",
}

type observedTLSMinTarget struct {
	key  string
	gvr  schema.GroupVersionResource
	name string
	path []string
}

type configMapTLSMinTarget struct {
	namespace string
	name      string
	key       string
}

type tlsMinReading struct {
	min    string
	found  bool
	absent bool
}

// RestoreAPIServerTLSAndWait writes profile back onto the APIServer and waits
// until the operators that publish it have observed that minimum version and
// finished rolling out. A deployment that is merely Available can still be the
// previous revision.
func RestoreAPIServerTLSAndWait(ctx context.Context, oc *CLI, profile *configv1.TLSSecurityProfile, fips bool, deployment DeploymentTLSTarget, timeout time.Duration) error {
	current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to read APIServer before restoring TLS profile: %w", err)
	}
	if equality.Semantic.DeepEqual(current.Spec.TLSSecurityProfile, profile) {
		e2e.Logf("APIServer TLS profile already matches the saved profile")
		return nil
	}
	if err := RestoreAPIServerTLSProfile(ctx, oc, profile); err != nil {
		return err
	}

	want := EffectiveMinTLSVersion(profile)
	envExpectation := TLSEnvForProfile(profile, fips)
	e2e.Logf("waiting up to %s for operators to observe minTLSVersion %s", timeout, want)

	var stableSince time.Time
	var lastProblems []string
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		problems := restoredTLSProblems(ctx, oc, want, envExpectation, deployment)
		lastProblems = problems
		if len(problems) > 0 {
			stableSince = time.Time{}
			e2e.Logf("restored TLS profile is not fully observed:\n  %s", strings.Join(problems, "\n  "))
			return false, nil
		}
		if stableSince.IsZero() {
			stableSince = time.Now()
			e2e.Logf("restored TLS profile is observed; waiting %s for operators to stay settled", restoredTLSStableFor)
			return false, nil
		}
		if time.Since(stableSince) < restoredTLSStableFor {
			return false, nil
		}
		e2e.Logf("operators observed restored minTLSVersion %s", want)
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("restored APIServer TLS profile was not observed within %s: %w\n%s", timeout, err, strings.Join(lastProblems, "\n"))
	}
	return nil
}

func (t configMapTLSMinTarget) mapKey() string {
	return t.namespace + "/" + t.name
}

func restoredTLSProblems(ctx context.Context, oc *CLI, want string, envExpectation TLSEnvExpectation, deployment DeploymentTLSTarget) []string {
	var problems []string
	for _, target := range apiServerTLSObservers.observed {
		reading, err := readObservedTLSMin(ctx, oc, target)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", target.key, err))
			continue
		}
		if reading.absent {
			continue
		}
		if !reading.found || reading.min != want {
			problems = append(problems, fmt.Sprintf("%s minTLSVersion=%q, expected %s", target.key, reading.min, want))
		}
	}
	for _, target := range apiServerTLSObservers.configMap {
		reading, err := readConfigMapTLSMin(ctx, oc, target)
		if err != nil {
			problems = append(problems, fmt.Sprintf("configmap %s: %v", target.mapKey(), err))
			continue
		}
		if reading.absent {
			continue
		}
		if !reading.found || reading.min != want {
			problems = append(problems, fmt.Sprintf("configmap %s minTLSVersion=%q, expected %s", target.mapKey(), reading.min, want))
		}
	}
	if deployment.Namespace != "" && deployment.Deployment != "" {
		env, err := ReadDeploymentTLSEnv(ctx, oc, deployment)
		switch {
		case errors.Is(err, ErrDeploymentRolling):
			problems = append(problems, fmt.Sprintf("%s/%s is rolling out", deployment.Namespace, deployment.Deployment))
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s/%s: %v", deployment.Namespace, deployment.Deployment, err))
		case !TLSEnvMatches(env, envExpectation):
			problems = append(problems, fmt.Sprintf("%s/%s env min=%q groups=%q, expected min=%s groups=%v specified=%v",
				deployment.Namespace, deployment.Deployment, env.MinVersion, env.Groups, envExpectation.MinVersion, envExpectation.Groups, envExpectation.GroupsSpecified))
		}
	}
	settled, detail, err := tlsOperatorsSettled(ctx, oc)
	switch {
	case err != nil:
		problems = append(problems, fmt.Sprintf("cluster operators: %v", err))
	case !settled:
		problems = append(problems, detail)
	}
	return problems
}

func readObservedTLSMin(ctx context.Context, oc *CLI, target observedTLSMinTarget) (tlsMinReading, error) {
	obj, err := oc.AdminDynamicClient().Resource(target.gvr).Get(ctx, target.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return tlsMinReading{absent: true}, nil
	}
	if err != nil {
		return tlsMinReading{}, err
	}
	path := append(append([]string{"spec", "observedConfig"}, target.path...), "minTLSVersion")
	min, found, err := unstructured.NestedString(obj.Object, path...)
	if err != nil {
		return tlsMinReading{}, err
	}
	return tlsMinReading{min: min, found: found}, nil
}

func readConfigMapTLSMin(ctx context.Context, oc *CLI, target configMapTLSMinTarget) (tlsMinReading, error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(target.namespace).Get(ctx, target.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return tlsMinReading{absent: true}, nil
	}
	if err != nil {
		return tlsMinReading{}, err
	}
	configData, found := cm.Data[target.key]
	if !found || configData == "" {
		return tlsMinReading{}, nil
	}
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(configData), &parsed); err != nil {
		return tlsMinReading{}, fmt.Errorf("parse %s/%s %s: %w", target.namespace, target.name, target.key, err)
	}
	min, found, err := unstructured.NestedString(parsed, "servingInfo", "minTLSVersion")
	if err != nil {
		return tlsMinReading{}, err
	}
	return tlsMinReading{min: min, found: found}, nil
}

func tlsOperatorsSettled(ctx context.Context, oc *CLI) (bool, string, error) {
	var problems []string
	for _, name := range tlsConsumingOperators {
		operator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, "", err
		}
		available := operatorConditionStatus(operator.Status.Conditions, configv1.OperatorAvailable)
		degraded := operatorConditionStatus(operator.Status.Conditions, configv1.OperatorDegraded)
		progressing := operatorConditionStatus(operator.Status.Conditions, configv1.OperatorProgressing)
		if available == configv1.ConditionTrue && degraded == configv1.ConditionFalse && progressing == configv1.ConditionFalse {
			continue
		}
		problems = append(problems, fmt.Sprintf("clusteroperator/%s Available=%s Progressing=%s Degraded=%s", name, available, progressing, degraded))
	}
	if len(problems) > 0 {
		return false, strings.Join(problems, "; "), nil
	}
	return true, "", nil
}

func operatorConditionStatus(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) configv1.ConditionStatus {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return configv1.ConditionUnknown
}
