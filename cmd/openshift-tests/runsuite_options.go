package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/version"
)

// TODO collapse this with cmd_runsuite
type RunSuiteOptions struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	Suite                 *testginkgo.TestSuite

	FromRepository    string
	CloudProviderJSON string

	genericclioptions.IOStreams
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
	if err := verifyImages(); err != nil {
		return err
	}

	o.GinkgoRunSuiteOptions.CommandEnv = o.TestCommandEnvironment()
	if !o.GinkgoRunSuiteOptions.DryRun {
		fmt.Fprintf(os.Stderr, "%s version: %s\n", filepath.Base(os.Args[0]), version.Get().String())
	}
	exitErr := o.GinkgoRunSuiteOptions.Run(o.Suite, "openshift-tests", false)
	if exitErr != nil {
		fmt.Fprintf(os.Stderr, "Suite run returned error: %s\n", exitErr.Error())
	}

	// Special debugging carve-outs for teams is likely to age poorly.
	clusterdiscovery.PrintStorageCapabilities(o.GinkgoRunSuiteOptions.Out)
	return exitErr
}
