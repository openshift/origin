package networking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/types"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-kni/commatrix/pkg/utils"
	exutil "github.com/openshift/origin/test/extended/util"

	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	listeningsockets "github.com/openshift-kni/commatrix/pkg/listening-sockets"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

const (
	serviceNodePortMin = 30000
	serviceNodePortMax = 32767
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("should validate the communication matrix ports match the node's listening ports", func() {
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

		utilsHelpers := utils.New(cs)

		g.By("generate the communication matrix")
		epExporter, err := endpointslices.New(cs)
		o.Expect(err).ToNot(o.HaveOccurred())

		commMatrix, err := commatrixcreator.New(epExporter, "", "", env, deployment)
		o.Expect(err).ToNot(o.HaveOccurred())

		matrix, err := commMatrix.CreateEndpointMatrix()
		o.Expect(err).ToNot(o.HaveOccurred())

		err = matrix.WriteMatrixToFileByType(utilsHelpers, "communication-matrix", types.FormatCSV, deployment, artifactsDir)
		o.Expect(err).ToNot(o.HaveOccurred())

		listeningCheck, err := listeningsockets.NewCheck(cs, utilsHelpers, artifactsDir)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("generate the ss matrix and ss raws")
		ssMat, ssOutTCP, ssOutUDP, err := listeningCheck.GenerateSS()
		o.Expect(err).ToNot(o.HaveOccurred())

		err = listeningCheck.WriteSSRawFiles(ssOutTCP, ssOutUDP)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = ssMat.WriteMatrixToFileByType(utilsHelpers, "ss-generated-matrix", types.FormatCSV, deployment, artifactsDir)
		o.Expect(err).ToNot(o.HaveOccurred())

		// generate the diff matrix between the enpointslice and the ss matrix
		ssFilteredMat, err := filterSSMatrix(configClient, ssMat)
		o.Expect(err).ToNot(o.HaveOccurred())

		diff := matrixdiff.Generate(matrix, ssFilteredMat)
		diffStr, err := diff.String()
		o.Expect(err).ToNot(o.HaveOccurred())

		err = utilsHelpers.WriteFile(filepath.Join(artifactsDir, "matrix-diff-ss"), []byte(diffStr))
		o.Expect(err).ToNot(o.HaveOccurred())

		notUsedEPSMat := diff.GenerateUniquePrimary()
		if len(notUsedEPSMat.Matrix) > 0 {
			logrus.Warningf("the following ports are not used: \n %s", notUsedEPSMat)
		}

		missingEPSMat := diff.GenerateUniqueSecondary()
		if len(missingEPSMat.Matrix) > 0 {
			err := fmt.Errorf("the following ports are used but don't have an endpointslice: \n %s", missingEPSMat)
			o.Expect(err).ToNot(o.HaveOccurred())
		}
	})
})

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(oc *configv1client.ConfigV1Client) (bool, error) {
	infrastructureType, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	logrus.Infof("the cluster type is %s", infrastructureType.Status.ControlPlaneTopology)
	return infrastructureType.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

// isBMCluster will check if OCP is running on a BareMetal platform
func isBMCluster(oc *configv1client.ConfigV1Client) (bool, error) {
	infrastructureType, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	logrus.Infof("the cluster platform is %s", infrastructureType.Status.PlatformStatus.Type)
	return infrastructureType.Status.PlatformStatus.Type == configv1.BareMetalPlatformType, nil
}

// Filter ss known ports to skip in matrix diff
func filterSSMatrix(oc *configv1client.ConfigV1Client, mat *types.ComMatrix) (*types.ComMatrix, error) {
	nodePortMin := serviceNodePortMin
	nodePortMax := serviceNodePortMax

	clusterNetwork, err := oc.Networks().Get(context.TODO(), "cluster", metav1.GetOptions{})
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

		res = append(res, cd)
	}

	return &types.ComMatrix{Matrix: res}, nil
}
