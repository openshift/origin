package run_upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/monitortestframework"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/clioptions/upgradeoptions"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/version"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// TODO collapse this with cmd_runsuite
type RunUpgradeSuiteOptions struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	Suite                 *testginkgo.TestSuite

	ToImage        string
	FromRepository string
	// I don't see where this is initialized in this flow
	// CloudProviderJSON string

	TestOptions []string

	CloseFn iooptions.CloseFunc

	genericclioptions.IOStreams
}

func (o *RunUpgradeSuiteOptions) TestCommandEnvironment() []string {
	var args []string
	args = append(args, "KUBE_TEST_REPO_LIST=") // explicitly prevent selective override
	args = append(args, fmt.Sprintf("KUBE_TEST_REPO=%s", o.FromRepository))
	// args = append(args, fmt.Sprintf("TEST_PROVIDER=%s", o.CloudProviderJSON))  I don't think we actually have this.
	args = append(args, fmt.Sprintf("TEST_JUNIT_DIR=%s", o.GinkgoRunSuiteOptions.JUnitDir))
	for i := 10; i > 0; i-- {
		if klog.V(klog.Level(i)).Enabled() {
			args = append(args, fmt.Sprintf("TEST_LOG_LEVEL=%d", i))
			break
		}
	}

	upgradeOptions := upgradeoptions.UpgradeOptions{
		Suite:       o.Suite.Name,
		ToImage:     o.ToImage,
		TestOptions: o.TestOptions,
	}
	args = append(args, fmt.Sprintf("TEST_UPGRADE_OPTIONS=%s", upgradeOptions.ToEnv()))

	return args
}

// UpgradeTestPreSuite validates the test options and gathers data useful prior to launching the upgrade and it's
// related tests.
func (o *RunUpgradeSuiteOptions) UpgradeTestPreSuite() error {
	config, err := clusterdiscovery.DecodeProvider(os.Getenv("TEST_PROVIDER"), o.GinkgoRunSuiteOptions.DryRun, false, nil)
	if err != nil {
		return err
	}
	if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, config, o.GinkgoRunSuiteOptions.DryRun); err != nil {
		return err
	}
	klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

	return nil
}

func (o *RunUpgradeSuiteOptions) Run(ctx context.Context) error {
	defer o.CloseFn()

	// set globals so that helpers will create pods with the mapped images if we create them from this process.
	// this must be before `verifyImages` to ensure that the argument takes precedence over the env var.
	// we cannot eliminate the env var usage until we convert run-test, which we may be able to do in a followup.
	image.InitializeImages(o.FromRepository)

	if err := imagesetup.VerifyTestImageRepoEnvVarUnset(); err != nil {
		return err
	}
	if err := imagesetup.VerifyImages(); err != nil {
		return err
	}

	// this env var must be set to trigger the upstream code to resolve images in this binary.  See usage here
	// https://github.com/kubernetes/kubernetes/blob/99190634ab252604a4496882912ac328542d649d/test/utils/image/manifest.go#L282-L284
	if err := os.Setenv("KUBE_TEST_REPO", o.FromRepository); err != nil {
		return err
	}
	// we now re-trigger the upstream image determination since one of the env vars is set with our repo value.
	// this will re-write the images to be used.
	// TODO fix the upstream so that the AfterReadingAllFlags will properly check for either of the inputs having values.
	k8simage.Init("")

	if err := o.UpgradeTestPreSuite(); err != nil {
		return err
	}

	// TODO the gingkoRunSuiteOptions needs to have flags then calculated options to express specified versus computed values
	monitorTestInfo := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest:        monitortestframework.Stable,
		UpgradeTargetPayloadImagePullSpec: o.ToImage,
		ExactMonitorTests:                 o.GinkgoRunSuiteOptions.ExactMonitorTests,
		DisableMonitorTests:               o.GinkgoRunSuiteOptions.DisableMonitorTests,
	}

	o.GinkgoRunSuiteOptions.CommandEnv = o.TestCommandEnvironment()
	if !o.GinkgoRunSuiteOptions.DryRun {
		fmt.Fprintf(os.Stderr, "%s version: %s\n", filepath.Base(os.Args[0]), version.Get().String())
	}
	exitErr := o.GinkgoRunSuiteOptions.Run(o.Suite, nil, "openshift-tests-upgrade", monitorTestInfo, true)
	if exitErr != nil {
		fmt.Fprintf(os.Stderr, "Suite run returned error: %s\n", exitErr.Error())
	}

	return exitErr
}
