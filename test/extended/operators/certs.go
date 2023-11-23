package operators

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphutils"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/monitortestlibrary/nodeaccess"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	ownership "github.com/openshift/origin/tls"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

const certInspectResultFile = "pkiList.json"

var (
	//go:embed manifests/namespace.yaml
	namespaceYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/rolebinding-privileged.yaml
	roleBindingPrivilegedYaml []byte
	//go:embed manifests/pod.yaml
	podYaml []byte
)

var _ = g.Describe("[sig-arch][Late]", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")
	ctx := context.Background()
	var (
		kubeClient         kubernetes.Interface
		actualPKIContent   *certgraphapi.PKIList
		expectedPKIContent *certgraphapi.PKIRegistryInfo
	)

	g.BeforeAll(func() {
		kubeClient = oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		controlPlaneLabel := labels.SelectorFromSet(map[string]string{"node-role.kubernetes.io/control-plane": ""})
		nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: controlPlaneLabel.String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		masters := []*corev1.Node{}
		for i := range nodeList.Items {
			masters = append(masters, &nodeList.Items[i])
		}

		inClusterPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient,
			certgraphanalysis.ElideProxyCADetails,
			certgraphanalysis.SkipRevisioned,
			certgraphanalysis.SkipHashed,
			certgraphanalysis.RewriteNodeIPs(masters))
		o.Expect(err).NotTo(o.HaveOccurred())

		// openshiftTestImagePullSpec, err := getOpenshiftTestsImagePullSpec(ctx, oc.AdminConfig())
		// o.Expect(err).NotTo(o.HaveOccurred())
		openshiftTestImagePullSpec := "quay.io/vrutkovs/ocp:tests-34111656de"

		originalOnDiskPKIContent, err := fetchOnDiskCertificates(ctx, kubeClient, nodeList, openshiftTestImagePullSpec)
		o.Expect(err).NotTo(o.HaveOccurred())

		onDiskPKIContent, err := processOnNodePKIList(originalOnDiskPKIContent)
		o.Expect(err).NotTo(o.HaveOccurred())
		actualPKIContent = certgraphanalysis.MergePKILists(ctx, inClusterPKIContent, onDiskPKIContent)

		expectedPKIContent, err = certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIOwnership)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("collect certificate data", func() {
		jobType, err := platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		tlsArtifactFilename := fmt.Sprintf("raw-tls-artifacts-%s-%s-%s-%s.json", jobType.Topology, jobType.Architecture, jobType.Platform, jobType.Network)

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

		newTLSRegistry := &certgraphapi.PKIRegistryInfo{}

		for _, currCertKeyPair := range actualPKIContent.InClusterResourceData.CertKeyPairs {
			currLocation := currCertKeyPair.SecretLocation
			if _, err := certgraphutils.LocateCertKeyPair(currLocation, violationsPKIContent.CertKeyPairs); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCertKeyPair(currLocation, expectedPKIContent.CertKeyPairs)
			if err != nil {
				newTLSRegistry.CertKeyPairs = append(newTLSRegistry.CertKeyPairs, currCertKeyPair)
			}
		}

		for _, currCABundle := range actualPKIContent.InClusterResourceData.CertificateAuthorityBundles {
			currLocation := currCABundle.ConfigMapLocation
			if _, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, violationsPKIContent.CertificateAuthorityBundles); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, expectedPKIContent.CertificateAuthorityBundles)
			if err != nil {
				newTLSRegistry.CertificateAuthorityBundles = append(newTLSRegistry.CertificateAuthorityBundles, currCABundle)
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
			testresult.Flakef(fmt.Sprintf("Unregistered TLS certificates found:\n%s\nSee tls/ownership/README.md in origin repo", registryString))
		}
	})

	g.It("all registered tls artifacts must have expected owners", func() {
		messages := []string{}

		for _, currCertKeyPair := range actualPKIContent.InClusterResourceData.CertKeyPairs {
			currLocation := currCertKeyPair.SecretLocation
			expectedSecret, err := certgraphutils.LocateCertKeyPair(currLocation, expectedPKIContent.CertKeyPairs)
			if err != nil {
				continue
			}
			if diff := cmp.Diff(expectedSecret.CertKeyInfo.OwningJiraComponent, currCertKeyPair.CertKeyInfo.OwningJiraComponent); len(diff) > 0 {
				messages = append(messages, fmt.Sprintf("--namespace=%s, secret/%s:\n%v\n", currLocation.Namespace, currLocation.Name, diff))
			}
		}

		for _, currCABundle := range actualPKIContent.InClusterResourceData.CertificateAuthorityBundles {
			currLocation := currCABundle.ConfigMapLocation
			expectedCABundle, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, expectedPKIContent.CertificateAuthorityBundles)
			if err != nil {
				continue
			}
			if diff := cmp.Diff(expectedCABundle.CABundleInfo.OwningJiraComponent, currCABundle.CABundleInfo.OwningJiraComponent); len(diff) > 0 {
				messages = append(messages, fmt.Sprintf("--namespace=%s, configmap/%s:\n%v\n", currLocation.Namespace, currLocation.Name, diff))
			}
		}
		if len(messages) > 0 {
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(strings.Join(messages, "\n"))
			testresult.Flakef(strings.Join(messages, "\n"))
		}
	})

})

func fetchOnDiskCertificates(ctx context.Context, kubeClient kubernetes.Interface, nodeList *corev1.NodeList, testPullSpec string) (*certgraphapi.PKIList, error) {
	namespace, err := createNamespace(ctx, kubeClient)
	if err != nil {
		return nil, err
	}
	defer kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})

	err = createServiceAccount(ctx, kubeClient, namespace)
	if err != nil {
		return nil, err
	}
	err = createRBACBindings(ctx, kubeClient, namespace)
	if err != nil {
		return nil, err
	}

	err = createPods(ctx, kubeClient, namespace, nodeList, testPullSpec)
	if err != nil {
		return nil, err
	}

	ret := &certgraphapi.PKIList{}
	errs := []error{}
	for _, node := range nodeList.Items {
		nodePKIList, err := fetchNodePKIList(ctx, kubeClient, &node)
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

func createRBACBindings(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	roleBindingObj := resourceread.ReadRoleBindingV1OrDie(roleBindingPrivilegedYaml)
	roleBindingObj.Namespace = namespace

	client := kubeClient.RbacV1().RoleBindings(namespace)
	_, err := client.Create(ctx, roleBindingObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating hostaccess SCC CRB: %v", err)
	}
	return nil
}

func createPods(ctx context.Context, kubeClient kubernetes.Interface, namespace string, nodeList *corev1.NodeList, testImagePullSpec string) error {
	client := kubeClient.CoreV1().Pods(namespace)

	podTemplate := resourceread.ReadPodV1OrDie(podYaml)
	for _, node := range nodeList.Items {
		podObj := podTemplate.DeepCopy()
		podObj.Namespace = namespace
		podObj.Spec.NodeName = node.Name
		podObj.Spec.Containers[0].Image = testImagePullSpec

		actualPod, err := client.Create(ctx, podObj, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating pod on node %s: %v", node.Name, err)
		}

		timeLimitedCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
			cache.NewListWatchFromClient(
				kubeClient.CoreV1().RESTClient(), "pods", namespace, fields.OneTermEqualSelector("metadata.name", actualPod.Name)),
			&corev1.Pod{},
			nil,
			func(event watch.Event) (bool, error) {
				pod := event.Object.(*corev1.Pod)
				return pod.Status.Phase == corev1.PodSucceeded, nil
			},
		); watchErr != nil {
			return fmt.Errorf("pod %s in namespace %s didn't complete: %v", actualPod.Name, namespace, watchErr)
		}
	}
	return nil
}

func fetchNodePKIList(ctx context.Context, kubeClient kubernetes.Interface, node *corev1.Node) (*certgraphapi.PKIList, error) {
	pkiList := &certgraphapi.PKIList{}

	allBytes, err := nodeaccess.GetNodeLogFile(ctx, kubeClient, node.Name, certInspectResultFile)
	if err != nil {
		return pkiList, fmt.Errorf("failed to fetch file %s on node %s: %v", certInspectResultFile, node.Name, err)
	}
	err = json.Unmarshal(allBytes, pkiList)
	if err != nil {
		return pkiList, fmt.Errorf("failed to unmarshal file %s on node %s: %v", certInspectResultFile, node.Name, err)
	}

	return pkiList, nil
}

func processOnNodePKIList(pkiList *certgraphapi.PKIList) (*certgraphapi.PKIList, error) {
	// Remove keys without matched cert
	for idx, cert := range pkiList.CertKeyPairs.Items {
		locations := []certgraphapi.OnDiskCertKeyPairLocation{}
		for _, loc := range cert.Spec.OnDiskLocations {
			if loc.Cert.Path == "" && loc.Key.Path != "" {
				continue
			}
			locations = append(locations, loc)
		}
		pkiList.CertKeyPairs.Items[idx].Spec.OnDiskLocations = locations
	}

	// Remove certs which have no secret locations or on disk locations
	certs := []certgraphapi.CertKeyPair{}
	for _, cert := range pkiList.CertKeyPairs.Items {
		if len(cert.Spec.OnDiskLocations) == 0 && len(cert.Spec.SecretLocations) == 0 {
			continue
		}
		certs = append(certs, cert)
	}
	pkiList.CertKeyPairs.Items = certs
	return pkiList, nil
}

// GetOpenshiftTestsImagePullSpec returns the pull spec or an error.
func getOpenshiftTestsImagePullSpec(ctx context.Context, adminRESTConfig *rest.Config) (string, error) {
	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return "", err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("clusterversion/version not found and no image pull spec specified")
	}
	if err != nil {
		return "", err
	}
	payloadImagePullSpec := clusterVersion.Status.History[0].Image

	// runImageExtract extracts src from specified image to dst
	cmd := exec.Command("oc", "adm", "release", "info", payloadImagePullSpec, "--image-for=tests")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("unable to determine openshift-tests image: %v: %v", err, errOut.String())
	}
	openshiftTestsImagePullSpec := strings.TrimSpace(out.String())
	fmt.Printf("openshift-tests image pull spec is %v\n", openshiftTestsImagePullSpec)

	return openshiftTestsImagePullSpec, nil
}
