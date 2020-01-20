package clusterlogging

import (
	"fmt"
	"strconv"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	//CLO resource files to create subscription for cluster-logging-operator
	CLO = OperatorObjects{
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "01-clo-project.yaml"),
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "02-clo-og.yaml"),
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "04-clo-sub.yaml")}
	//EO resource files to create subscription for elasticsearch-operator
	EO = OperatorObjects{
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "01-eo-project.yaml"),
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "02-eo-og.yaml"),
		exutil.FixturePath("testdata", "clusterlogging", "deployment", "05-eo-sub.yaml")}
	rbac = exutil.FixturePath("testdata", "clusterlogging", "deployment", "04-eo-rbac.yaml")
)

var _ = g.Describe("[Feature:Logging][Serial] Logging", func() {
	defer g.GinkgoRecover()
	var (
		oc            = exutil.NewCLIWithoutNamespace("logging")
		loggingNs     = "openshift-logging"
		eoNs          = "openshift-operators-redhat"
		marketplaceNs = "openshift-marketplace"
	)
	g.Describe("Deploy Logging Via OLM: ", func() {
		g.BeforeEach(func() {
			g.By("Check packagemanifests", func() {
				err := oc.AsAdmin().Run("get").Args("-n", marketplaceNs, "packagemanifests", "cluster-logging").Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "Can't get packagemanifest cluster-logging")
				err = oc.AsAdmin().Run("get").Args("-n", marketplaceNs, "packagemanifests", "elasticsearch-operator").Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "Can't get packagemanifest elasticsearch-operator")
			})

			g.By("Creating subscription for cluster-logging-operator", func() {
				err := createLoggingResources(oc, CLO)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Creating subscription for elasticsearch-operator", func() {
				err := createLoggingResources(oc, EO)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().Run("create").Args("-f", rbac).Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create RBAC")
			})

			g.By("Check if the cluster-logging-operator is ready or not", func() {
				err := waitForDeployPodsToBeReady(oc, loggingNs, "cluster-logging-operator")
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Check if the elasticsearch-operator is ready or not", func() {
				err := waitForDeployPodsToBeReady(oc, eoNs, "elasticsearch-operator")
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})

		g.AfterEach(func() {
			err := deleteNamespace(oc, loggingNs)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = deleteNamespace(oc, eoNs)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = e2e.WaitForNamespacesDeleted(oc.AdminKubeClient(), []string{loggingNs, eoNs}, timeout)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.It("should deploy logging successfully when using fluentd as logcollector", func() {
			instance := exutil.FixturePath("testdata", "clusterlogging", "instances", "example.yaml")
			g.By("Creating clusterlogging instance", func() {
				err := oc.AsAdmin().Run("create").Args("-f", instance).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("waiting for the fluentd pods to be ready...", func() {
				err := waitForDaemonsetPodsToBeReady(oc, "fluentd", loggingNs)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("waiting for the kibana pod to be ready...", func() {
				err := waitForDeployPodsToBeReady(oc, loggingNs, "kibana")
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("waiting for the elasticsearch pod to be ready...", func() {
				output, err := oc.AsAdmin().Run("get").Args("-n", loggingNs, "clusterlogging", "instance", "-o=jsonpath={.spec.logStore.elasticsearch.nodeCount}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				msg := fmt.Sprintf("%v", output)
				escount, err := strconv.ParseInt(msg, 10, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
				deployName, err := getDeploymentsNameViaLabel(oc, loggingNs, "cluster-name=elasticsearch", int(escount))
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, name := range deployName {
					err = waitForDeployPodsToBeReady(oc, loggingNs, name)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			})

			g.By("check cronjob", func() {
				err := checkCronJob(oc, "curator", loggingNs)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("check CRDs", func() {
				err := checkResourcesCreatedByOperators(oc, loggingNs, "clusterlogging", "instance")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = checkResourcesCreatedByOperators(oc, loggingNs, "elasticsearch", "elasticsearch")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = checkResourcesCreatedByOperators(oc, loggingNs, "servicemonitor", "fluentd")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = checkResourcesCreatedByOperators(oc, loggingNs, "servicemonitor", "monitor-elasticsearch-cluster")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = checkResourcesCreatedByOperators(oc, loggingNs, "prometheusrule", "elasticsearch-prometheus-rules")
				o.Expect(err).NotTo(o.HaveOccurred())
				//err = checkResourcesCreatedByOperators(oc, loggingNs, "prometheusrule", "fluentd")
				//o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})

})
