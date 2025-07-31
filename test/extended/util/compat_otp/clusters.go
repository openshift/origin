package compat_otp

import (
	"context"
	"fmt"
	"os"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/tidwall/gjson"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	AKSNodeLabel = "kubernetes.azure.com/cluster"
)

// Extract pull secrect from cluster
func GetPullSec(oc *exutil.CLI, dirname string) (err error) {
	if err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--to="+dirname, "--confirm").Execute(); err != nil {
		return fmt.Errorf("extract pull-secret failed: %v", err)
	}
	return
}

// GetMirrorRegistry returns mirror registry from icsp
func GetMirrorRegistry(oc *exutil.CLI) (registry string, err error) {
	if registry, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageContentSourcePolicy",
		"-o", "jsonpath={.items[0].spec.repositoryDigestMirrors[0].mirrors[0]}").Output(); err == nil {
		registry, _, _ = strings.Cut(registry, "/")
	} else {
		err = fmt.Errorf("failed to acquire mirror registry from ICSP: %v", err)
	}
	return
}

// GetUserCA dump user certificate from user-ca-bundle configmap to File
func GetUserCAToFile(oc *exutil.CLI, filename string) (err error) {
	cert, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "-n", "openshift-config",
		"user-ca-bundle", "-o", "jsonpath={.data.ca-bundle\\.crt}").Output()
	if err != nil {
		return fmt.Errorf("failed to acquire user ca bundle from configmap: %v", err)
	} else {
		err = os.WriteFile(filename, []byte(cert), 0644)
		if err != nil {
			return fmt.Errorf("failed to dump cert to file: %v", err)
		}
		return
	}
}

// GetClusterVersion returns the cluster version as string value (Ex: 4.8) and cluster build (Ex: 4.8.0-0.nightly-2021-09-28-165247)
func GetClusterVersion(oc *exutil.CLI) (string, string, error) {
	clusterBuild, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.version}").Output()
	if err != nil {
		return "", "", err
	}
	splitValues := strings.Split(clusterBuild, ".")
	clusterVersion := splitValues[0] + "." + splitValues[1]
	return clusterVersion, clusterBuild, err
}

// GetReleaseImage returns the release image as string value (Ex: registry.ci.openshift.org/ocp/release@sha256:b13971e61312f5dddd6435ccf061ac1a8447285a85828456edcd4fc2504cfb8f)
func GetReleaseImage(oc *exutil.CLI) (string, error) {
	releaseImage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.image}").Output()
	if err != nil {
		return "", err
	}
	return releaseImage, nil
}

// GetInfraID returns the infra id
func GetInfraID(oc *exutil.CLI) (string, error) {
	infraID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o", "jsonpath='{.status.infrastructureName}'").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(infraID, "'"), err
}

// GetGcpProjectID returns the gcp project id
func GetGcpProjectID(oc *exutil.CLI) (string, error) {
	projectID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o", "jsonpath='{.status.platformStatus.gcp.projectID}'").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(projectID, "'"), err
}

// GetClusterPrefixName return Cluster Prefix Name
func GetClusterPrefixName(oc *exutil.CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("route", "console", "-n", "openshift-console", "-o=jsonpath={.spec.host}").Output()
	if err != nil {
		e2e.Logf("Get cluster console route failed with err %v .", err)
		return ""
	}
	return strings.Split(output, ".")[2]
}

// SkipBaselineCaps skip the test if cluster has no required resources.
// sets is comma separated list of baselineCapabilitySets to skip.
// for example: "None, v4.11"
func SkipBaselineCaps(oc *exutil.CLI, sets string) {
	baselineCapabilitySet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "version", "-o=jsonpath={.spec.capabilities.baselineCapabilitySet}").Output()
	if err != nil {
		e2e.Failf("get baselineCapabilitySet failed err %v .", err)
	}
	sets = strings.ReplaceAll(sets, " ", "")
	for _, s := range strings.Split(sets, ",") {
		if strings.Contains(baselineCapabilitySet, s) {
			g.Skip("Skip for cluster with baselineCapabilitySet = '" + baselineCapabilitySet + "' matching filter: " + s)
		}
	}
}

// SkipNoCapabilities skip the test if the cluster has no one capability
func SkipNoCapabilities(oc *exutil.CLI, capability string) {
	clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	hasCapability := func(capabilities []configv1.ClusterVersionCapability, checked string) bool {
		cap := configv1.ClusterVersionCapability(checked)
		for _, capability := range capabilities {
			if capability == cap {
				return true
			}
		}
		return false
	}
	if clusterVersion.Status.Capabilities.KnownCapabilities != nil &&
		hasCapability(clusterVersion.Status.Capabilities.KnownCapabilities, capability) &&
		(clusterVersion.Status.Capabilities.EnabledCapabilities == nil ||
			!hasCapability(clusterVersion.Status.Capabilities.EnabledCapabilities, capability)) {
		g.Skip(fmt.Sprintf("the cluster has no %v and skip it", capability))
	}
}

// SkipIfCapEnabled skips the test if a capability is enabled
func SkipIfCapEnabled(oc *exutil.CLI, capability string) {
	clusterversion, err := oc.
		AdminConfigClient().
		ConfigV1().
		ClusterVersions().
		Get(context.Background(), "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	var capKnown bool
	for _, knownCap := range clusterversion.Status.Capabilities.KnownCapabilities {
		if capability == string(knownCap) {
			capKnown = true
			break
		}
	}
	if !capKnown {
		g.Skip(fmt.Sprintf("Will skip as capability %s is unknown (i.e. cannot be disabled in the first place)", capability))
	}
	for _, enabledCap := range clusterversion.Status.Capabilities.EnabledCapabilities {
		if capability == string(enabledCap) {
			g.Skip(fmt.Sprintf("Will skip as capability %s is enabled", capability))
		}
	}
}

// SkipNoOLMCore skip the test if the cluster has no OLM component
// from 4.15, OLM become optional core component. it means there is no OLM component for some profiles.
// so, the OLM case and optioinal operator case can not run on such cluster.
func SkipNoOLMCore(oc *exutil.CLI) {
	SkipNoCapabilities(oc, "OperatorLifecycleManager")
}

func SkipNoOLMv1Core(oc *exutil.CLI) {
	SkipNoCapabilities(oc, "OperatorLifecycleManagerV1")
}

// SkipNoBuild skip the test if the cluster has no Build component
func SkipNoBuild(oc *exutil.CLI) {
	SkipNoCapabilities(oc, "Build")
}

// SkipNoDeploymentConfig skip the test if the cluster has no DeploymentConfig component
func SkipNoDeploymentConfig(oc *exutil.CLI) {
	SkipNoCapabilities(oc, "DeploymentConfig")
}

// SkipNoImageRegistry skip the test if the cluster has no ImageRegistry component
func SkipNoImageRegistry(oc *exutil.CLI) {
	SkipNoCapabilities(oc, "ImageRegistry")
}

// IsTechPreviewNoUpgrade checks if a cluster is a TechPreviewNoUpgrade cluster
func IsTechPreviewNoUpgrade(oc *exutil.CLI) bool {
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "could not retrieve feature-gate: %v", err)
	}

	return featureGate.Spec.FeatureSet == configv1.TechPreviewNoUpgrade
}

// GetAWSClusterRegion returns AWS region of the cluster
func GetAWSClusterRegion(oc *exutil.CLI) (string, error) {
	region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	return region, err
}

// SkipNoDefaultSC skip the test if cluster has no default storageclass or has more than 1 default storageclass
func SkipNoDefaultSC(oc *exutil.CLI) {
	allSCRes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sc", "-o", "json").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	defaultSCRes := gjson.Get(allSCRes, "items.#(metadata.annotations.storageclass\\.kubernetes\\.io\\/is-default-class=true)#.metadata.name")
	e2e.Logf("The default storageclass list: %s", defaultSCRes)
	defaultSCNub := len(defaultSCRes.Array())
	if defaultSCNub != 1 {
		e2e.Logf("oc get sc:\n%s", allSCRes)
		g.Skip("Skip for unexpected default storageclass!")
	}
}

// SkipIfPlatformTypeNot skips all platforms other than supported
// platforms is comma separated list of allowed platforms
// for example: "gcp, aws"
func SkipIfPlatformTypeNot(oc *exutil.CLI, platforms string) {
	platformType, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	if err != nil {
		e2e.Failf("get infrastructure platformStatus type failed err %v .", err)
	}
	if !strings.Contains(strings.ToLower(platforms), strings.ToLower(platformType)) {
		g.Skip("Skip for non-" + platforms + " cluster: " + platformType)
	}
}

// skip platform
func SkipIfPlatformType(oc *exutil.CLI, platforms string) {
	platformType, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(strings.ToLower(platforms), strings.ToLower(platformType)) {
		g.Skip("Skip for " + platforms + " cluster: " + platformType)
	}
}

// IsHypershiftHostedCluster
func IsHypershiftHostedCluster(oc *exutil.CLI) bool {
	topology, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructures.config.openshift.io", "cluster", "-o=jsonpath={.status.controlPlaneTopology}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("topology is %s", topology)
	if topology == "" {
		status, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructures.config.openshift.io", "cluster", "-o=jsonpath={.status}").Output()
		e2e.Logf("cluster status %s", status)
		e2e.Failf("failure: controlPlaneTopology returned empty")
	}
	return strings.Compare(topology, "External") == 0
}

// IsRosaCluster
func IsRosaCluster(oc *exutil.CLI) bool {
	product, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("clusterclaims/product.open-cluster-management.io", "-o=jsonpath={.spec.value}").Output()
	return strings.Compare(product, "ROSA") == 0
}

// IsSTSCluster determines if an AWS cluster is using STS
func IsSTSCluster(oc *exutil.CLI) bool {
	return IsWorkloadIdentityCluster(oc)
}

// IsWorkloadIdentityCluster judges whether the Azure/GCP cluster is using the Workload Identity
func IsWorkloadIdentityCluster(oc *exutil.CLI) bool {
	serviceAccountIssuer, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("authentication", "cluster", "-o=jsonpath={.spec.serviceAccountIssuer}").Output()
	o.Expect(err).ShouldNot(o.HaveOccurred(), "Failed to get serviceAccountIssuer")
	return len(serviceAccountIssuer) > 0
}

// GetOIDCProvider returns the OIDC provider for current cluster
func GetOIDCProvider(oc *exutil.CLI) (string, error) {
	oidc, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("authentication.config", "cluster", "-o=jsonpath={.spec.serviceAccountIssuer}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(oidc, "https://"), nil
}

// Skip the test if there is not catalogsource/qe-app-registry in the cluster
func SkipMissingQECatalogsource(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", "qe-app-registry").Output()
	if strings.Contains(output, "NotFound") || err != nil {
		g.Skip("Skip the test since no catalogsource/qe-app-registry in the cluster")
	}
}

// Skip the test if default catsrc is disable
func SkipIfDisableDefaultCatalogsource(oc *exutil.CLI) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("operatorhubs", "cluster", "-o=jsonpath={.spec.disableAllDefaultSources}").Output()
	if output == "true" || err != nil {
		g.Skip("Skip the test, the default catsrc is disable or don't have operatorhub resource")
	}
}

// IsInfrastructuresHighlyAvailable check if it is HighlyAvailable for infrastructures. Available for both classic OCP and the hosted cluster.
func IsInfrastructuresHighlyAvailable(oc *exutil.CLI) bool {
	topology, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructures.config.openshift.io", "cluster", `-o=jsonpath={.status.infrastructureTopology}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("infrastructures topology is %s", topology)
	if topology == "" {
		status, _ := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructures.config.openshift.io", "cluster", "-o=jsonpath={.status}").Output()
		e2e.Logf("cluster status %s", status)
		e2e.Failf("failure: controlPlaneTopology returned empty")
	}
	return strings.Compare(topology, "HighlyAvailable") == 0
}

// IsExternalOIDCCluster checks if the cluster is using external OIDC.
func IsExternalOIDCCluster(oc *exutil.CLI) (bool, error) {
	switch IsExternalOIDCClusterFlag {
	case "yes":
		e2e.Logf("it is external oidc cluster")
		return true, nil
	case "no":
		e2e.Logf("it is not external oidc cluster")
		return false, nil
	default:
		e2e.Logf("do not know if it is external oidc cluster or not, and try to check it again")
		authType, stdErr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("authentication/cluster", "-o=jsonpath={.spec.type}").Outputs()
		if err != nil {
			return false, fmt.Errorf("error checking if the cluster is using external OIDC: %v", stdErr)
		}
		e2e.Logf("Found authentication type used: %v", authType)
		return authType == string(configv1.AuthenticationTypeOIDC), nil
	}
}

// IsKeycloakExtOIDCCluster assumes the cluster uses external oidc auth but checks if the oidc issuer is Keycloak.
func IsKeycloakExtOIDCCluster() bool {
	if os.Getenv("KEYCLOAK_ISSUER") != "" && os.Getenv("KEYCLOAK_TEST_USERS") != "" && os.Getenv("KEYCLOAK_CLI_CLIENT_ID") != "" {
		return true
	}
	return false
}

// IsOpenShiftCluster checks if the active cluster is OpenShift or a derivative
func IsOpenShiftCluster(ctx context.Context, c corev1client.NamespaceInterface) (bool, error) {
	switch _, err := c.Get(ctx, "openshift-controller-manager", metav1.GetOptions{}); {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, fmt.Errorf("unable to determine if we are running against an OpenShift cluster: %v", err)
	}
}

// SkipOnOpenShiftNess skips the test if the cluster type doesn't match the expected type.
func SkipOnOpenShiftNess(expectOpenShift bool) {
	switch IsKubernetesClusterFlag {
	case "yes":
		if expectOpenShift {
			g.Skip("Expecting OpenShift but the active cluster is not, skipping the test")
		}
	// Treat both "no" and "unknown" as OpenShift
	default:
		if !expectOpenShift {
			g.Skip("Expecting non-OpenShift but the active cluster is OpenShift, skipping the test")
		}
	}
}

// IsAKSCluster checks if the active cluster is an AKS (Azure Kubernetes Service) cluster or not
func IsAKSCluster(ctx context.Context, oc *exutil.CLI) (bool, error) {
	nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}
	_, labelFound := nodeList.Items[0].Labels[AKSNodeLabel]
	return labelFound, nil
}

func CheckAKSCluster(ctx context.Context, oc *exutil.CLI) bool {
	isAKS, err := IsAKSCluster(ctx, oc)
	if err != nil {
		e2e.Logf("failed to determine if the active cluster is AKS or not: %v, defaulting to non-AKS", err)
		return false
	}
	return isAKS
}

func SkipOnAKSNess(ctx context.Context, oc *exutil.CLI, expectAKS bool) {
	isAKS := CheckAKSCluster(ctx, oc)
	if isAKS && !expectAKS {
		g.Skip("Expecting non-AKS but the active cluster is AKS, skip the test")
	}
	if !isAKS && expectAKS {
		g.Skip("Expecting AKS but the active cluster is not, skip the test")
	}
}

// Skip for proxy platform
func SkipOnProxyCluster(oc *exutil.CLI) {
	g.By("Check if cluster is a proxy platform")
	httpProxy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("proxy/cluster", "-o=jsonpath={.spec.httpProxy}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	httpsProxy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("proxy/cluster", "-o=jsonpath={.spec.httpsProxy}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(httpProxy) != 0 || len(httpsProxy) != 0 {
		g.Skip("Skip for proxy platform")
	}
}

// A two-node-arbiter OpenShift cluster is a special type of high availability (HA) deployment designed to run in edge environments
func IsArbiterCluster(oc *exutil.CLI) bool {
	masterNodes, _ := GetClusterNodesBy(oc, "master")
	workerNodes, _ := GetClusterNodesBy(oc, "worker")

	if len(masterNodes) != 2 || len(workerNodes) != 2 {
		return false
	}

	// Check if master and worker nodes are the same set
	masterSet := map[string]struct{}{masterNodes[0]: {}, masterNodes[1]: {}}
	if _, ok1 := masterSet[workerNodes[0]]; !ok1 {
		return false
	}
	if _, ok2 := masterSet[workerNodes[1]]; !ok2 {
		return false
	}

	// Check control plane topology
	output, err := oc.Run("get").Args("infrastructure/cluster", "-o", "jsonpath={.status.controlPlaneTopology}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return strings.TrimSpace(output) == "HighlyAvailableArbiter"
}
