package machine_config

import (
	"context"
	"path/filepath"

	osconfigv1 "github.com/openshift/api/config/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

// This test is [Serial] because it modifies the cluster/machineconfigurations.operator.openshift.io object in each test.
var _ = g.Describe("[sig-mco][OCPFeatureGate:ManagedBootImages][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigurationBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigurations")
		partialMachineSetFixture       = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-partial.yaml")
		allMachineSetFixture           = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-all.yaml")
		noneMachineSetFixture          = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-none.yaml")
		oc                             = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip this test if not on GCP platform
		skipUnlessTargetPlatform(oc, osconfigv1.GCPPlatformType)
		//skip this test if the cluster is not using MachineAPI
		skipUnlessFunctionalMachineAPI(oc)
		//skip this test on single node platforms
		skipOnSingleNodeTopology(oc)
	})

	g.AfterEach(func() {
		// Clear out boot image configuration between tests
		err := oc.Run("apply").Args("-f", noneMachineSetFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should update boot images only on MachineSets that are opted in [apigroup:machineconfiguration.openshift.io]", func() {
		PartialMachineSetTest(oc, partialMachineSetFixture)
	})

	g.It("Should update boot images on all MachineSets when configured [apigroup:machineconfiguration.openshift.io]", func() {
		AllMachineSetTest(oc, allMachineSetFixture)
	})

	g.It("Should not update boot images on any MachineSet when not configured [apigroup:machineconfiguration.openshift.io]", func() {
		NoneMachineSetTest(oc, noneMachineSetFixture)
	})

	g.It("Should degrade on a MachineSet with an OwnerReference [apigroup:machineconfiguration.openshift.io]", func() {
		DegradeOnOwnerRefTest(oc, allMachineSetFixture)
	})

	g.It("Should stamp coreos-bootimages configmap with current MCO hash and release version [apigroup:machineconfiguration.openshift.io]", func() {
		EnsureConfigMapStampTest(oc, allMachineSetFixture)
	})
})
