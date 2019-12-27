package operators

import (
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Platform] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	operators := "operators.coreos.com"
	providedAPIs := []struct {
		fromAPIService bool
		group          string
		version        string
		plural         string
	}{
		{
			fromAPIService: true,
			group:          "packages." + operators,
			version:        "v1",
			plural:         "packagemanifests",
		},
		{
			group:   operators,
			version: "v1",
			plural:  "operatorgroups",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "clusterserviceversions",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "catalogsources",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "installplans",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "subscriptions",
		},
	}

	for i := range providedAPIs {
		api := providedAPIs[i]
		g.It(fmt.Sprintf("be installed with %s at version %s", api.plural, api.version), func() {
			if api.fromAPIService {
				// Ensure spec.version matches expected
				raw, err := oc.AsAdmin().Run("get").Args("apiservices", fmt.Sprintf("%s.%s", api.version, api.group), "-o=jsonpath={.spec.version}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.Equal(api.version))
			} else {
				// Ensure expected version exists in spec.versions and is both served and stored
				raw, err := oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath={.spec.versions[?(@.name==\"%s\")]}", api.version)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.ContainSubstring("served:true"))
				o.Expect(raw).To(o.ContainSubstring("storage:true"))
			}
		})
	}

	// OCP-24061 - [bz 1685230] OLM operator should use imagePullPolicy: IfNotPresent
	// author: bandrade@redhat.com
	g.It("have imagePullPolicy:IfNotPresent on thier deployments", func() {
		deploymentResource := []string{"catalog-operator", "olm-operator", "packageserver"}
		for _, v := range deploymentResource {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "deployment", v, "-o=jsonpath={.spec.template.spec.containers[*].imagePullPolicy}").Output()
			e2e.Logf("%s.imagePullPolicy:%s", v, msg)
			if err != nil {
				e2e.Failf("Unable to get %s, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.Equal("IfNotPresent"))
		}
	})

	// OCP-21082 - Implement packages API server and list packagemanifest info with namespace not NULL
	// author: bandrade@redhat.com
	g.It("Implement packages API server and list packagemanifest info with namespace not NULL", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		packageserverLines := strings.Split(msg, "\n")
		if len(packageserverLines) > 0 {
			packageserverLine := strings.Fields(packageserverLines[0])
			if strings.Index(packageserverLines[0], packageserverLine[0]) != 0 {
				e2e.Failf("It should display a namespace for CSV: %s [ref:bz1670311]", packageserverLines[0])
			}
		} else {
			e2e.Failf("No packages for evaluating if package namespace is not NULL")
		}
	})

	// OCP-24057 - Check OLM pods termination message
	// author: bandrade@redhat.com
	g.It("Have terminationMessagePolicy defined as FallbackToLogsOnError", func() {
		msg, err := oc.SetNamespace("openshift-operator-lifecycle-manager").AsAdmin().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].name}{\"\t\"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		amountOfContainers := len(strings.Split(msg, "\t"))

		msg, err = oc.SetNamespace("openshift-operator-lifecycle-manager").AsAdmin().Run("get").Args("pods", "-o=jsonpath={range .items[*].spec}{.containers[*].terminationMessagePolicy}{\"t\"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		regexp := regexp.MustCompile("FallbackToLogsOnError")
		amountOfContainersWithFallbackToLogsOnError := len(regexp.FindAllStringIndex(msg, -1))
		o.Expect(amountOfContainers).To(o.Equal(amountOfContainersWithFallbackToLogsOnError))
		if amountOfContainers != amountOfContainersWithFallbackToLogsOnError {
			e2e.Failf("OLM does not have all containers definied with FallbackToLogsOnError terminationMessagePolicy")
		}
	})

})
