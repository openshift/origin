package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/monitortestframework"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/version"
	"github.com/openshift/origin/test/extended/util/image"
)

// TODO collapse this with cmd_runsuite
type RunSuiteOptions struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	Suite                 *testginkgo.TestSuite

	FromRepository    string
	CloudProviderJSON string

	CloseFn iooptions.CloseFunc
	genericclioptions.IOStreams

	// ClusterConfig contains cluster-specific configuration for filtering tests
	ClusterConfig *clusterdiscovery.ClusterConfiguration

	// Extension is the internal origin extension of its own test specs.
	Extension *extension.Extension
}

func (o *RunSuiteOptions) TestCommandEnvironment() []string {
	var args []string
	args = append(args, "KUBE_TEST_REPO_LIST=") // explicitly prevent selective override
	args = append(args, fmt.Sprintf("KUBE_TEST_REPO=%s", o.FromRepository))
	args = append(args, fmt.Sprintf("TEST_PROVIDER=%s", o.CloudProviderJSON))
	args = append(args, fmt.Sprintf("TEST_JUNIT_DIR=%s", o.GinkgoRunSuiteOptions.JUnitDir))
	for i := 10; i > 0; i-- {
		if klog.V(klog.Level(i)).Enabled() {
			args = append(args, fmt.Sprintf("TEST_LOG_LEVEL=%d", i))
			break
		}
	}
	args = append(args, "TEST_UPGRADE_OPTIONS=")

	return args
}

func (o *RunSuiteOptions) Run(ctx context.Context) error {
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
	// TODO fix the the upstream so that the AfterReadingAllFlags will properly check for either of the inputs having values.
	k8simage.Init("")

	stabilitySetting := testginkgo.Stable
	switch {
	case len(o.GinkgoRunSuiteOptions.ClusterStabilityDuringTest) > 0:
		stabilitySetting = testginkgo.ClusterStabilityDuringTest(o.GinkgoRunSuiteOptions.ClusterStabilityDuringTest)
	case len(o.Suite.ClusterStabilityDuringTest) > 0:
		stabilitySetting = o.Suite.ClusterStabilityDuringTest
	}

	monitorTestInfo := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest: monitortestframework.ClusterStabilityDuringTest(stabilitySetting),
		ExactMonitorTests:          o.GinkgoRunSuiteOptions.ExactMonitorTests,
		DisableMonitorTests:        o.GinkgoRunSuiteOptions.DisableMonitorTests,
	}

	o.GinkgoRunSuiteOptions.CommandEnv = o.TestCommandEnvironment()
	if !o.GinkgoRunSuiteOptions.DryRun {
		fmt.Fprintf(os.Stderr, "%s version: %s\n", filepath.Base(os.Args[0]), version.Get().String())
	}

	o.GinkgoRunSuiteOptions.Extension = o.Extension

	// ensure we run at least 1 time in the case only invocation was provided
	if o.GinkgoRunSuiteOptions.Invocations < o.GinkgoRunSuiteOptions.Invocation {
		o.GinkgoRunSuiteOptions.Invocations = o.GinkgoRunSuiteOptions.Invocation
	}
	// TEST ONLY
	o.GinkgoRunSuiteOptions.Invocations = 2

	exitErrs := o.GinkgoRunSuiteOptions.Run(o.Suite, o.ClusterConfig, "openshift-tests", o.GinkgoRunSuiteOptions.Invocation, o.GinkgoRunSuiteOptions.Invocations, monitorTestInfo, false)

	for i := range exitErrs {
		if exitErrs[i] != nil {
			fmt.Fprintf(os.Stderr, "Suite run (%d) returned error: %s\n", i, exitErrs[i].Error())
		}
	}

	return exitErrs[0]
}
