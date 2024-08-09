package networking

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	clientutil "github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	docCommatrixUrl  = "https://raw.githubusercontent.com/openshift/openshift-docs/main/snippets/network-flow-matrix.csv"
	diffFileComments = "// `+` indicates a port that isn't in the current doccumented matrix, and has to be added.\n" +
		"// `-` indicates a port that has to be removed from the doccumented matrix.\n"
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("should be equal to documeneted communication matrix", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")

		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		cs, err := clientutil.New()
		o.Expect(err).ToNot(o.HaveOccurred())

		deployment := types.MNO
		isSNO, err := isSNOCluster(cs)
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSNO {
			deployment = types.SNO
		}

		g.By("preparing for commatrices generation")
		epExporter, err := endpointslices.New(cs)
		o.Expect(err).ToNot(o.HaveOccurred())
		utilsHelpers := utils.New(cs)

		g.By("generating new commatrix")
		newComMatrixCreator, err := commatrixcreator.New(epExporter, "", "", types.Cloud, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix, err := newComMatrixCreator.CreateEndpointMatrix()
		o.Expect(err).ToNot(o.HaveOccurred())
		newComMatrix.WriteMatrixToFileByType(utilsHelpers, "new-commatrix", types.FormatCSV, deployment, artifactsDir)

		g.By("get documented commatrix")
		fp := filepath.Join(artifactsDir, "doc-commatrix.csv")
		createCSVFromUrl(docCommatrixUrl, fp)
		docComMatrixCreator, err := commatrixcreator.New(epExporter, fp, types.FormatCSV, types.Cloud, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		docComDetailsList, err := docComMatrixCreator.GetComDetailsListFromFile()
		o.Expect(err).ToNot(o.HaveOccurred())
		if isSNO {
			docComDetailsList = filterMasterNodeComDetailsFromList(docComDetailsList)
		}
		docComDetailsList = excludeBareMetalEntries(docComDetailsList)
		docComMatrix := &types.ComMatrix{Matrix: docComDetailsList}

		g.By("generating diff between matrices for testing purposes")
		diff, err := newComMatrix.GenerateMatrixDiff(docComMatrix)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(artifactsDir, "doc-diff-new"), []byte(diffFileComments+diff), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("comparing new and documented commatrices")
		areEqual := comMatricesAreEqual(*newComMatrix, *docComMatrix)
		o.Expect(areEqual).To(o.BeTrue())
	})
})

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(cs *clientutil.ClientSet) (bool, error) {
	nodes, err := cs.CoreV1Interface.Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	return len(nodes.Items) == 1, nil
}

// createCSVFromUrl creates a CSV from the given URL at fp filepath
func createCSVFromUrl(url string, fp string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
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

// filterMasterNodeComDetailsFromList filters and returns only comDetails with "master" as a Node roel
func filterMasterNodeComDetailsFromList(comDetails []types.ComDetails) []types.ComDetails {
	masterComDetails := []types.ComDetails{}
	for _, cd := range comDetails {
		if cd.NodeRole == "master" {
			masterComDetails = append(masterComDetails, cd)
		}
	}
	return masterComDetails
}

// excludeBareMetalEntries excludes and returns only comDetails that are not bare metal static entries
func excludeBareMetalEntries(comDetails []types.ComDetails) []types.ComDetails {
	nonBMComDetails := []types.ComDetails{}
	bmMasterStaticEntriesMatrix := &types.ComMatrix{Matrix: types.BaremetalStaticEntriesMaster}
	bmWorkerStaticEntriesMatrix := &types.ComMatrix{Matrix: types.BaremetalStaticEntriesWorker}
	for _, cd := range comDetails {
		switch cd.NodeRole {
		case "master":
			if !bmMasterStaticEntriesMatrix.Contains(cd) {
				nonBMComDetails = append(nonBMComDetails, cd)
			}
		case "worker":
			if !bmWorkerStaticEntriesMatrix.Contains(cd) {
				nonBMComDetails = append(nonBMComDetails, cd)
			}
		}
	}
	return nonBMComDetails
}