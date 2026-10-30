package images

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

// Image registry TLS group coverage for:
//   - https://github.com/openshift/cluster-image-registry-operator/pull/1379
//   - https://github.com/openshift/image-registry/pull/488
//
// Shared profile, env, and openssl helpers live in test/extended/util/tls_groups.go.

const (
	registryNamespace    = "openshift-image-registry"
	registryDeployment   = "image-registry"
	registryService      = "image-registry"
	registryPort         = "5000"
	registryServiceSNI   = "image-registry.openshift-image-registry.svc"
	registryGroupsEnv    = "OPENSHIFT_REGISTRY_HTTP_TLS_GROUPS"
	registryMinEnv       = "REGISTRY_HTTP_TLS_MINVERSION"
	registryCiphersEnv   = "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES"
	registryRolloutLimit = 8 * time.Minute
)

type tlsGroupsTest struct {
	oc       *exutil.CLI
	ctx      context.Context
	fips     bool
	original *configv1.TLSSecurityProfile
}

func registryTLSTarget() exutil.DeploymentTLSTarget {
	return exutil.DeploymentTLSTarget{
		Namespace:  registryNamespace,
		Deployment: registryDeployment,
		MinEnv:     registryMinEnv,
		GroupsEnv:  registryGroupsEnv,
		CiphersEnv: registryCiphersEnv,
	}
}

func registryTLSEndpoint() exutil.TLSEndpoint {
	return exutil.TLSEndpoint{
		Namespace:  registryNamespace,
		Service:    registryService,
		Port:       registryPort,
		ServerName: registryServiceSNI,
	}
}

var _ = g.Describe("[sig-imageregistry][OCPFeatureGate:TLSGroupPreferences][Serial][Disruptive][Suite:openshift/tls-observed-config][apigroup:config.openshift.io][apigroup:imageregistry.operator.openshift.io] Image registry TLS groups", func() {
	defer g.GinkgoRecover()

	// NewCLI registers BeforeEach and AfterEach, so it has to run while the
	// spec tree is being built. Calling it from a running BeforeEach makes
	// Ginkgo exit before the spec can report a result.
	oc := exutil.NewCLI("registry-tls-groups")
	var harness *tlsGroupsTest

	g.BeforeEach(func() {
		harness = setupTLSGroupsTest(oc)
	})

	g.AfterEach(func() {
		if harness == nil {
			return
		}
		harness.restore()
	})

	// TLS 1.2 runs before TLS 1.3. Origin starts each spec in its own process,
	// so each spec restores the APIServer profile before it returns.
	g.It("TLS 1.2 validates registry TLS group configuration [Timeout:120m]", func() {
		harness.runTLS12Cases()
	})

	g.It("TLS 1.3 validates registry TLS group configuration [Timeout:120m]", func() {
		harness.runTLS13Cases()
	})
})

func setupTLSGroupsTest(oc *exutil.CLI) *tlsGroupsTest {
	ctx := context.Background()

	isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
	o.Expect(err).NotTo(o.HaveOccurred())
	if isMicroShift {
		g.Skip("image registry TLS group tests do not apply to MicroShift")
	}
	if _, err := exec.LookPath("openssl"); err != nil {
		g.Skip("openssl is required to probe the image registry TLS handshake")
	}
	if _, err := exec.LookPath("oc"); err != nil {
		g.Skip("oc is required to port-forward to the image registry")
	}

	enabled, err := exutil.TLSGroupPreferencesEnabled(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	if !enabled {
		g.Skip("TLSGroupPreferences is not enabled")
	}

	registryClient, err := imageregistry.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	registryConfig, err := registryClient.ImageregistryV1().Configs().Get(ctx, "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip("image registry config is not installed")
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	if registryConfig.Spec.ManagementState == operatorv1.Removed || registryConfig.Spec.ManagementState == operatorv1.Unmanaged {
		g.Skip(fmt.Sprintf("image registry managementState is %s", registryConfig.Spec.ManagementState))
	}
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(registryNamespace).Get(ctx, registryDeployment, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip("image registry deployment is not installed")
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas == 0 {
		g.Skip("image registry deployment has no replicas")
	}

	fips, err := exutil.IsFIPS(oc.AdminKubeClient().CoreV1())
	o.Expect(err).NotTo(o.HaveOccurred())

	apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	var original *configv1.TLSSecurityProfile
	if apiServer.Spec.TLSSecurityProfile != nil {
		original = apiServer.Spec.TLSSecurityProfile.DeepCopy()
	}

	return &tlsGroupsTest{oc: oc, ctx: ctx, fips: fips, original: original}
}

func (t *tlsGroupsTest) restore() {
	// A ready image-registry deployment is not enough. Its current revision stays
	// Available until the operator starts the next rollout, and the other
	// APIServer observers (kube-apiserver, etcd, openshift-apiserver) lag behind
	// the registry. The next tls-observed-config spec reads that restored profile
	// immediately, so this has to wait until those components report it.
	g.By("restoring the original APIServer TLS profile and waiting for operators to observe it")
	o.Expect(exutil.RestoreAPIServerTLSAndWait(t.ctx, t.oc, t.original, t.fips, registryTLSTarget(), 25*time.Minute)).NotTo(o.HaveOccurred())
}

func (t *tlsGroupsTest) runTLS12Cases() {
	singleGroup := []string{string(configv1.TLSGroupSecP384r1)}
	subsetGroups := []string{string(configv1.TLSGroupSecP256r1), string(configv1.TLSGroupSecP384r1)}
	defaults := exutil.DefaultTLSProfileGroups(t.fips)

	g.By("rejecting an invalid TLS group")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), exutil.TLS12HTTP2Ciphers, []string{exutil.InvalidTLSGroupName}), exutil.ExpectInvalidTLSGroupError)

	g.By("rejecting a mix of valid and invalid TLS groups")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), exutil.TLS12HTTP2Ciphers, []string{
		string(configv1.TLSGroupSecP256r1),
		exutil.InvalidTLSGroupName,
	}), exutil.ExpectInvalidTLSGroupError)

	g.By("applying the intermediate profile so the registry has the default TLS groups")
	t.apply(exutil.NamedAPIServerTLSProfile(string(configv1.TLSProfileIntermediateType)))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS12),
		Groups:          defaults,
		GroupsSpecified: true,
	})

	g.By("accepting a TLS 1.2 cipher list that contains one valid cipher")
	err := exutil.PatchAPIServerTLSProfile(t.ctx, t.oc, exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), []string{
		"NOT-A-CIPHER-SUITE",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
		"ECDHE-RSA-AES128-GCM-SHA256",
	}, nil))
	o.Expect(err).NotTo(o.HaveOccurred())
	exutil.ExpectAPIServerTLSProfileType(t.ctx, t.oc, configv1.TLSProfileCustomType)
	// Admission accepts the profile when one cipher is usable. The minimum
	// version stays VersionTLS12 (intermediate). Groups are not set on this
	// custom profile, so the registry uses Go's default groups.
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion: string(configv1.VersionTLS12),
	})
	t.expectCipherEnvDropsUnknown()
	t.expectNegotiated(exutil.TLS12Flag, exutil.TLS12OrderedGroupOffer(string(configv1.TLSGroupSecP384r1), string(configv1.TLSGroupSecP256r1), t.fips), string(configv1.TLSGroupSecP384r1))
	t.expectNegotiated(exutil.TLS12Flag, []string{string(configv1.TLSGroupSecP521r1)}, string(configv1.TLSGroupSecP521r1))

	// A custom profile whose minimum is below TLS 1.3 must include an HTTP/2 cipher.
	// APIServer admission rejects groups alone, before the registry operator can apply them.
	g.By("rejecting a single TLS group without a cipher list or minTLSVersion")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile("", nil, singleGroup), exutil.ExpectMissingTLSCipherError)

	g.By("applying a single TLS group with a cipher list and the default intermediate minimum version")
	t.apply(exutil.CustomAPIServerTLSProfile("", exutil.TLS12HTTP2Ciphers, singleGroup))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS12),
		AllowUnsetMin:   true,
		Groups:          singleGroup,
		GroupsSpecified: true,
	})
	t.expectNegotiated(exutil.TLS12Flag, exutil.OtherTLSGroupsFirst(singleGroup[0], t.fips), singleGroup[0])

	g.By("applying a single TLS group with minTLSVersion VersionTLS12")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), exutil.TLS12HTTP2Ciphers, singleGroup))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS12),
		Groups:          singleGroup,
		GroupsSpecified: true,
	})
	t.expectNegotiated(exutil.TLS12Flag, exutil.OtherTLSGroupsFirst(singleGroup[0], t.fips), singleGroup[0])

	g.By("applying a subset of TLS groups with ciphers and minTLSVersion VersionTLS12")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), append(append([]string{}, exutil.TLS12HTTP2Ciphers...),
		"ECDHE-ECDSA-AES256-GCM-SHA384",
		"ECDHE-RSA-AES256-GCM-SHA384",
	), subsetGroups))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS12),
		Groups:          subsetGroups,
		GroupsSpecified: true,
	})
	t.expectNegotiated(exutil.TLS12Flag, []string{string(configv1.TLSGroupSecP256r1)}, string(configv1.TLSGroupSecP256r1))
	t.expectNegotiated(exutil.TLS12Flag, []string{string(configv1.TLSGroupSecP384r1)}, string(configv1.TLSGroupSecP384r1))

	g.By("checking openssl s_client group order against the default TLS 1.2 groups")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), exutil.TLS12HTTP2Ciphers, defaults))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS12),
		Groups:          defaults,
		GroupsSpecified: true,
	})
	// X25519MLKEM768 is a TLS 1.3 group, so it is left out of the TLS 1.2 client offer.
	// TLS 1.2 selects the first client group the server still allows.
	t.expectNegotiated(exutil.TLS12Flag, exutil.TLS12OrderedGroupOffer(string(configv1.TLSGroupSecP384r1), string(configv1.TLSGroupSecP256r1), t.fips), string(configv1.TLSGroupSecP384r1))
	t.expectNegotiated(exutil.TLS12Flag, exutil.TLS12OrderedGroupOffer(string(configv1.TLSGroupSecP256r1), string(configv1.TLSGroupSecP384r1), t.fips), string(configv1.TLSGroupSecP256r1))

	g.By("applying VersionTLS12 without TLS groups and checking openssl s_client group order")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS12), exutil.TLS12HTTP2Ciphers, nil))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:    string(configv1.VersionTLS12),
		AllowUnsetMin: false,
	})
	t.expectNegotiated(exutil.TLS12Flag, exutil.TLS12OrderedGroupOffer(string(configv1.TLSGroupSecP384r1), string(configv1.TLSGroupSecP256r1), t.fips), string(configv1.TLSGroupSecP384r1))
	t.expectNegotiated(exutil.TLS12Flag, exutil.TLS12OrderedGroupOffer(string(configv1.TLSGroupSecP256r1), string(configv1.TLSGroupSecP384r1), t.fips), string(configv1.TLSGroupSecP256r1))
	t.expectNegotiated(exutil.TLS12Flag, []string{string(configv1.TLSGroupSecP521r1)}, string(configv1.TLSGroupSecP521r1))
}

func (t *tlsGroupsTest) runTLS13Cases() {
	singleGroup := []string{string(configv1.TLSGroupSecP384r1)}
	subsetGroups := []string{string(configv1.TLSGroupSecP256r1), string(configv1.TLSGroupSecP384r1)}
	defaults := exutil.DefaultTLSProfileGroups(t.fips)

	g.By("rejecting an invalid TLS group")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), nil, []string{exutil.InvalidTLSGroupName}), exutil.ExpectInvalidTLSGroupError)

	g.By("rejecting a mix of valid and invalid TLS groups")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), nil, []string{
		string(configv1.TLSGroupX25519),
		exutil.InvalidTLSGroupName,
	}), exutil.ExpectInvalidTLSGroupError)

	g.By("rejecting a TLS 1.3 profile that sets cipher suites")
	t.rejectProfile(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), []string{"TLS_AES_128_GCM_SHA256"}, singleGroup), exutil.ExpectTLS13CipherError)

	g.By("applying a single TLS group with minTLSVersion VersionTLS13")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), nil, singleGroup))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS13),
		Groups:          singleGroup,
		GroupsSpecified: true,
	})
	t.expectNegotiated(exutil.TLS13Flag, exutil.OtherTLSGroupsFirst(singleGroup[0], t.fips), singleGroup[0])
	t.expectHandshakeRejected(exutil.TLS12Flag)

	g.By("applying a subset of TLS groups with minTLSVersion VersionTLS13")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), nil, subsetGroups))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS13),
		Groups:          subsetGroups,
		GroupsSpecified: true,
	})
	t.expectNegotiated(exutil.TLS13Flag, []string{string(configv1.TLSGroupSecP256r1)}, string(configv1.TLSGroupSecP256r1))
	t.expectNegotiated(exutil.TLS13Flag, []string{string(configv1.TLSGroupSecP384r1)}, string(configv1.TLSGroupSecP384r1))

	g.By("checking the default TLS 1.3 profile groups")
	t.apply(exutil.NamedAPIServerTLSProfile(string(configv1.TLSProfileModernType)))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion:      string(configv1.VersionTLS13),
		Groups:          defaults,
		GroupsSpecified: true,
	})
	if t.fips {
		e2e.Logf("FIPS drops X25519MLKEM768; the negotiated group is one of the remaining default groups")
		exutil.ExpectTLSGroupOneOf(t.ctx, registryTLSEndpoint(), exutil.TLS13Flag, []string{
			string(configv1.TLSGroupSecP384r1),
			string(configv1.TLSGroupSecP256r1),
		}, defaults)
	} else {
		// Go's TLS 1.3 server prefers X25519MLKEM768 when the client offers it,
		// including when it is the last group in the openssl -groups list.
		// https://github.com/golang/go/blob/release-branch.go1.25/src/crypto/tls/handshake_server_tls13.go
		t.expectNegotiated(exutil.TLS13Flag, []string{
			string(configv1.TLSGroupSecP384r1),
			string(configv1.TLSGroupX25519),
			string(configv1.TLSGroupX25519MLKEM768),
		}, string(configv1.TLSGroupX25519MLKEM768))
	}

	g.By("applying VersionTLS13 without TLS groups and checking openssl s_client group order")
	t.apply(exutil.CustomAPIServerTLSProfile(string(configv1.VersionTLS13), nil, nil))
	t.waitForRegistry(exutil.TLSEnvExpectation{
		MinVersion: string(configv1.VersionTLS13),
	})
	t.expectNegotiated(exutil.TLS13Flag, []string{
		string(configv1.TLSGroupSecP384r1),
		string(configv1.TLSGroupSecP256r1),
		string(configv1.TLSGroupX25519),
	}, string(configv1.TLSGroupSecP384r1))
	t.expectNegotiated(exutil.TLS13Flag, []string{
		string(configv1.TLSGroupSecP256r1),
		string(configv1.TLSGroupSecP384r1),
		string(configv1.TLSGroupX25519),
	}, string(configv1.TLSGroupSecP256r1))

	g.By("checking a non-existent openssl group in the client group list")
	// ffdhe2048 is a real openssl group the registry does not select. openssl puts
	// the key share on that first group, which Go ignores, so selection falls back
	// to Go's default curve order. P-256 comes before P-384. X25519MLKEM768 still
	// wins when the client offers it, regardless of position.
	// https://github.com/golang/go/blob/release-branch.go1.25/src/crypto/tls/handshake_server_tls13.go
	t.expectNegotiated(exutil.TLS13Flag, []string{
		"ffdhe2048",
		string(configv1.TLSGroupSecP384r1),
		string(configv1.TLSGroupSecP256r1),
	}, string(configv1.TLSGroupSecP256r1))
	if !t.fips {
		t.expectNegotiated(exutil.TLS13Flag, []string{
			"ffdhe2048",
			string(configv1.TLSGroupSecP256r1),
			string(configv1.TLSGroupX25519MLKEM768),
		}, string(configv1.TLSGroupX25519MLKEM768))
	}
}

func (t *tlsGroupsTest) rejectProfile(profile exutil.APIServerTLSProfile, check func(error)) {
	check(exutil.PatchAPIServerTLSProfile(t.ctx, t.oc, profile))
}

func (t *tlsGroupsTest) apply(profile exutil.APIServerTLSProfile) {
	o.Expect(exutil.PatchAPIServerTLSProfile(t.ctx, t.oc, profile)).NotTo(o.HaveOccurred())
}

func (t *tlsGroupsTest) waitForRegistry(exp exutil.TLSEnvExpectation) {
	o.Expect(exutil.WaitForDeploymentTLSEnv(t.ctx, t.oc, registryTLSTarget(), exp, registryRolloutLimit)).NotTo(o.HaveOccurred())
}

func (t *tlsGroupsTest) expectCipherEnvDropsUnknown() {
	env, err := exutil.ReadDeploymentTLSEnv(t.ctx, t.oc, registryTLSTarget())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(env.Ciphers).NotTo(o.ContainSubstring("NOT-A-CIPHER-SUITE"))
	o.Expect(env.CiphersFound).To(o.BeTrue())
	upper := strings.ToUpper(env.Ciphers)
	o.Expect(upper).To(o.Or(
		o.ContainSubstring("AES_128_GCM"),
		o.ContainSubstring("AES128-GCM"),
	))
}

func (t *tlsGroupsTest) expectNegotiated(tlsFlag string, clientGroups []string, want string) {
	exutil.ExpectNegotiatedTLSGroup(t.ctx, registryTLSEndpoint(), tlsFlag, clientGroups, want)
}

func (t *tlsGroupsTest) expectHandshakeRejected(tlsFlag string) {
	exutil.ExpectTLSVersionRejected(t.ctx, registryTLSEndpoint(), tlsFlag)
}
