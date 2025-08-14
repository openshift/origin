package networking

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	version "github.com/hashicorp/go-version"
	"github.com/openshift-kni/commatrix/pkg/types"
	configv1 "github.com/openshift/api/config/v1"
	clientOptions "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	listeningsockets "github.com/openshift-kni/commatrix/pkg/listening-sockets"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/utils"
	"github.com/openshift-kni/commatrix/test/pkg/cluster"
)

const (
	docTypeHolder           = "TYPE"
	docVersionHolder        = "VERSION"
	docReleaseTemplate      = "release-" + docVersionHolder
	docCommatrixTemplateURL = "https://raw.githubusercontent.com/openshift-kni/commatrix/refs/heads/%s/docs/stable/raw/%s.csv"
	diffFileComments        = "// `+` indicates a port that isn't in the current documented matrix, and has to be added.\n" +
		"// `-` indicates a port that has to be removed from the documented matrix.\n"
	minimalDocCommatrixVersionStr = "4.18"
	commatrixArfticatFolder       = "commatrix-e2e"
	docFilePath                   = "doc-commatrix.csv"
	docDiffCoammtrixFilePath      = "doc-diff-commatrix"
	masterNodeRole                = "master"
	workerNodeRole                = "worker"
	testNS                        = "openshift-commatrix-test"
	serviceNodePortMin            = 30000
	serviceNodePortMax            = 32767
)

var docCommatrixBaseURL = fmt.Sprintf(docCommatrixTemplateURL, docReleaseTemplate, docTypeHolder)

// Temporarly exception for external port used only interanly.
var staticOpenPortsToIgnore = []types.ComDetails{
	{
		Direction: "Ingress",
		Protocol:  "TCP",
		Port:      9447,
		NodeRole:  "master",
		Service:   "baremetal-operator-webhook-service",
		Namespace: "openshift-machine-api",
		Pod:       "metal3-baremetal-operator",
		Container: "metal3-baremetal-operator",
		Optional:  false,
	},
}

var (
	cs                *client.ClientSet
	epExporter        *endpointslices.EndpointSlicesExporter
	isSNO             bool
	infraType         configv1.PlatformType
	deployment        types.Deployment
	utilsHelpers      utils.UtilsInterface
	artifactsDir      string
	commMatrixCreator *commatrixcreator.CommunicationMatrixCreator
	commatrix         *types.ComMatrix
)

var _ = Describe("[sig-network][Feature:commatrix][apigroup:config.openshift.io][Serial]", func() {
	BeforeEach(func() {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			Fail("KUBECONFIG not set")
		}

		By("Creating output folder")
		artifactsDir = os.Getenv("ARTIFACT_DIR")
		if artifactsDir == "" {
			log.Println("env var ARTIFACT_DIR is not set, using default value")
		}
		artifactsDir = filepath.Join(artifactsDir, commatrixArfticatFolder)

		err := os.MkdirAll(artifactsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		By("Creating the clients for the Generating step")
		cs, err = client.New()
		Expect(err).NotTo(HaveOccurred())

		utilsHelpers = utils.New(cs)
		epExporter, err = endpointslices.New(cs)
		Expect(err).NotTo(HaveOccurred())

		By("Get cluster's deployment and infrastructure types")
		deployment = types.Standard
		isSNO, err = utilsHelpers.IsSNOCluster()
		Expect(err).NotTo(HaveOccurred())

		if isSNO {
			deployment = types.SNO
		}

		infraType, err = utilsHelpers.GetPlatformType()
		Expect(err).NotTo(HaveOccurred())

		// if cluster's type is not supported by the commatrix app, skip tests
		if !slices.Contains(types.SupportedPlatforms, infraType) {
			Skip(fmt.Sprintf("unsupported platform type: %s. Supported platform types are: %v", infraType, types.SupportedPlatforms))
		}

		By("Generating cluster's communication matrix creator")
		commMatrixCreator, err = commatrixcreator.New(epExporter, "", "", infraType, deployment)
		Expect(err).NotTo(HaveOccurred())

		By("Create endpoint matrix ")
		commatrix, err = commMatrixCreator.CreateEndpointMatrix()
		Expect(err).NotTo(HaveOccurred())

		err = commatrix.WriteMatrixToFileByType(utilsHelpers, "communication-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())
	})

	It("generated communication matrix should be equal to documented communication matrix", func() {
		By("Get cluster's version and check if it's suitable for test")
		clusterVersionStr, err := cluster.GetClusterVersion(cs)
		Expect(err).NotTo(HaveOccurred())
		clusterVersion, err := version.NewVersion(clusterVersionStr)
		Expect(err).ToNot(HaveOccurred())

		minimalDocCommatrixVersion, err := version.NewVersion(minimalDocCommatrixVersionStr)
		Expect(err).ToNot(HaveOccurred())

		if clusterVersion.LessThan(minimalDocCommatrixVersion) {
			Skip(fmt.Sprintf("Cluster version is lower than the lowest version that has a documented communication matrix (%v)", minimalDocCommatrixVersionStr))
		}

		By("Generate documented commatrix file path")
		docCommatrixURL := strings.Replace(docCommatrixBaseURL, docVersionHolder, clusterVersionStr, 1)

		// clusters with unsupported platform types had skip the test, so we assume the platform type is supported
		var docType string
		switch infraType {
		case configv1.AWSPlatformType:
			docType = "aws"
		case configv1.BareMetalPlatformType:
			docType = "bm"
		case configv1.NonePlatformType:
			docType = "none"
		}

		if isSNO {
			docType += "-sno"
		}
		docCommatrixURL = strings.Replace(docCommatrixURL, docTypeHolder, docType, 1)

		By(fmt.Sprintf("Get documented commatrix version %s type %s", clusterVersionStr, docType))
		// get documented commatrix from URL
		resp, err := http.Get(docCommatrixURL)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		// if response status code equals to "status not found", compare generated commatrix to the main documented commatrix
		if resp.StatusCode == http.StatusNotFound {
			docCommatrixURL = strings.Replace(docCommatrixBaseURL, docReleaseTemplate, "main", 1)
			docCommatrixURL = strings.Replace(docCommatrixURL, docTypeHolder, docType, 1)
			resp, err = http.Get(docCommatrixURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).ToNot(Equal(http.StatusNotFound))
		}

		By("Write documented commatrix to artifact file")
		docCommatrixContent, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		docCommatrixFilePath := filepath.Join(artifactsDir, docFilePath)
		err = os.WriteFile(docCommatrixFilePath, docCommatrixContent, 0644)
		Expect(err).ToNot(HaveOccurred())

		By("Get documented commatrix details")
		docCommatrixFileContent, err := os.ReadFile(docCommatrixFilePath)
		Expect(err).ToNot(HaveOccurred(), "Failed to read documented communication matrix file")
		docComDetailsList, err := types.ParseToComDetailsList(docCommatrixFileContent, types.FormatCSV)
		Expect(err).ToNot(HaveOccurred())
		docComMatrix := &types.ComMatrix{Matrix: docComDetailsList}

		By("Generating diff between matrices for testing purposes")
		endpointslicesDiffWithDocMat := matrixdiff.Generate(commatrix, docComMatrix)
		diffStr, err := endpointslicesDiffWithDocMat.String()
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filepath.Join(artifactsDir, "doc-diff-commatrix"), []byte(diffFileComments+diffStr), 0644)
		Expect(err).ToNot(HaveOccurred())

		By("Comparing new and documented commatrices")
		// Get ports that are in the documented commatrix but not in the generated commatrix.
		notUsedPortsMat := endpointslicesDiffWithDocMat.GetUniqueSecondary()
		if len(notUsedPortsMat.Matrix) > 0 {
			logrus.Warningf("unused documented ports found:\n%s", notUsedPortsMat)
		}

		portsToIgnoreMat := &types.ComMatrix{Matrix: staticOpenPortsToIgnore}

		openPortsToIgnoreFile, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FILE")
		openPortsToIgnoreFormat, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FORMAT")

		// Get ports that are in the generated commatrix but not in the documented commatrix,
		// and ignore the ports in given file (if exists)
		if openPortsToIgnoreFile != "" && openPortsToIgnoreFormat != "" {
			// generate open ports to ignore commatrix
			portsToIgnoreFileContent, err := os.ReadFile(openPortsToIgnoreFile)
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreComDetails, err := types.ParseToComDetailsList(portsToIgnoreFileContent, openPortsToIgnoreFormat)
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreMat.Matrix = append(portsToIgnoreMat.Matrix, portsToIgnoreComDetails...)
		}

		// generate the diff matrix between the open ports to ignore matrix and the missing ports in the documented commatrix (based on the diff between the enpointslice and the doc matrix)
		nonDocumentedEndpointslicesMat := endpointslicesDiffWithDocMat.GetUniquePrimary()
		endpointslicesDiffWithIgnoredPorts := matrixdiff.Generate(nonDocumentedEndpointslicesMat, portsToIgnoreMat)
		missingPortsMat := endpointslicesDiffWithIgnoredPorts.GetUniquePrimary()

		Expect(missingPortsMat.Matrix).To(BeEmpty(), fmt.Sprintf("non-documented open ports found: \n%v", missingPortsMat))

		if len(missingPortsMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but are not documented:\n%s", missingPortsMat)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("should validate the communication matrix ports match the node's listening ports", func() {
		listeningCheck, err := listeningsockets.NewCheck(cs, utilsHelpers, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		By("Creating test namespace")
		err = utilsHelpers.CreateNamespace(testNS)
		Expect(err).ToNot(HaveOccurred())

		By("generate the ss matrix and ss raws")
		ssMat, ssOutTCP, ssOutUDP, err := listeningCheck.GenerateSS(testNS)
		Expect(err).ToNot(HaveOccurred())

		err = listeningCheck.WriteSSRawFiles(ssOutTCP, ssOutUDP)
		Expect(err).ToNot(HaveOccurred())

		err = ssMat.WriteMatrixToFileByType(utilsHelpers, "ss-generated-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		// generate the diff matrix between the enpointslice and the ss matrix
		ssFilteredMat, err := filterSSMatrix(ssMat)
		Expect(err).ToNot(HaveOccurred())

		diff := matrixdiff.Generate(commatrix, ssFilteredMat)
		diffStr, err := diff.String()
		Expect(err).ToNot(HaveOccurred())

		err = utilsHelpers.WriteFile(filepath.Join(artifactsDir, "matrix-diff-ss"), []byte(diffStr))
		Expect(err).ToNot(HaveOccurred())

		notUsedEPSMat := diff.GetUniquePrimary()
		if len(notUsedEPSMat.Matrix) > 0 {
			logrus.Warningf("the following ports are not used: \n %s", notUsedEPSMat)
		}

		missingEPSMat := diff.GetUniqueSecondary()
		if len(missingEPSMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but don't have an endpointslice: \n %s", missingEPSMat)
			Expect(err).ToNot(HaveOccurred())
		}

		By("Deleting test Namespace")
		err = utilsHelpers.DeleteNamespace(testNS)
		Expect(err).ToNot(HaveOccurred())
	})
})

// Filter ss known ports to skip in matrix diff.
func filterSSMatrix(mat *types.ComMatrix) (*types.ComMatrix, error) {
	nodePortMin := serviceNodePortMin
	nodePortMax := serviceNodePortMax

	clusterNetwork := &configv1.Network{}
	err := cs.Get(context.Background(), clientOptions.ObjectKey{Name: "cluster"}, clusterNetwork)
	if err != nil {
		return nil, err
	}

	serviceNodePortRange := clusterNetwork.Spec.ServiceNodePortRange
	if serviceNodePortRange != "" {
		rangeStr := strings.Split(serviceNodePortRange, "-")

		nodePortMin, err = strconv.Atoi(rangeStr[0])
		if err != nil {
			return nil, err
		}

		nodePortMax, err = strconv.Atoi(rangeStr[1])
		if err != nil {
			return nil, err
		}
	}

	res := []types.ComDetails{}
	for _, cd := range mat.Matrix {
		// Skip "ovnkube" ports in the nodePort range, these are dynamic open ports on the node,
		// no need to mention them in the matrix diff
		if cd.Service == "ovnkube" && cd.Port >= nodePortMin && cd.Port <= nodePortMax {
			continue
		}

		// Skip "rpc.statd" ports, these are randomly open ports on the node,
		// no need to mention them in the matrix diff
		if cd.Service == "rpc.statd" {
			continue
		}

		// Skip crio stream server port, allocated to a random free port number,
		// shouldn't be exposed to the public Internet for security reasons,
		// no need to mention it in the matrix diff
		if cd.Service == "crio" && cd.Port > nodePortMax {
			continue
		}

		// Skip dns ports used during provisioning for dhcp and tftp,
		// not used for external traffic
		if cd.Service == "dnsmasq" || cd.Service == "dig" {
			continue
		}

		res = append(res, cd)
	}

	return &types.ComMatrix{Matrix: res}, nil
}
