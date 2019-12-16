package operators

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"path/filepath"
	"strings"
	"time"
)

var _ = g.Describe("[Feature:Platform] Service Catalog should", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("service-catalog", exutil.KubeConfigPath())
		// serviceCatalogWait is how long to wait for Service Catalog operations to be ready
		serviceCatalogWait  = 180 * time.Second
		err                 error
		buildPruningBaseDir = exutil.FixturePath("testdata", "service_catalog")
		upsBroker           = filepath.Join(buildPruningBaseDir, "ups-broker.yaml")
		upsDeployment       = filepath.Join(buildPruningBaseDir, "ups-deployment.yaml")
		upsService          = filepath.Join(buildPruningBaseDir, "ups-service.yaml")
		upsInstance         = filepath.Join(buildPruningBaseDir, "ups-instance.yaml")
		upsBinding          = filepath.Join(buildPruningBaseDir, "ups-binding.yaml")
	)

	enableServiceCatalogResources := []struct {
		object    string
		namespace string
		name      string
	}{
		{"servicecatalogapiserver", "openshift-service-catalog-apiserver", "apiserver"},
		{"servicecatalogcontrollermanager", "openshift-service-catalog-controller-manager", "controller-manager"},
	}
	// Enable Service Catalog
	g.BeforeEach(func() {
		g.By("Enable Service Catalog")
		for _, v := range enableServiceCatalogResources {
			_, err := oc.AsAdmin().Run("patch").Args(v.object, "cluster", "-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge").Output()
			if err != nil {
				e2e.Failf("Unable to create: %s, error:%v", v.object, err)
			}

			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args("pods", "-n", v.namespace, "-o=jsonpath={.items[0].status.phase}").Output()
				if err != nil && !strings.Contains(output, "out of bounds") {
					e2e.Failf("output string: %s", output)
					return false, err
				}
				if output == "Running" {
					e2e.Logf("%s works well!", v.object)
					return true, nil
				}
				return false, nil
			})

			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}, 120)

	// The below steps will cover OCP-24049, author: jiazha@redhat.com
	// Disable these servicecatalogoperator resource
	g.AfterEach(func() {
		g.By("delete this ups broker")
		output, err := oc.AsAdmin().Run("get").Args("clusterservicebroker").Output()
		if strings.Contains(output, "ups-broker") {
			_, err = oc.AsAdmin().Run("delete").Args("clusterservicebroker", "ups-broker").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err = oc.AsAdmin().Run("get").Args("clusterserviceclass").Output()
				if err != nil {
					e2e.Failf("Failed to delete ups-broker, error:%v", err)
					return false, err
				}
				if !strings.Contains(output, "user-provided-service") {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		for _, v := range enableServiceCatalogResources {
			g.By("disable " + v.object)
			_, err := oc.AsAdmin().Run("patch").Args(v.object, "cluster", "-p", `{"spec":{"managementState":"Removed"}}`, "--type=merge").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args("ns", v.namespace).Output()
				if err != nil {
					if strings.Contains(output, "not found") {
						e2e.Logf("Disable %s successfully.", v.object)
						return true, nil
					}
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}, 120)

	g.It("check basic usages: OCP-24062, OCP-24049, OCP-15600", func() {
		g.By("Check Service Catalog apiservice")
		err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("apiservices", "v1beta1.servicecatalog.k8s.io", "-o=jsonpath={.status.conditions[0].reason}").Output()
			if err != nil {
				if strings.Contains(output, "not found") {
					return false, nil
				}
				return false, err
			}
			if strings.Contains(output, "Passed") {
				e2e.Logf("v1beta1.servicecatalog.k8s.io apiservice works well!")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// The below steps will cover test case: OCP-15600, author: jiazha@redhat.com
		g.By("Deploy a fake broker")

		upsFiles := []string{upsDeployment, upsService, upsBroker}
		e2e.Logf("current namespace:%s", oc.Namespace())
		for _, v := range upsFiles {
			configFile, err := oc.AsAdmin().Run("process").Args("-f", v, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				err = oc.AsAdmin().Run("create").Args("-f", configFile).Execute()
				if err != nil {
					e2e.Failf("Failed to install:%v, error:%v", v, err)
					return false, err
				}
				e2e.Logf("Install:%v successfully", v)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("clusterservicebroker", "ups-broker", "-o=jsonpath={.status.conditions[0].status}").Output()
			if err != nil {
				e2e.Failf("Failed to install ups-broker, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "True") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		upsBrokers := []struct {
			file         string
			object       string
			resourceName string
			expect       string
		}{
			{upsInstance, "serviceinstance", "ups-instance", "ProvisionedSuccessfully"},
			{upsBinding, "servicebinding", "ups-binding", "InjectedBindResult"},
		}
		for _, item := range upsBrokers {
			g.By(fmt.Sprintf("create %s", item))
			configFile, err := oc.AsAdmin().Run("process").Args("-f", item.file, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				err = oc.AsAdmin().Run("create").Args("-f", configFile).Execute()
				if err != nil {
					e2e.Failf("Failed to install:%v, error:%v", item.file, err)
					return false, err
				}
				e2e.Logf("Install:%v successfully", item.file)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err := oc.AsAdmin().Run("get").Args(item.object, "-n", oc.Namespace(), item.resourceName, "-o=jsonpath={.status.conditions[0].reason}").Output()
				if err != nil && !strings.Contains(output, "executing jsonpath") {
					e2e.Failf("Failed to install %s, error:%v", item.resourceName, err)
					return false, err
				}
				if strings.Contains(output, item.expect) {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		output, err := oc.AsAdmin().Run("get").Args("secret", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("my-secret"))

		g.By("unbind from this instance")
		output, err = oc.AsAdmin().Run("delete").Args("servicebinding", "-n", oc.Namespace(), "ups-binding").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("servicebinding", "-n", oc.Namespace()).Output()
			if err != nil {
				e2e.Failf("Failed to delete servicebinding, error:%v", err)
				return false, err
			}
			if !strings.Contains(output, "ups-binding") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.AsAdmin().Run("get").Args("secret", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring("my-secret"))

		g.By("delete this ups instance")
		_, err = oc.AsAdmin().Run("delete").Args("serviceinstance", "-n", oc.Namespace(), "ups-instance").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
			output, err = oc.AsAdmin().Run("get").Args("serviceinstance", "-n", oc.Namespace()).Output()
			if err != nil {
				e2e.Failf("Failed to delete serviceinstance, error:%v", err)
				return false, err
			}
			if !strings.Contains(output, "ups-instance") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// The below steps will cover OCP-24062, author: jiazha@redhat.com
		// TODO: once https://bugzilla.redhat.com/show_bug.cgi?id=1712297 fix, we should cover the `Force` managementState
		// Unmanage these servicecatalogoperator resource
		g.By("set Service Catalog to Unmanaged")
		for _, v := range enableServiceCatalogResources {
			_, err := oc.AsAdmin().Run("patch").Args(v.object, "cluster", "-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type=merge").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err = oc.AsAdmin().Run("get").Args(v.object, "-n", v.namespace, "cluster", "-o=jsonpath={.spec.managementState}").Output()
				if err != nil && !strings.Contains(output, "executing jsonpath") {
					e2e.Failf("Failed to set the Unmanaged for Service Catalog, error:%v", err)
					return false, err
				}
				if strings.Contains(output, "Unmanaged") {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			output, err := oc.AsAdmin().Run("patch").Args("daemonset", v.name, "-n", v.namespace, "-p", `{"spec":{"template":{"spec":{"priorityClassName":"test"}}}}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("patched"))

			err = wait.Poll(3*time.Second, serviceCatalogWait, func() (bool, error) {
				output, err = oc.AsAdmin().Run("get").Args("daemonset", v.name, "-n", v.namespace, "-o=jsonpath={.spec.template.spec.priorityClassName}").Output()
				if err != nil && !strings.Contains(output, "executing jsonpath") {
					e2e.Failf("Failed to check the priority of the daemonset, error:%v", err)
					return false, err
				}
				if strings.Contains(output, "test") {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

})
