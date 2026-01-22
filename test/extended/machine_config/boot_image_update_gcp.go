package machine_config

import (
	"context"
	"path/filepath"

	osconfigv1 "github.com/openshift/api/config/v1"

	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
)

// These tests are `Serial` because they all modify the cluster/machineconfigurations.operator.openshift.io object.
var _ = g.Describe("[sig-mco][OCPFeatureGate:ManagedBootImages][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigurationBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigurations")
		partialMachineSetFixture       = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-partial.yaml")
		allMachineSetFixture           = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-all.yaml")
		noneMachineSetFixture          = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-none.yaml")
		skewEnforcementDisabledFixture = filepath.Join(MCOMachineConfigurationBaseDir, "skewenforcement-disabled.yaml")
		emptyMachineSetFixture         = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-empty.yaml")
		oc                             = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip this test if not on GCP platform
		skipUnlessTargetPlatform(oc, osconfigv1.GCPPlatformType)
		//skip this test if the cluster is not using MachineAPI
		skipUnlessFunctionalMachineAPI(oc)
		//skip this test on single node platforms
		skipOnSingleNodeTopology(oc)
		// Disable boot image skew enforcement
		ApplyMachineConfigurationFixture(oc, skewEnforcementDisabledFixture)
	})

	g.AfterEach(func() {
		// Clear out boot image configuration between tests
		ApplyMachineConfigurationFixture(oc, emptyMachineSetFixture)
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
