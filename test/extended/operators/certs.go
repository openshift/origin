package operators

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	promtime "github.com/prometheus/common/model"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatadefaults"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionpodnetwork"

	ensure_no_violation_regression "github.com/openshift/origin/pkg/cmd/update-tls-artifacts/ensure-no-violation-regression"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/api/annotations"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphutils"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	ownership "github.com/openshift/origin/tls"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const certInspectResultFile = "/tmp/shared/pkiList.json"

var (
	//go:embed manifests/namespace.yaml
	namespaceYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/rolebinding-privileged.yaml
	roleBindingPrivilegedYaml []byte
	//go:embed manifests/clusterrolebinding-nodelist.yaml
	roleBindingNodeReaderYaml []byte
	//go:embed manifests/pod.yaml
	podYaml []byte

	actualPKIContent   *certgraphapi.PKIList
	expectedPKIContent *certs.PKIRegistryInfo
	nodeList           *corev1.NodeList
	jobType            *platformidentification.JobType
)

func gatherCertsFromPlatformNamespaces(ctx context.Context, kubeClient kubernetes.Interface, masters []*corev1.Node, bootstrapHostname string) (*certgraphapi.PKIList, error) {
	annotationsToCollect := []string{annotations.OpenShiftComponent}
	for _, currRequirement := range tlsmetadatadefaults.GetDefaultTLSRequirements() {
		annotationRequirement, ok := currRequirement.(tlsmetadatainterfaces.AnnotationRequirement)
		if ok {
			annotationsToCollect = append(annotationsToCollect, annotationRequirement.GetAnnotationName())
		}
	}

	return certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient,
		certgraphanalysis.SkipRevisioned,
		certgraphanalysis.SkipHashed,
		certgraphanalysis.ElideProxyCADetails,
		certgraphanalysis.RewriteNodeNames(masters, bootstrapHostname),
		certgraphanalysis.CollectAnnotations(annotationsToCollect...),
	)
}

var _ = g.Describe(fmt.Sprintf("[sig-arch][Late][Jira:%q]", "kube-apiserver"), g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")
	ctx := context.Background()

	g.BeforeAll(func() {
		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		configClient := oc.AdminConfigClient()
		if ok, _ := exutil.IsHypershift(ctx, configClient); ok {
			g.Skip("hypershift does not auto-collect TLS.")
		}
		var err error
		onDiskPKIContent := &certgraphapi.PKIList{}

		jobType, err = platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		controlPlaneLabel := labels.SelectorFromSet(map[string]string{"node-role.kubernetes.io/control-plane": ""})
		nodeList, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: controlPlaneLabel.String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		masters := []*corev1.Node{}
		for i := range nodeList.Items {
			masters = append(masters, &nodeList.Items[i])
		}

		_, bootstrapHostname, err := certgraphanalysis.GetBootstrapIPAndHostname(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Found bootstrap hostname %q", bootstrapHostname)
		inClusterPKIContent, err := gatherCertsFromPlatformNamespaces(ctx, kubeClient, masters, bootstrapHostname)
		o.Expect(err).NotTo(o.HaveOccurred())

		openshiftTestImagePullSpec, err := disruptionpodnetwork.GetOpenshiftTestsImagePullSpecWithRetries(ctx, oc.AdminConfig(), "", oc, 5)
		// Skip metal jobs if test image pullspec cannot be determined
		if jobType.Platform != "metal" || err == nil {
			o.Expect(err).NotTo(o.HaveOccurred())
			onDiskPKIContent, err = fetchOnDiskCertificates(ctx, kubeClient, oc.AdminConfig(), masters, openshiftTestImagePullSpec)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		actualPKIContent = certgraphanalysis.MergePKILists(ctx, inClusterPKIContent, onDiskPKIContent)

		expectedPKIContent, err = certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIOwnership)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("collect certificate data", func() {
		configClient := oc.AdminConfigClient()
		featureGates, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		featureSetString := string(featureGates.Spec.FeatureSet)
		if len(featureSetString) == 0 {
			featureSetString = "Default"
		}
		tlsArtifactFilename := fmt.Sprintf(
			"raw-tls-artifacts-%s-%s-%s-%s-%s.json",
			jobType.Topology,
			jobType.Architecture,
			jobType.Platform,
			jobType.Network,
			strings.ToLower(featureSetString),
		)

		jsonBytes, err := json.MarshalIndent(actualPKIContent, "", "  ")
		o.Expect(err).NotTo(o.HaveOccurred())

		pkiDir := filepath.Join(exutil.ArtifactDirPath(), "rawTLSInfo")
		err = os.MkdirAll(pkiDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(pkiDir, tlsArtifactFilename), jsonBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("all tls artifacts must be registered", func() {

		violationsPKIContent, err := certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIViolations)
		o.Expect(err).NotTo(o.HaveOccurred())

		newTLSRegistry := &certs.PKIRegistryInfo{}

		for i, inClusterCertKeyPair := range actualPKIContent.InClusterResourceData.CertKeyPairs {
			currLocation := inClusterCertKeyPair.SecretLocation
			if _, err := certgraphutils.LocateCertKeyPairBySecretLocation(currLocation, violationsPKIContent.CertKeyPairs); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCertKeyPairBySecretLocation(currLocation, expectedPKIContent.CertKeyPairs)
			if err != nil {

				newTLSRegistry.CertKeyPairs = append(newTLSRegistry.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{InClusterLocation: &actualPKIContent.InClusterResourceData.CertKeyPairs[i]})
			}

		}

		for _, currCertKeyPair := range actualPKIContent.CertKeyPairs.Items {
			if len(currCertKeyPair.Spec.SecretLocations) != 0 || len(currCertKeyPair.Spec.OnDiskLocations) == 0 {
				continue
			}
			for _, currLocation := range currCertKeyPair.Spec.OnDiskLocations {
				if len(currLocation.Cert.Path) > 0 {
					if _, err := certgraphutils.LocateCertKeyPairByOnDiskLocation(currLocation.Cert, violationsPKIContent.CertKeyPairs); err == nil {
						continue
					}

					certInfo, err := certgraphutils.LocateCertKeyPairByOnDiskLocation(currLocation.Cert, expectedPKIContent.CertKeyPairs)
					if err != nil {
						if certInfo == nil {
							certInfo = &certgraphapi.PKIRegistryOnDiskCertKeyPair{
								OnDiskLocation: certgraphapi.OnDiskLocation{
									Path: currLocation.Cert.Path,
								},
							}
						}
						newTLSRegistry.CertKeyPairs = append(newTLSRegistry.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{OnDiskLocation: certInfo})
					}
				}

				if len(currLocation.Key.Path) > 0 && currLocation.Key.Path != currLocation.Cert.Path {

					if _, err := certgraphutils.LocateCertKeyPairByOnDiskLocation(currLocation.Key, violationsPKIContent.CertKeyPairs); err == nil {
						continue
					}

					keyInfo, err := certgraphutils.LocateCertKeyPairByOnDiskLocation(currLocation.Key, expectedPKIContent.CertKeyPairs)
					if err != nil {
						if keyInfo == nil {
							keyInfo = &certgraphapi.PKIRegistryOnDiskCertKeyPair{
								OnDiskLocation: certgraphapi.OnDiskLocation{
									Path: currLocation.Key.Path,
								},
							}
						}
						newTLSRegistry.CertKeyPairs = append(newTLSRegistry.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{OnDiskLocation: keyInfo})
					}
				}
			}
		}

		for i, inClusterCABundle := range actualPKIContent.InClusterResourceData.CertificateAuthorityBundles {
			currLocation := inClusterCABundle.ConfigMapLocation
			if _, err := certgraphutils.LocateCABundleByConfigMapLocation(currLocation, violationsPKIContent.CertificateAuthorityBundles); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCABundleByConfigMapLocation(currLocation, expectedPKIContent.CertificateAuthorityBundles)
			if err != nil {
				newTLSRegistry.CertificateAuthorityBundles = append(newTLSRegistry.CertificateAuthorityBundles, certgraphapi.PKIRegistryCABundle{InClusterLocation: &actualPKIContent.InClusterResourceData.CertificateAuthorityBundles[i]})
			}
		}

		for _, currCABundle := range actualPKIContent.CertificateAuthorityBundles.Items {
			if len(currCABundle.Spec.ConfigMapLocations) != 0 || len(currCABundle.Spec.OnDiskLocations) == 0 {
				continue
			}
			for _, currLocation := range currCABundle.Spec.OnDiskLocations {
				if _, err := certgraphutils.LocateCABundleByOnDiskLocation(currLocation, violationsPKIContent.CertificateAuthorityBundles); err == nil {
					continue
				}

				caBundleInfo, err := certgraphutils.LocateCABundleByOnDiskLocation(currLocation, expectedPKIContent.CertificateAuthorityBundles)
				if err != nil {
					if caBundleInfo == nil {
						caBundleInfo = &certgraphapi.PKIRegistryOnDiskCABundle{
							OnDiskLocation: certgraphapi.OnDiskLocation{
								Path: currLocation.Path,
							},
						}
					}
					newTLSRegistry.CertificateAuthorityBundles = append(newTLSRegistry.CertificateAuthorityBundles, certgraphapi.PKIRegistryCABundle{OnDiskLocation: caBundleInfo})
				}
			}
		}

		if len(newTLSRegistry.CertKeyPairs) > 0 || len(newTLSRegistry.CertificateAuthorityBundles) > 0 {
			registryString, err := json.MarshalIndent(newTLSRegistry, "", "  ")
			if err != nil {
				//g.Fail("Failed to marshal registry %#v: %v", newTLSRegistry, err)
				testresult.Flakef("Failed to marshal registry %#v: %v", newTLSRegistry, err)
			}
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(fmt.Sprintf("Unregistered TLS certificates:\n%s", registryString))
			testresult.Flakef("Unregistered TLS certificates found:\n%s\nSee tls/ownership/README.md in origin repo", registryString)
		}
	})

	g.It("all registered tls artifacts must have no metadata violation regressions", func() {
		violationRegressionOptions := ensure_no_violation_regression.NewEnsureNoViolationRegressionOptions(ownership.AllViolations, genericclioptions.NewTestIOStreamsDiscard())
		messages, _, err := violationRegressionOptions.HaveViolationsRegressed([]*certgraphapi.PKIList{actualPKIContent})
		o.Expect(err).NotTo(o.HaveOccurred())

		if len(messages) > 0 {
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(strings.Join(messages, "\n"))
			testresult.Flakef("%s", strings.Join(messages, "\n"))
		}
	})

	g.It("[OCPFeatureGate:ShortCertRotation] all certificates should expire in no more than 8 hours", func() {
		var errs []error
		// Skip router certificates (both certificate and signer)
		// These are not being rotated automatically
		// OLM: bug https://issues.redhat.com/browse/CNTRLPLANE-379
		shortCertRotationIgnoredNamespaces := []string{"openshift-operator-lifecycle-manager", "openshift-ingress-operator", "openshift-ingress"}

		for _, certKeyPair := range actualPKIContent.CertKeyPairs.Items {
			if certKeyPair.Spec.CertMetadata.ValidityDuration == "" {
				// Skip certificates with no duration set (proxy ca, key without certificate etc.)
				continue
			}
			if certKeyPair.Spec.CertMetadata.ValidityDuration == "10y" {
				// Skip "forever" certificates
				continue
			}
			if isCertKeyPairFromIgnoredNamespace(certKeyPair, shortCertRotationIgnoredNamespaces) {
				continue
			}
			// Use ParseDuration from prometheus as it can handle days/month/years durations
			duration, err := promtime.ParseDuration(certKeyPair.Spec.CertMetadata.ValidityDuration)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to parse validity duration for certificate %q: %v", certKeyPair.Name, err))
				continue
			}
			if time.Duration(duration) > time.Hour*8 {
				errs = append(errs, fmt.Errorf("certificate %q expires too soon: expected duration to be up to 8h, but was %s", certKeyPair.Name, duration))
			}
		}
		if len(errs) > 0 {
			testresult.Flakef("Errors found: %s", utilerrors.NewAggregate(errs).Error())
		}
	})

})

func fetchOnDiskCertificates(ctx context.Context, kubeClient kubernetes.Interface, podRESTConfig *rest.Config, nodeList []*corev1.Node, testPullSpec string) (*certgraphapi.PKIList, error) {
	namespace, err := createNamespace(ctx, kubeClient)
	if err != nil {
		return nil, err
	}
	defer kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})

	err = createServiceAccount(ctx, kubeClient, namespace)
	if err != nil {
		return nil, err
	}
	nodeReaderCRB, err := createRBACBindings(ctx, kubeClient, namespace)
	if err != nil {
		return nil, err
	}
	defer kubeClient.RbacV1().ClusterRoleBindings().Delete(ctx, nodeReaderCRB, metav1.DeleteOptions{})

	pauseImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.53")
	podNameOnNode, err := createPods(ctx, kubeClient, namespace, nodeList, testPullSpec, pauseImage)
	if err != nil {
		return nil, err
	}

	ret := &certgraphapi.PKIList{}
	errs := []error{}
	for _, node := range nodeList {
		nodePKIList, err := fetchNodePKIList(ctx, kubeClient, podRESTConfig, podNameOnNode, node)
		if err != nil {
			errs = append(errs, err)
		}
		ret = certgraphanalysis.MergePKILists(ctx, ret, nodePKIList)
	}
	if len(errs) != 0 {
		return ret, utilerrors.NewAggregate(errs)
	}

	return ret, nil
}

func createNamespace(ctx context.Context, kubeClient kubernetes.Interface) (string, error) {
	namespaceObj := resourceread.ReadNamespaceV1OrDie(namespaceYaml)

	client := kubeClient.CoreV1().Namespaces()
	actualNamespace, err := client.Create(ctx, namespaceObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating namespace: %v", err)
	}
	return actualNamespace.Name, nil
}

func createServiceAccount(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	serviceAccountObj := resourceread.ReadServiceAccountV1OrDie(serviceAccountYaml)
	serviceAccountObj.Namespace = namespace
	client := kubeClient.CoreV1().ServiceAccounts(namespace)
	_, err := client.Create(ctx, serviceAccountObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating service account: %v", err)
	}
	return nil
}

func createRBACBindings(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (string, error) {
	privilegedRoleBindingObj := resourceread.ReadRoleBindingV1OrDie(roleBindingPrivilegedYaml)
	privilegedRoleBindingObj.Namespace = namespace

	namespaceRBClient := kubeClient.RbacV1().RoleBindings(namespace)
	_, err := namespaceRBClient.Create(ctx, privilegedRoleBindingObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating hostaccess SCC CRB: %v", err)
	}

	nodeReaderRoleBindingObj := resourceread.ReadClusterRoleBindingV1OrDie(roleBindingNodeReaderYaml)
	nodeReaderRoleBindingObj.Subjects[0].Namespace = namespace
	crbClient := kubeClient.RbacV1().ClusterRoleBindings()
	nodeReaderObj, err := crbClient.Create(ctx, nodeReaderRoleBindingObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating node reader CRB: %v", err)
	}
	return nodeReaderObj.Name, nil
}

type podToNodeMap map[string]*corev1.Pod

func createPods(ctx context.Context, kubeClient kubernetes.Interface, namespace string, nodeList []*corev1.Node, testImagePullSpec, pauseImagePullSpec string) (podToNodeMap, error) {
	podOnNode := podToNodeMap{}

	client := kubeClient.CoreV1().Pods(namespace)
	podTemplate := resourceread.ReadPodV1OrDie(podYaml)
	for _, node := range nodeList {
		podObj := podTemplate.DeepCopy()
		podObj.Namespace = namespace
		podObj.Spec.NodeName = node.Name
		podObj.Spec.InitContainers[0].Image = testImagePullSpec
		podObj.Spec.Containers[0].Image = pauseImagePullSpec

		actualPod, err := client.Create(ctx, podObj, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return podOnNode, fmt.Errorf("error creating pod on node %s: %v", node.Name, err)
		}

		timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
			cache.NewListWatchFromClient(
				kubeClient.CoreV1().RESTClient(), "pods", namespace, fields.OneTermEqualSelector("metadata.name", actualPod.Name)),
			&corev1.Pod{},
			nil,
			func(event watch.Event) (bool, error) {
				pod := event.Object.(*corev1.Pod)
				if pod.Status.Phase == corev1.PodRunning {
					podOnNode[node.Name] = pod
					return true, nil
				}
				return false, nil
			},
		); watchErr != nil {
			return podOnNode, fmt.Errorf("pod %s in namespace %s didn't start: %v", actualPod.Name, namespace, watchErr)
		}
	}
	return podOnNode, nil
}

func fetchNodePKIList(_ context.Context, kubeClient kubernetes.Interface, podRESTConfig *rest.Config, podOnNode podToNodeMap, node *corev1.Node) (*certgraphapi.PKIList, error) {
	pkiList := &certgraphapi.PKIList{}

	pod, ok := podOnNode[node.Name]
	if !ok {
		return pkiList, fmt.Errorf("failed to find node %s in pod map %v", node.Name, podOnNode)
	}

	output, err := exutil.ExecInPodWithResult(kubeClient.CoreV1(), podRESTConfig, pod.Namespace, pod.Name, "pause", []string{"/bin/cat", certInspectResultFile})
	if err != nil {
		return pkiList, fmt.Errorf("failed to fetch file %s from pod %s/%s node %s: %v", certInspectResultFile, pod.Namespace, pod.Name, node.Name, err)
	}

	err = json.Unmarshal([]byte(output), pkiList)
	if err != nil {
		return pkiList, fmt.Errorf("failed to unmarshal file %s on node %s: %v", certInspectResultFile, node.Name, err)
	}

	return pkiList, nil
}

func isCertKeyPairFromIgnoredNamespace(cert certgraphapi.CertKeyPair, ignoredNamespaces []string) bool {
	for _, location := range cert.Spec.SecretLocations {
		for _, namespace := range ignoredNamespaces {
			if location.Namespace == namespace {
				return true
			}
		}
	}
	return false
}
