package networking

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/openshift-kni/commatrix/commatrix"
	"github.com/openshift-kni/commatrix/consts"
	"github.com/openshift-kni/commatrix/debug"

	clientutil "github.com/openshift-kni/commatrix/client"
	"github.com/openshift-kni/commatrix/ss"
	"github.com/openshift-kni/commatrix/types"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	serviceNodePortMin = 30000
	serviceNodePortMax = 32767
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("should cover all ports that the nodes are actually listening on", func() {
		artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")
		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("generating the commatrix")
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

		comMatrix, err := commatrix.New(kubeconfig, "", "", commatrix.Cloud, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("fetching open ports on nodes with ss")
		nodesList, err := cs.CoreV1Interface.Nodes().List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		tcpFile, err := os.OpenFile(path.Join(artifactsDir, "raw-ss-tcp"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		o.Expect(err).ToNot(o.HaveOccurred())
		defer tcpFile.Close()

		udpFile, err := os.OpenFile(path.Join(artifactsDir, "raw-ss-udp"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		o.Expect(err).ToNot(o.HaveOccurred())
		defer udpFile.Close()

		err = debug.CreateNamespace(cs, consts.DefaultDebugNamespace)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := debug.DeleteNamespace(cs, consts.DefaultDebugNamespace)
			if err != nil {
				panic(err)
			}
		}()

		nodesComDetails := []types.ComDetails{}
		nLock := &sync.Mutex{}
		eg := new(errgroup.Group)
		for _, n := range nodesList.Items {
			node := n
			eg.Go(func() error {
				debugPod, err := debug.New(cs, node.Name, consts.DefaultDebugNamespace, consts.DefaultDebugPodImage)
				if err != nil {
					return err
				}
				defer func() {
					err := debugPod.Clean()
					if err != nil {
						fmt.Printf("failed cleaning debug pod %s: %v", debugPod, err)
					}
				}()

				cds, err := ss.CreateComDetailsFromNode(debugPod, &node, tcpFile, udpFile)
				if err != nil {
					return err
				}
				nLock.Lock()
				nodesComDetails = append(nodesComDetails, cds...)
				nLock.Unlock()
				return nil
			})
		}

		err = eg.Wait()
		o.Expect(err).ToNot(o.HaveOccurred())

		cleanedComDetails := types.CleanComDetails(nodesComDetails)
		ssComMat := types.ComMatrix{Matrix: cleanedComDetails}

		g.By("Writing test artifacts")
		res, err := types.ToCSV(*comMatrix)
		o.Expect(err).ToNot(o.HaveOccurred())
		comMatrixFileName := filepath.Join(artifactsDir, "communication-matrix.csv")
		err = os.WriteFile(comMatrixFileName, []byte(string(res)), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		res, err = types.ToCSV(ssComMat)
		o.Expect(err).ToNot(o.HaveOccurred())
		ssMatrixFileName := filepath.Join(artifactsDir, "ss-generated-matrix.csv")
		err = os.WriteFile(ssMatrixFileName, []byte(string(res)), 0644)
		o.Expect(err).ToNot(o.HaveOccurred())

		diff := buildMatrixDiff(*comMatrix, ssComMat, cs)

		err = os.WriteFile(filepath.Join(artifactsDir, "matrix-diff-ss"),
			[]byte(diff),
			0644)
		if err != nil {
			panic(err)
		}

		diff1 := comMatrix.Diff(ssComMat)
		diff2 := ssComMat.Diff(*comMatrix)

		if len(diff1.Matrix) > 0 {
			csv, err := types.ToCSV(diff1)
			o.Expect(err).ToNot(o.HaveOccurred())

			logrus.Warnf("the following ports are found in the communication matrix but not in the ss output: %s", string(csv))
		}

		if len(diff2.Matrix) > 0 {
			csv, err := types.ToCSV(diff2)
			o.Expect(err).ToNot(o.HaveOccurred())

			logrus.Warnf("the following ports are found in the ss output but not in the communication matrix: %s", string(csv))
		}
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

func buildMatrixDiff(mat1 types.ComMatrix, mat2 types.ComMatrix, cs *clientutil.ClientSet) string {
	nodePortMin := serviceNodePortMin
	nodePortMax := serviceNodePortMax

	clusterNetwork, err := cs.ConfigV1Interface.Networks().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	serviceNodePortRange := clusterNetwork.Spec.ServiceNodePortRange
	if serviceNodePortRange != "" {
		rangeStr := strings.Split(serviceNodePortRange, "-")
		nodePortMin, err = strconv.Atoi(rangeStr[0])
		if err != nil {
			panic(err)
		}
		nodePortMax, err = strconv.Atoi(rangeStr[1])
		if err != nil {
			panic(err)
		}
	}

	diff := ""
	for _, cd := range mat1.Matrix {
		if mat2.Contains(cd) {
			diff += fmt.Sprintf("%s\n", cd)
			continue
		}

		diff += fmt.Sprintf("+ %s\n", cd)
	}

	for _, cd := range mat2.Matrix {
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

		if !mat1.Contains(cd) {
			diff += fmt.Sprintf("- %s\n", cd)
		}
	}

	return diff
}
