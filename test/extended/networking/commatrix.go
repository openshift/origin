package networking

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	client "github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	minimalDocCommatrixVersion = 4.16
	docCommatrixBaseUrl        = "https://raw.githubusercontent.com/openshift/openshift-docs/enterprise-VERSION/snippets/network-flow-matrix.csv"
	diffFileComments           = "// `+` indicates a port that isn't in the current documented matrix, and has to be added.\n" +
		"// `-` indicates a port that has to be removed from the documented matrix.\n"
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

var _ = g.Describe("[sig-network][Feature:commatrix][apigroup:config.openshift.io][Serial]", func() {
	g.It("generated communication matrix should be equal to documented communication matrix", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")

		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		cs, err := client.New()
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("get cluster's version and check if it's suitable for test")
		clusterVersion, err := getClusterVersion(cs)
		o.Expect(err).NotTo(o.HaveOccurred())
		floatClusterVersion, err := strconv.ParseFloat(clusterVersion, 64)
		o.Expect(err).ToNot(o.HaveOccurred())

		if floatClusterVersion < minimalDocCommatrixVersion {
			g.Skip(fmt.Sprintf("If the cluster version is lower than the lowest version that "+
				"has a documented communication matrix (%v), skip test", minimalDocCommatrixVersion))
		}

		g.By("preparing for commatrices generation")
		epExporter, err := endpointslices.New(cs)
		o.Expect(err).ToNot(o.HaveOccurred())
		utilsHelpers := utils.New(cs)

		deployment := types.Standard
		isSNO, err := utilsHelpers.IsSNOCluster()
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSNO {
			deployment = types.SNO
		}

		env := types.Cloud
		isBM, err := utilsHelpers.IsBMInfra()
		o.Expect(err).NotTo(o.HaveOccurred())
		if isBM {
			env = types.Baremetal
		}

		g.By("generating new commatrix")
		newComMatrixCreator, err := commatrixcreator.New(epExporter, "", "", env, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix, err := newComMatrixCreator.CreateEndpointMatrix()
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix.WriteMatrixToFileByType(utilsHelpers, "new-commatrix", types.FormatCSV, deployment, artifactsDir)

		g.By(fmt.Sprintf("get documented commatrix version %s", clusterVersion))
		// get documented commatrix from URL
		resp, err := http.Get(strings.Replace(docCommatrixBaseUrl, "VERSION", clusterVersion, 1))
		o.Expect(err).ToNot(o.HaveOccurred())
		defer resp.Body.Close()
		// if response status code equals to "status not found", compare generated commatrix to the master documented commatrix
		if resp.StatusCode == http.StatusNotFound {
			resp, err = http.Get(strings.Replace(docCommatrixBaseUrl, "enterprise-VERSION", "main", 1))
			o.Expect(err).ToNot(o.HaveOccurred())
			o.Expect(resp.StatusCode).ToNot(o.Equal(http.StatusNotFound))
		}

		// write documented commatrix to file
		docCommatrixContent, err := io.ReadAll(resp.Body)
		o.Expect(err).ToNot(o.HaveOccurred())
		docFilePath := filepath.Join(artifactsDir, "doc-commatrix.csv")
		err = os.WriteFile(docFilePath, docCommatrixContent, 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Filter documented commatrix for diff generation")
		// get origin documented commatrix details
		docComMatrixCreator, err := commatrixcreator.New(epExporter, docFilePath, types.FormatCSV, env, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		docComDetailsList, err := docComMatrixCreator.GetComDetailsListFromFile()
		o.Expect(err).ToNot(o.HaveOccurred())

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
		docComMatrix := &types.ComMatrix{Matrix: docComDetailsList}

		g.By("generating diff between matrices for testing purposes")
		diff := matrixdiff.Generate(newComMatrix, docComMatrix)
		diffStr, err := diff.String()
		o.Expect(err).ToNot(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(artifactsDir, "doc-diff-new"), []byte(diffFileComments+diffStr), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("comparing new and documented commatrices")
		// Get ports that are in the documented commatrix but not in the generated commatrix.
		notUsedPortsMat := diff.GenerateUniqueSecondary()
		if len(notUsedPortsMat.Matrix) > 0 {
			logrus.Warningf("the following ports are documented but are not used: \n %s", notUsedPortsMat)
		}

		// Get ports that are in the generated commatrix but not in the documented commatrix.
		missingPortsMat := diff.GenerateUniquePrimary()
		if len(missingPortsMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but are not documented: \n %s", missingPortsMat)
			o.Expect(err).ToNot(o.HaveOccurred())
		}
	})
})

// getClusterVersion return cluster's Y stream version
func getClusterVersion(cs *client.ClientSet) (string, error) {
	configClient := configv1client.NewForConfigOrDie(cs.Config)
	clusterVersion, err := configClient.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterVersionParts := strings.SplitN(clusterVersion.Status.Desired.Version, ".", 3)
	return strings.Join(clusterVersionParts[:2], "."), nil
}

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
