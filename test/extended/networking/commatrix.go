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
	configv1 "github.com/openshift/api/config/v1"
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

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("generated communication matrix should be equal to documented communication matrix", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")

		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		cs, err := client.New()
		o.Expect(err).ToNot(o.HaveOccurred())

		configClient := configv1client.NewForConfigOrDie(cs.Config)

		deployment := types.Standard
		isSNO, err := isSNOCluster(configClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSNO {
			deployment = types.SNO
		}

		env := types.Cloud
		isBM, err := isBMCluster(configClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		if isBM {
			env = types.Baremetal
		}

		g.By("get cluster's version and check if it's suitable for test")
		clusterVersion, err := getClusterVersion(configClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		floatClusterVersion, err := strconv.ParseFloat(clusterVersion, 64)
		o.Expect(err).ToNot(o.HaveOccurred())

		if floatClusterVersion < minimalDocCommatrixVersion {
			g.Skip(fmt.Sprintf("If the cluster version is lower than the lowest version that "+
				"has a documented communication matrix (%v), skip test", minimalDocCommatrixVersion))
		}
		
		docCommatrixVersionedUrl := strings.Replace(docCommatrixBaseUrl, "VERSION", clusterVersion, 1)

		g.By("preparing for commatrices generation")
		epExporter, err := endpointslices.New(cs)
		o.Expect(err).ToNot(o.HaveOccurred())
		utilsHelpers := utils.New(cs)

		g.By("generating new commatrix")
		newComMatrixCreator, err := commatrixcreator.New(epExporter, "", "", env, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix, err := newComMatrixCreator.CreateEndpointMatrix()
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix.WriteMatrixToFileByType(utilsHelpers, "new-commatrix", types.FormatCSV, deployment, artifactsDir)

		g.By("get documented commatrix")
		// get documented commatrix from URL
		resp, err := http.Get(docCommatrixVersionedUrl)
		o.Expect(err).ToNot(o.HaveOccurred())
		defer resp.Body.Close()
		// expect response status code to differ from status not found
		o.Expect(resp.StatusCode).ToNot(o.Equal(http.StatusNotFound))

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
		areEqual := comMatricesAreEqual(*newComMatrix, *docComMatrix)
		o.Expect(areEqual).To(o.BeTrue())
	})
})

// getClusterVersion return cluster's Y stream version
func getClusterVersion(configClient *configv1client.ConfigV1Client) (string, error) {
	clusterVersion, err := configClient.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterVersionParts := strings.SplitN(clusterVersion.Status.Desired.Version, ".", 3)
	return strings.Join(clusterVersionParts[:2], "."), nil
}

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(oc *configv1client.ConfigV1Client) (bool, error) {
	infrastructureType, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	logrus.Infof("the cluster type is %s", infrastructureType.Status.ControlPlaneTopology)
	return infrastructureType.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

func isBMCluster(oc *configv1client.ConfigV1Client) (bool, error) {
	infrastructureType, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	logrus.Infof("the cluster platform is %s", infrastructureType.Status.PlatformStatus.Type)
	return infrastructureType.Status.PlatformStatus.Type == configv1.BareMetalPlatformType, nil
}

// comMatricesAreEqual return true if given comMatrices are equal, and false otherwise
func comMatricesAreEqual(cm1 types.ComMatrix, cm2 types.ComMatrix) bool {
	diff1 := cm1.Diff(cm2)
	diff2 := cm2.Diff(cm1)

	// Check if the Diff matrices are not empty
	if len(diff1.Matrix) > 0 || len(diff2.Matrix) > 0 {
		return false
	}

	// Diff matrices are empty, ComMatrices are equal
	return true
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
