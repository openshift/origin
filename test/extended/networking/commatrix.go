package networking

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"slices"
	"strings"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	configv1 "github.com/openshift/api/config/v1"

	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/types"

	version "github.com/hashicorp/go-version"
	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/utils"
	"github.com/openshift-kni/commatrix/test/pkg/cluster"
	"github.com/openshift/library-go/pkg/git"
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
	commatrixRepoDir              = "/tmp/commatrix"
	commatrixFile                 = "communication-matrix.csv"
	docFilePath                   = "doc-commatrix.csv"
	docDiffCoammtrixFilePath      = "doc-diff-commatrix"
	masterNodeRole                = "master"
	workerNodeRole                = "worker"
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

// port used only interanly just on ci.
var staticOpenPortsToIgnoreInStaticEntry = []types.ComDetails{
	{
		Direction: "Ingress",
		Protocol:  "TCP",
		Port:      10250,
		NodeRole:  "worker",
		Optional:  false,
	},
	{
		Direction: "Ingress",
		Protocol:  "TCP",
		Port:      10250,
		NodeRole:  "master",
		Optional:  false,
	},
	{
		Direction: "Ingress",
		Protocol:  "TCP",
		Port:      6385,
		NodeRole:  "master",
		Optional:  false,
	},
}

var (
	cs                *client.ClientSet
	epExporter        *endpointslices.EndpointSlicesExporter
	isSNO             bool
	platform          configv1.PlatformType
	deployment        types.Deployment
	utilsHelpers      utils.UtilsInterface
	artifactsDir      string
	clusterVersionStr string
	commMatrixCreator *commatrixcreator.CommunicationMatrixCreator
	commatrix         *types.ComMatrix
)

func installCommatrixPlugin() error {
	if err := os.RemoveAll(commatrixRepoDir); err != nil {
		return fmt.Errorf("unable to remove %s directory: %v", commatrixRepoDir, err)
	}

	if err := git.NewRepository().Clone("/tmp/commatrix", "https://github.com/openshift-kni/commatrix.git"); err != nil {
		return fmt.Errorf("error cloning commatrix repo: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("sh", "-c", `
		if [ ! -d /tmp/bin ]; then
			mkdir -p /tmp/bin
		fi
		chmod -R a+w /tmp/bin
	`)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %s\nstdout:\n%s\nstderr:\n%s", cmd.String(), stdout.String(), stderr.String())
	}

	cmd = exec.Command("sh", "-c", "make build && make install INSTALL_DIR=/tmp/bin/")
	cmd.Dir = "/tmp/commatrix"
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %s\nstdout:\n%s\nstderr:\n%s", cmd.String(), stdout.String(), stderr.String())
	}

	return nil
}

func getDocumentedCommatrixType(platform configv1.PlatformType, isSNO bool) (string, error) {
	var docType string
	switch platform {
	case configv1.AWSPlatformType:
		docType = "aws"
	case configv1.BareMetalPlatformType:
		docType = "bm"
	case configv1.NonePlatformType:
		docType = "none"
	default:
		return "", fmt.Errorf("platform type %s is not supported", platform)
	}

	if isSNO {
		docType += "-sno"
	}

	return docType, nil
}

var _ = Describe("[sig-network][Feature:commatrix][apigroup:config.openshift.io][Serial]", Ordered, func() {
	BeforeAll(func() {
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

		By("Install commatrix plugin")
		err = installCommatrixPlugin()
		Expect(err).NotTo(HaveOccurred())

		By("Creating the clients for the Generating step")
		cs, err = client.New()
		Expect(err).NotTo(HaveOccurred())

		utilsHelpers = utils.New(cs)
		epExporter, err = endpointslices.New(cs)
		Expect(err).NotTo(HaveOccurred())

		By("Get cluster's deployment and platform types")
		deployment = types.Standard
		isSNO, err = utilsHelpers.IsSNOCluster()
		Expect(err).NotTo(HaveOccurred())

		if isSNO {
			deployment = types.SNO
		}

		platform, err = utilsHelpers.GetPlatformType()
		Expect(err).NotTo(HaveOccurred())

		// if cluster's type is not supported by the commatrix app, skip tests
		if !slices.Contains(types.SupportedPlatforms, platform) {
			Skip(fmt.Sprintf("unsupported platform type: %s. Supported platform types are: %v", platformType, types.SupportedPlatforms))
		}

		commMatrixCreator, err = commatrixcreator.New(epExporter, "", "", platform, deployment)
		Expect(err).NotTo(HaveOccurred())

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
	})

	It("generated communication matrix should be equal to documented communication matrix", func() {
		By("Generating communication matrix using oc command")
		cmd := exec.Command("oc", "commatrix", "generate", "--host-open-ports", "--destDir", artifactsDir)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf(
			"Failed to run command: %s\nstdout:\n%s\nstderr:\n%s",
			cmd.String(), stdout.String(), stderr.String(),
		))

		By("Reading the generated commatrix files")
		commatrixFilePath := filepath.Join(artifactsDir, commatrixFile)
		commatrixFileContent, err := os.ReadFile(commatrixFilePath)
		Expect(err).ToNot(HaveOccurred(), "Failed to read generated commatrix file")

		ComDetailsMatrix, err := types.ParseToComDetailsList(commatrixFileContent, types.FormatCSV)
		Expect(err).ToNot(HaveOccurred(), "Failed to parse generated commatrix")
		commatrix = &types.ComMatrix{Matrix: ComDetailsMatrix}

		By("Generate documented commatrix file path")
		docCommatrixURL := strings.Replace(docCommatrixBaseURL, docVersionHolder, clusterVersionStr, 1)
		docType, err := getDocumentedCommatrixType(platform, isSNO)
		Expect(err).ToNot(HaveOccurred())
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

	It("Static entries should not overlap with those in the EndpointSlice; any shared entries must be removed", func() {
		By("Get EndpointSlice matrix")
		err := epExporter.LoadExposedEndpointSlicesInfo()
		Expect(err).NotTo(HaveOccurred())

		epSliceComDetails, err := epExporter.ToComDetails()
		Expect(err).NotTo(HaveOccurred())

		By("Get static entries list")
		staticEntries, err := commMatrixCreator.GetStaticEntries()
		Expect(err).NotTo(HaveOccurred())

		staticEntriesMat := &types.ComMatrix{Matrix: staticEntries}
		epSliceComDetailsMat := &types.ComMatrix{Matrix: epSliceComDetails}

		By("Write the matrix to files")
		err = staticEntriesMat.WriteMatrixToFileByType(utilsHelpers, "static-entry-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		err = epSliceComDetailsMat.WriteMatrixToFileByType(utilsHelpers, "expose-communication-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())

		By("Generating the Diff between the static entris and the expose communication matrix")
		endpointslicesDiffWithstaticEntrieMat := matrixdiff.Generate(epSliceComDetailsMat, staticEntriesMat)
		sharedEntries := endpointslicesDiffWithstaticEntrieMat.GetSharedEntries()

		portsToIgnoreMat := &types.ComMatrix{Matrix: staticOpenPortsToIgnoreInStaticEntry}
		sharedEntriesDiffWithIgnoredPorts := matrixdiff.Generate(sharedEntries, portsToIgnoreMat)
		staticEntryNeedToRemove := sharedEntriesDiffWithIgnoredPorts.GetUniquePrimary()

		if len(staticEntryNeedToRemove.Matrix) > 0 {
			err := fmt.Errorf("the following ports must be removed from the static entry file, as they already exist in an EndpointSlice:\n%s", staticEntryNeedToRemove)
			Expect(err).NotTo(HaveOccurred())
		}
	})
})
