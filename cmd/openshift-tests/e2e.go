package main

import (
	"strings"
	"time"

	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/test/ginkgo"

	_ "github.com/openshift/origin/test/extended"
	_ "github.com/openshift/origin/test/extended/util/annotate/generated"
)

// staticSuites are all known test suites this binary should run
var staticSuites = []*ginkgo.TestSuite{
	{
		Name: "openshift/conformance",
		Description: templates.LongDesc(`
		Tests that ensure an OpenShift cluster and components are working properly.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/conformance/parallel",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run in parallel.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/parallel")
		},
		Parallelism:          30,
		MaximumAllowedFlakes: 15,
	},
	{
		Name: "openshift/conformance/serial",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run serially.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/serial")
		},
	},
	{
		Name: "openshift/disruptive",
		Description: templates.LongDesc(`
		The disruptive test suite.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Disruptive]") && strings.Contains(name, "[dr-quorum-restore]")
		},
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "kubernetes/conformance",
		Description: templates.LongDesc(`
		The default Kubernetes conformance suite.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:k8s]") && strings.Contains(name, "[Conformance]")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/build",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift build functionality.
		`),
		Matches: func(name string) bool {
			return !strings.Contains(name, "[Disabled") && strings.Contains(name, "[Feature:Builds]")
		},
		Parallelism: 7,
		// TODO: Builds are really flaky right now, remove when we land perf updates and fix io on workers
		MaximumAllowedFlakes: 3,
		// Jenkins tests can take a really long time
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/image-registry",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift image-registry functionality.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[registry]") && !strings.Contains(name, "[Local]")
		},
	},
	{
		Name: "openshift/image-ecosystem",
		Description: templates.LongDesc(`
		Tests that exercise language and tooling images shipped as part of OpenShift.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[image_ecosystem]") && !strings.Contains(name, "[Local]")
		},
		Parallelism: 7,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Feature:Jenkins]")
		},
		Parallelism: 4,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/scalability",
		Description: templates.LongDesc(`
		Tests that verify the scalability characteristics of the cluster. Currently this is focused on core performance behaviors and preventing regressions.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/scalability]")
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/conformance-excluded",
		Description: templates.LongDesc(`
		Run only tests that are excluded from conformance. Makes identifying omitted tests easier.
		`),
		Matches: func(name string) bool { return !strings.Contains(name, "[Suite:openshift/conformance/") },
	},
	{
		Name: "openshift/test-cmd",
		Description: templates.LongDesc(`
		Run only tests for test-cmd.
		`),
		Matches: func(name string) bool { return strings.Contains(name, "[Suite:openshift/test-cmd]") },
	},
	{
		Name: "openshift/csi",
		Description: templates.LongDesc(`
		Run tests for an installed CSI driver. TEST_CSI_DRIVER_FILES env. variable must be set and it must be a comma separated list of CSI driver definition files.
        See https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/external/README.md for required format of the files.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/csi") && !strings.Contains(name, "[Disruptive]")
		},
	},
	{
		Name: "openshift/network/stress",
		Description: templates.LongDesc(`
		This test suite repeatedly verifies the networking function of the cluster in parallel to find flakes.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/") && strings.Contains(name, "[sig-network]")
		},
		Parallelism: 30,
		Count:       15,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool { return true },
	},
}
