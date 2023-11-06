package networking

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/liornoy/node-comm-lib/commatrix"
	clientutil "github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ss"
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	oc := exutil.NewCLI("commatrix")

	g.It("should cover all ports that the nodes are actually listening on", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")
		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("generating the commatrix")
		kubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			g.Fail("must set the KUBECONFIG environment variable")
		}

		comMatrix, err := commatrix.New(kubeconfig, "", commatrix.AWS)
		o.Expect(err).ToNot(o.HaveOccurred())

		cs, err := clientutil.New(kubeconfig)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("fetching open ports on nodes with ss")
		nodesList, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		tcpfile, err := os.OpenFile(path.Join(artifactsDir, "raw-ss-tcp"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		o.Expect(err).ToNot(o.HaveOccurred())
		defer tcpfile.Close()

		udpfile, err := os.OpenFile(path.Join(artifactsDir, "raw-ss-udp"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		o.Expect(err).ToNot(o.HaveOccurred())
		defer tcpfile.Close()

		nodesComDeatils := []types.ComDetails{}
		for _, n := range nodesList.Items {
			cds, err := ss.ToComDetails(oc, &n, tcpfile, udpfile)
			o.Expect(err).ToNot(o.HaveOccurred())

			nodesComDeatils = append(nodesComDeatils, cds...)
		}
		cleanedComDetails := types.RemoveDups(nodesComDeatils)
		expectedComMat := types.ComMatrix{Matrix: cleanedComDetails}

		diff1 := comMatrix.Diff(expectedComMat)
		diff2 := expectedComMat.Diff(*comMatrix)

		g.By("Writing test artifacts")
		comMatrixFileName := filepath.Join(artifactsDir, "communication-matrix.csv")
		err = os.WriteFile(comMatrixFileName, []byte(comMatrix.String()), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		ssMatrixFileName := filepath.Join(artifactsDir, "ss-generated-matrix.csv")
		err = os.WriteFile(ssMatrixFileName, []byte(expectedComMat.String()), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = os.WriteFile(filepath.Join(artifactsDir, "matrix-diff-ss"),
			[]byte(fmt.Sprintf("The following entries were found in the communication matrix, but not in the 'ss' output:\n%s", &diff1)),
			0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = os.WriteFile(filepath.Join(artifactsDir, "ss-diff-matrix"),
			[]byte(fmt.Sprintf("The following entries were found in the 'ss' output, but not in the communication matrix:\n%s", &diff2)),
			0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		if len(diff1.Matrix) > 0 {
			csv, err := diff1.ToCSV()
			o.Expect(err).ToNot(o.HaveOccurred())

			logrus.Warnf("Warning! the following ports are found in the communication matrix but not in the ss output: %s", string(csv))
		}

		if len(diff2.Matrix) > 0 {
			csv, err := diff2.ToCSV()
			o.Expect(err).ToNot(o.HaveOccurred())

			logrus.Warnf("Warning! the following ports are found in the ss output but not in the communication matrix: %s", string(csv))
		}
	})
})
