package main

import (
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/origin/pkg/test/ginkgo"

	_ "github.com/openshift/origin/test/extended"
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
		Parallelism: 30,
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
			return strings.Contains(name, "[Feature:Builds]")
		},
		Parallelism: 7,
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
		Name: "openshift/smoke-4",
		Description: templates.LongDesc(`
		Tests that verify a 4.X cluster (using the new operator based core) is ready. This
		suite will be removed in favor of openshift/conformance once all functionality is
		available.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/smoke-4]") && strings.Contains(name, "[Suite:openshift/conformance/")
		},
		AllowPassWithFlakes: true,
		Parallelism:         30,
	},
	{
		Name: "openshift/all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool { return true },
	},
	{
		Name: "kubernetes/all",
		Description: templates.LongDesc(`
		Run all Kubernetes tests.
		`),
		Matches: func(name string) bool { return strings.Contains(name, "[k8s.io]") },
	},
}
