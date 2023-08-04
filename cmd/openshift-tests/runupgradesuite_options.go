package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/test/ginkgo"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/version"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
)

// TODO collapse this with cmd_runsuite
type RunUpgradeSuiteOptions struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	Suite                 *testginkgo.TestSuite

	ToImage        string
	FromRepository string
	// I don't see where this is initialized in this flow
	//CloudProviderJSON string

	TestOptions []string

	CloseFn iooptions.CloseFunc

	genericclioptions.IOStreams
}

func (o *RunUpgradeSuiteOptions) TestCommandEnvironment() []string {
	var args []string
	args = append(args, "KUBE_TEST_REPO_LIST=") // explicitly prevent selective override
	args = append(args, fmt.Sprintf("KUBE_TEST_REPO=%s", o.FromRepository))
	//args = append(args, fmt.Sprintf("TEST_PROVIDER=%s", o.CloudProviderJSON))  I don't think we actually have this.
	args = append(args, fmt.Sprintf("TEST_JUNIT_DIR=%s", o.GinkgoRunSuiteOptions.JUnitDir))
	for i := 10; i > 0; i-- {
		if klog.V(klog.Level(i)).Enabled() {
			args = append(args, fmt.Sprintf("TEST_LOG_LEVEL=%d", i))
			break
		}
	}

	upgradeOptions := UpgradeOptions{
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
	if !o.GinkgoRunSuiteOptions.DryRun {
		testOpt := ginkgo.NewTestOptions(o.IOStreams)
		config, err := clusterdiscovery.DecodeProvider(os.Getenv("TEST_PROVIDER"), testOpt.DryRun, false, nil)
		if err != nil {
			return err
		}
		if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, config, testOpt.DryRun); err != nil {
			return err
		}
		klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

		if err := upgrade.GatherPreUpgradeResourceCounts(); err != nil {
			return errors.Wrap(err, "error gathering preupgrade resource counts")
		}
	}

	return SetUpgradeGlobalsFromTestOptions(o.TestOptions)
}

func (o *RunUpgradeSuiteOptions) Run(ctx context.Context) error {
	defer o.CloseFn()

	if err := verifyImages(); err != nil {
		return err
	}

	if err := o.UpgradeTestPreSuite(); err != nil {
		return err
	}

	o.GinkgoRunSuiteOptions.CommandEnv = o.TestCommandEnvironment()
	if !o.GinkgoRunSuiteOptions.DryRun {
		fmt.Fprintf(os.Stderr, "%s version: %s\n", filepath.Base(os.Args[0]), version.Get().String())
	}
	exitErr := o.GinkgoRunSuiteOptions.Run(o.Suite, "openshift-tests-upgrade", true)
	if exitErr != nil {
		fmt.Fprintf(os.Stderr, "Suite run returned error: %s\n", exitErr.Error())
	}

	return exitErr
}
