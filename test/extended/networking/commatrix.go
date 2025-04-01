package networking

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	configv1 "github.com/openshift/api/config/v1"

	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/types"

	version "github.com/hashicorp/go-version"
	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
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
)

var docCommatrixBaseURL = fmt.Sprintf(docCommatrixTemplateURL, docReleaseTemplate, docTypeHolder)

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
		artifactsDir = filepath.Join(artifactsDir, commatrixArfticatFolder)

		err := os.MkdirAll(artifactsDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		By("Creating the clients for the Generating step")
		cs, err := client.New()
		Expect(err).NotTo(HaveOccurred())

		utilsHelpers := utils.New(cs)

		epExporter, err := endpointslices.New(cs)
		Expect(err).ToNot(HaveOccurred())

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

		By("Get cluster's deployment and infrastructure types")
		deployment := types.Standard
		isSNO, err := utilsHelpers.IsSNOCluster()
		Expect(err).NotTo(HaveOccurred())
		if isSNO {
			deployment = types.SNO
		}

		platformType, err := utilsHelpers.GetPlatformType()
		Expect(err).NotTo(HaveOccurred())

		// if cluster's type is not supported by the commatrix app, skip tests
		if !slices.Contains(types.SupportedPlatforms, platformType) {
			Skip(fmt.Sprintf("unsupported platform type: %s. Supported platform types are: %v", platformType, types.SupportedPlatforms))
		}

		By("Generating cluster's communication matrix")
		commMatrixCreator, err := commatrixcreator.New(epExporter, "", "", platformType, deployment)
		Expect(err).NotTo(HaveOccurred())

		commatrix, err := commMatrixCreator.CreateEndpointMatrix()
		Expect(err).NotTo(HaveOccurred())

		err = commatrix.WriteMatrixToFileByType(utilsHelpers, "communication-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		By("Generate documented commatrix file path")
		docCommatrixURL := strings.Replace(docCommatrixBaseURL, docVersionHolder, clusterVersionStr, 1)

		// clusters with unsupported platform types had skip the test, so we assume the platform type is supported
		var docType string
		switch platformType {
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

		var portsToIgnoreMat *types.ComMatrix

		openPortsToIgnoreFile, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FILE")
		openPortsToIgnoreFormat, _ := os.LookupEnv("OPEN_PORTS_TO_IGNORE_IN_DOC_TEST_FORMAT")

		// Get ports that are in the generated commatrix but not in the documented commatrix,
		// and ignore the ports in given file (if exists)
		missingPortsMat := endpointslicesDiffWithDocMat.GetUniquePrimary()
		if openPortsToIgnoreFile != "" && openPortsToIgnoreFormat != "" {
			// generate open ports to ignore commatrix
			portsToIgnoreFileContent, err := os.ReadFile(openPortsToIgnoreFile)
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreComDetails, err := types.ParseToComDetailsList(portsToIgnoreFileContent, openPortsToIgnoreFormat)
			Expect(err).ToNot(HaveOccurred())
			portsToIgnoreMat = &types.ComMatrix{Matrix: portsToIgnoreComDetails}

			// generate the diff matrix between the open ports to ignore matrix and the missing ports in the documented commatrix (based on the diff between the enpointslice and the doc matrix)
			nonDocumentedEndpointslicesMat := endpointslicesDiffWithDocMat.GetUniquePrimary()
			endpointslicesDiffWithIgnoredPorts := matrixdiff.Generate(nonDocumentedEndpointslicesMat, portsToIgnoreMat)
			missingPortsMat = endpointslicesDiffWithIgnoredPorts.GetUniquePrimary()
		}

		Expect(missingPortsMat.Matrix).To(BeEmpty(), fmt.Sprintf("non-documented open ports found: \n%v", missingPortsMat))

		if len(missingPortsMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but are not documented:\n%s", missingPortsMat)
			Expect(err).ToNot(HaveOccurred())
		}
	})
})
