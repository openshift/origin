package networking

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	clientutil "github.com/openshift-kni/commatrix/client"
	"github.com/openshift-kni/commatrix/commatrix"
	"github.com/openshift-kni/commatrix/types"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	doc_commatrix_url = "https://raw.githubusercontent.com/openshift/openshift-docs/main/snippets/network-flow-matrix.csv"
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("should be equal to documeneted communication matrix", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")
		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		kubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			g.Fail("must set the KUBECONFIG environment variable")
		}

		cs, err := clientutil.New(kubeconfig)
		o.Expect(err).ToNot(o.HaveOccurred())

		deployment := commatrix.MNO
		isSNO, err := isSNOCluster(cs.ConfigV1Interface)
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSNO {
			deployment = commatrix.SNO
		}

		g.By("generating new commatrix")
		newComMatrix, err := commatrix.New(kubeconfig, "", "", commatrix.Cloud, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())
		commatrix.WriteMatrixToFileByType(*newComMatrix, "new-commatrix", types.FormatCSV, deployment, artifactsDir)

		g.By("get documented commatrix")
		fp := filepath.Join(artifactsDir, "doc-commatrix.csv")
		createCSVFromUrl(doc_commatrix_url, fp)
		docComDetailsList, err := commatrix.GetComDetailsListFromFile(fp, types.CSV)
		o.Expect(err).ToNot(o.HaveOccurred())
		docComMatrix := &types.ComMatrix{Matrix: docComDetailsList}

		g.By("generating diff between matrices for testing purposes")
		diff, err := commatrix.GenerateMatrixDiff(*newComMatrix, *docComMatrix)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(artifactsDir, "matrix-diff-ss"), []byte(diff), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("comparing new and documented commatrices")
		areEqual := comMatricesAreEqual(*newComMatrix, *docComMatrix)
		o.Expect(areEqual).To(o.BeTrue())
	})
})

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(oc configv1client.ConfigV1Interface) (bool, error) {
	infrastructureType, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	logrus.Infof("the cluster type is %s", infrastructureType.Status.ControlPlaneTopology)
	return infrastructureType.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
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
