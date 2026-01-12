package networking

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/openshift-kni/commatrix/pkg/types"
	configv1 "github.com/openshift/api/config/v1"
	clientOptions "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	listeningsockets "github.com/openshift-kni/commatrix/pkg/listening-sockets"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/utils"
)

const (
	commatrixArfticatFolder = "commatrix-e2e"
	testNS                  = "openshift-commatrix-test"
	serviceNodePortMin      = 30000
	serviceNodePortMax      = 32767
)

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

		ipv6Enabled, err := utilsHelpers.IsIPv6Enabled()
		Expect(err).NotTo(HaveOccurred())

		By("Generating cluster's communication matrix creator")
		commMatrixCreator, err = commatrixcreator.New(epExporter, "", "", infraType, deployment, ipv6Enabled)
		Expect(err).NotTo(HaveOccurred())

		By("Create endpoint matrix ")
		commatrix, err = commMatrixCreator.CreateEndpointMatrix()
		Expect(err).NotTo(HaveOccurred())

		err = commatrix.WriteMatrixToFileByType(utilsHelpers, "communication-matrix", types.FormatCSV, deployment, artifactsDir)
		Expect(err).ToNot(HaveOccurred())
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
