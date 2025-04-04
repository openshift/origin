package networking

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"

	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/types"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/utils"
	"github.com/openshift-kni/commatrix/test/pkg/cluster"
	corev1 "k8s.io/api/core/v1"
)

var (
	// Entries which are open on the worker node instead of master in standard cluster.
	// Will be excluded in diff generatation between documented and generated comMatrix.
	StandardExcludedMasterComDetails = []types.ComDetails{
		{
			Direction: "Ingress",
			Protocol:  "TCP",
			Port:      80,
			NodeRole:  "master",
			Service:   "router-default",
			Namespace: "openshift-ingress",
			Pod:       "router-default",
			Container: "router",
			Optional:  false,
		}, {
			Direction: "Ingress",
			Protocol:  "TCP",
			Port:      443,
			NodeRole:  "master",
			Service:   "router-default",
			Namespace: "openshift-ingress",
			Pod:       "router-default",
			Container: "router",
			Optional:  false,
		},
	}
)

const (
	docTypeHolder    = "TYPE"
	docVersionHolder = "VERSION"
	diffFileComments = "// `+` indicates a port that isn't in the current documented matrix, and has to be added.\n" +
		"// `-` indicates a port that has to be removed from the documented matrix.\n"
	testNS                     = "openshift-commatrix-test"
	serviceNodePortMin         = 30000
	serviceNodePortMax         = 32767
	minimalDocCommatrixVersion = 4.18
)

var docCommatrixBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/openshift-kni/commatrix/refs/heads/release-%s/docs/stable/raw/%s.csv", docVersionHolder, docTypeHolder)

var _ = Describe("[sig-network][Feature:commatrix][apigroup:config.openshift.io][Serial]", func() {
	It("generated communication matrix should be equal to documented communication matrix", func() {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			Fail("KUBECONFIG not set")
		}

		By("Creating output folder")
		artifactsDir := os.Getenv("ARTIFACT_DIR")
		if artifactsDir == "" {
			log.Println("env var ARTIFACT_DIR is not set, using default value")
		}
		artifactsDir = filepath.Join(artifactsDir, "commatrix-e2e")

		err := os.MkdirAll(artifactsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		By("Creating the clients for the Generating step")
		cs, err := client.New()
		Expect(err).NotTo(HaveOccurred())

		utilsHelpers := utils.New(cs)

		epExporter, err := endpointslices.New(cs)
		Expect(err).ToNot(HaveOccurred())

		By("Get cluster's deployment and infrastructure types")
		deployment := types.Standard
		isSNO, err := utilsHelpers.IsSNOCluster()
		Expect(err).NotTo(HaveOccurred())
		if isSNO {
			deployment = types.SNO
		}

		infra := types.Cloud
		isBM, err := utilsHelpers.IsBMInfra()
		Expect(err).NotTo(HaveOccurred())
		if isBM {
			infra = types.Baremetal
		}

		By("Generating comMatrix")
		commMatrixCreator, err := commatrixcreator.New(epExporter, "", "", infra, deployment)
		Expect(err).NotTo(HaveOccurred())

		commatrix, err := commMatrixCreator.CreateEndpointMatrix()
		Expect(err).NotTo(HaveOccurred())

		err = commatrix.WriteMatrixToFileByType(utilsHelpers, "communication-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		By("Creating test namespace")
		err = utilsHelpers.CreateNamespace(testNS)
		Expect(err).ToNot(HaveOccurred())

		nodeList := &corev1.NodeList{}
		err = cs.List(context.TODO(), nodeList)
		Expect(err).ToNot(HaveOccurred())

		By("get cluster's version and check if it's suitable for test")
		clusterVersion, err := cluster.GetClusterVersion(cs)
		Expect(err).NotTo(HaveOccurred())
		floatClusterVersion, err := strconv.ParseFloat(clusterVersion, 64)
		Expect(err).ToNot(HaveOccurred())

		if floatClusterVersion < minimalDocCommatrixVersion {
			Skip(fmt.Sprintf("If the cluster version is lower than the lowest version that "+
				"has a documented communication matrix (%v), skip test", minimalDocCommatrixVersion))
		}
		docCommatrixURL := strings.Replace(docCommatrixBaseURL, docVersionHolder, clusterVersion, 1)

		By("Generate documented commatrix file path")
		docType := "aws"
		if isBM {
			docType = "bm"
		}
		if isSNO {
			docType += "-sno"
		}
		docCommatrixURL = strings.Replace(docCommatrixURL, docTypeHolder, docType, 1)

		By(fmt.Sprintf("get documented commatrix version %s type %s", clusterVersion, docType))
		// get documented commatrix from URL
		resp, err := http.Get(docCommatrixURL)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		// if response status code equals to "status not found", compare generated commatrix to the main documented commatrix
		if resp.StatusCode == http.StatusNotFound {
			resp, err = http.Get(strings.Replace(docCommatrixBaseURL, "release-VERSION", "main", 1))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).ToNot(Equal(http.StatusNotFound))
		}

		By("write documented commatrix to artifact file")
		docCommatrixContent, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		docCommatrixFilePath := filepath.Join(artifactsDir, "doc-commatrix.csv")
		err = os.WriteFile(docCommatrixFilePath, docCommatrixContent, 0644)
		Expect(err).ToNot(HaveOccurred())

		By("Filter documented commatrix for diff generation")
		// get origin documented commatrix details
		docComMatrixCreator, err := commatrixcreator.New(epExporter, docCommatrixFilePath, types.FormatCSV, infra, deployment)
		Expect(err).ToNot(HaveOccurred())
		docComDetailsList, err := docComMatrixCreator.GetComDetailsListFromFile()
		Expect(err).ToNot(HaveOccurred())

		docComMatrix := filterDocumentedCommatrixByDeploymentAndInfraTypes(docComDetailsList, isSNO, isBM)

		By("generating diff between matrices for testing purposes")
		endpointslicesDiffWithDocMat := matrixdiff.Generate(commatrix, docComMatrix)
		diffStr, err := endpointslicesDiffWithDocMat.String()
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filepath.Join(artifactsDir, "doc-diff-commatrix"), []byte(diffFileComments+diffStr), 0644)
		Expect(err).ToNot(HaveOccurred())

		By("comparing new and documented commatrices")
		// Get ports that are in the documented commatrix but not in the generated commatrix.
		notUsedPortsMat := endpointslicesDiffWithDocMat.GenerateUniqueSecondary()
		if len(notUsedPortsMat.Matrix) > 0 {
			logrus.Warningf("the following ports are documented but are not used:\n%s", notUsedPortsMat)
		}

		var portsToIgnoreMat *types.ComMatrix

		openPortsToIgnoreFile, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FILE")
		openPortsToIgnoreFormat, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FORMAT")

		// Get ports that are in the generated commatrix but not in the documented commatrix,
		// and ignore the ports in given file (if exists)
		missingPortsMat := endpointslicesDiffWithDocMat.GenerateUniquePrimary()
		if openPortsToIgnoreFile != "" && openPortsToIgnoreFormat != "" {
			// generate open ports to ignore commatrix
			portsToIgnoreCommatrixCreator, err := commatrixcreator.New(epExporter, openPortsToIgnoreFile, openPortsToIgnoreFormat, infra, deployment)
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreComDetails, err := portsToIgnoreCommatrixCreator.GetComDetailsListFromFile()
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreMat = &types.ComMatrix{Matrix: portsToIgnoreComDetails}

			// generate the diff matrix between the open ports to ignore matrix and the missing ports in the documented commatrix (based on the diff between the enpointslice and the doc matrix)
			nonDocumentedEndpointslicesMat := endpointslicesDiffWithDocMat.GenerateUniquePrimary()
			endpointslicesDiffWithIgnoredPorts := matrixdiff.Generate(nonDocumentedEndpointslicesMat, portsToIgnoreMat)
			missingPortsMat = endpointslicesDiffWithIgnoredPorts.GenerateUniquePrimary()
		}

		if len(missingPortsMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but are not documented:\n%s", missingPortsMat)
			Expect(err).ToNot(HaveOccurred())
		}

		By("Deleting Namespace")
		err = utilsHelpers.DeleteNamespace(testNS)
		Expect(err).ToNot(HaveOccurred())
	})
})

// excludeStaticEntriesWithGivenNodeRole excludes from comDetails, static entries from staticEntriesMatrix with the given nodeRole
// The function returns filtered ComDetails without the excluded entries.
func excludeStaticEntriesWithGivenNodeRole(comDetails []types.ComDetails, staticEntriesMatrix *types.ComMatrix, nodeRole string) []types.ComDetails {
	filteredComDetails := []types.ComDetails{}
	for _, cd := range comDetails {
		switch cd.NodeRole {
		case nodeRole:
			if !staticEntriesMatrix.Contains(cd) {
				filteredComDetails = append(filteredComDetails, cd)
			}
		default:
			filteredComDetails = append(filteredComDetails, cd)
		}
	}
	return filteredComDetails
}

func filterDocumentedCommatrixByDeploymentAndInfraTypes(docComDetailsList []types.ComDetails, isSNO, isBM bool) *types.ComMatrix {
	if isSNO {
		// Exclude all worker nodes static entries.
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: docComDetailsList}, "worker")
		// Exclude static entries of standard deployment type.
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: types.StandardStaticEntries}, "master")
	} else {
		// Exclude specific master entries (see StandardExcludedMasterComDetails var description)
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: StandardExcludedMasterComDetails}, "master")
	}

	// if cluster is running on BM exclude Cloud static entries in diff generation
	// else cluster is running on Cloud and exclude BM static entries in diff generation.
	if isBM {
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: types.CloudStaticEntriesWorker}, "worker")
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: types.CloudStaticEntriesMaster}, "master")
	} else {
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: types.BaremetalStaticEntriesWorker}, "worker")
		docComDetailsList = excludeStaticEntriesWithGivenNodeRole(docComDetailsList, &types.ComMatrix{Matrix: types.BaremetalStaticEntriesMaster}, "master")
	}

	return &types.ComMatrix{Matrix: docComDetailsList}
}
