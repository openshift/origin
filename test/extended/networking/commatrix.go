package networking

import (
	"fmt"
	"log"

	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
)

const (
	commatrixArfticatFolder = "commatrix-e2e"
)

var _ = g.Describe("[sig-network][Feature:commatrix][apigroup:config.openshift.io][Serial]", func() {
	g.It("Static entries should not overlap with those in the EndpointSlice; any shared entries must be removed", func() {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			g.Fail("KUBECONFIG not set")
		}

		g.By("Creating output folder")
		artifactsDir := os.Getenv("ARTIFACT_DIR")
		if artifactsDir == "" {
			log.Println("env var ARTIFACT_DIR is not set, using default value")
		}
		artifactsDir = filepath.Join(artifactsDir, commatrixArfticatFolder)

		err := os.MkdirAll(artifactsDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating the clients for the Generating step")
		cs, err := client.New()
		o.Expect(err).NotTo(o.HaveOccurred())

		utilsHelpers := utils.New(cs)
		epExporter, err := endpointslices.New(cs)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get cluster's deployment and infrastructure types")
		deployment := types.Standard
		isSNO, err := utilsHelpers.IsSNOCluster()
		o.Expect(err).NotTo(o.HaveOccurred())

		if isSNO {
			deployment = types.SNO
		}

		platformType, err := utilsHelpers.GetPlatformType()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Generating cluster's communication matrix")
		commMatrixCreator, err := commatrixcreator.New(epExporter, "", "", platformType, deployment)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = epExporter.LoadExposedEndpointSlicesInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		epSliceComDetails, err := epExporter.ToComDetails()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Get static entries list")
		staticEntries, err := commMatrixCreator.GetStaticEntries()
		o.Expect(err).NotTo(o.HaveOccurred())

		staticEntriesMat := &types.ComMatrix{Matrix: staticEntries}
		epSliceComDetailsMat := &types.ComMatrix{Matrix: epSliceComDetails}

		g.By("Write the matrix to files")
		err = staticEntriesMat.WriteMatrixToFileByType(utilsHelpers, "static-entry-matrix", types.FormatCSV, deployment, artifactsDir)
		o.Expect(err).ToNot(o.HaveOccurred())

		err = epSliceComDetailsMat.WriteMatrixToFileByType(utilsHelpers, "expose-communication-matrix", types.FormatCSV, deployment, artifactsDir)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Generating the Diff between the static entris and the expose communication matrix")
		endpointslicesDiffWithstaticEntrieMat := matrixdiff.Generate(epSliceComDetailsMat, staticEntriesMat)
		staticEntryNeedToRemove := endpointslicesDiffWithstaticEntrieMat.GetSharedEntries()
		if len(staticEntryNeedToRemove.Matrix) > 0 {
			err := fmt.Errorf("the following ports must be removed from the static entry file, as they already exist in an EndpointSlice:\n%s", staticEntryNeedToRemove)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
