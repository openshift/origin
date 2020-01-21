package isv

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type packagemanifest struct {
	name                    string
	supportsOwnNamespace    bool
	supportsSingleNamespace bool
	supportsAllNamespaces   bool
	csvVersion              string
	namespace               string
	defaultChannel          string
	catalogSource           string
	catalogSourceNamespace  string
}

var _ = g.Describe("[Suite:openshift/isv] Operator", func() {
	var (
		oc                = exutil.NewCLI("isv", exutil.KubeConfigPath())
		catalogLabels     = []string{"redhat-operators", "certified-operators"}
		output, _         = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-l catalog="+catalogLabels[0], "-o=jsonpath={range .items[*].metadata}{.name}{'\\n'}{end}").Output()
		redhatPackages    = strings.Split(output, "\n")
		output1, _        = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-l catalog="+catalogLabels[1], "-o=jsonpath={range .items[*].metadata}{.name}{'\\n'}{end}").Output()
		certifiedPackages = strings.Split(output1, "\n")
		packages          = append(redhatPackages, certifiedPackages...)
		//packages       = []string{"serverless-operator"}
		currentPackage packagemanifest
	)
	defer g.GinkgoRecover()

	g.AfterEach(func() {
		if currentPackage.supportsSingleNamespace || currentPackage.supportsOwnNamespace {
			_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", currentPackage.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	for i := range packages {

		isv := packages[i]
		g.It(isv+" should work properly", func() {
			g.By("by installing", func() {
				currentPackage = createSubscription(isv, oc)
				checkDeployment(currentPackage, oc)
			})
			g.By("by unistalling", func() {
				removeOperatorDependencies(currentPackage, oc, true)
			})
		})
	}

})

func checkOperatorInstallModes(p packagemanifest, oc *exutil.CLI) packagemanifest {
	supportsAllNamespaces, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", p.name, "-o=jsonpath={.status.channels[?(.name=='"+p.defaultChannel+"')].currentCSVDesc.installModes[?(.type=='AllNamespaces')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsAllNamespacesAsBool, _ := strconv.ParseBool(supportsAllNamespaces)

	supportsSingleNamespace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", p.name, "-o=jsonpath={.status.channels[?(.name=='"+p.defaultChannel+"')].currentCSVDesc.installModes[?(.type=='SingleNamespace')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsSingleNamespaceAsBool, _ := strconv.ParseBool(supportsSingleNamespace)

	supportsOwnNamespace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", p.name, "-o=jsonpath={.status.channels[?(.name=='"+p.defaultChannel+"')].currentCSVDesc.installModes[?(.type=='OwnNamespace')].supported}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	supportsOwnNamespaceAsBool, _ := strconv.ParseBool(supportsOwnNamespace)

	p.supportsAllNamespaces = supportsAllNamespacesAsBool
	p.supportsSingleNamespace = supportsSingleNamespaceAsBool
	p.supportsOwnNamespace = supportsOwnNamespaceAsBool
	return p
}

func createSubscription(isv string, oc *exutil.CLI) packagemanifest {

	msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", isv, "-o=jsonpath={.status.catalogSource}:{.status.catalogSourceNamespace}:{.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	packageData := strings.Split(msg, ":")
	p := packagemanifest{catalogSource: packageData[0], catalogSourceNamespace: packageData[1], defaultChannel: packageData[2], name: isv}

	csvVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", p.name, "-o=jsonpath={.status.channels[?(.name=='"+p.defaultChannel+"')].currentCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	p.csvVersion = csvVersion

	p = checkOperatorInstallModes(p, oc)

	if p.supportsSingleNamespace || p.supportsOwnNamespace {
		p = createNamespace(p, oc)
		createOperatorGroup(p, oc)
	} else if p.supportsAllNamespaces {
		p.namespace = "openshift-operators"
	} else {
		g.Skip("Install Modes AllNamespaces and  SingleNamespace are disabled for Operator: " + isv)
	}

	templateSubscriptionYAML := writeSubscription(p)
	_, err = oc.SetNamespace(p.namespace).AsAdmin().Run("create").Args("-f", templateSubscriptionYAML).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return p
}

func createNamespace(p packagemanifest, oc *exutil.CLI) packagemanifest {
	p.namespace = names.SimpleNameGenerator.GenerateName("test-operators-")
	_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", p.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return p
}

func createOperatorGroup(p packagemanifest, oc *exutil.CLI) {

	templateOperatorGroupYAML := writeOperatorGroup(p.namespace)
	_, err := oc.SetNamespace(p.namespace).AsAdmin().Run("create").Args("-f", templateOperatorGroupYAML).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func writeOperatorGroup(namespace string) (templateOperatorYAML string) {
	isvBaseDir := exutil.FixturePath("testdata", "isv")
	operatorGroupYAML := filepath.Join(isvBaseDir, "operator_group.yaml")
	fileOperatorGroup, _ := os.Open(operatorGroupYAML)
	operatorGroup, _ := ioutil.ReadAll(fileOperatorGroup)
	operatorGroupTemplate := string(operatorGroup)
	templateOperatorYAML = strings.ReplaceAll(operatorGroupYAML, "operator_group.yaml", "operator_group_"+namespace+"_.yaml")
	operatorGroupString := strings.ReplaceAll(operatorGroupTemplate, "$OPERATOR_NAMESPACE", namespace)
	ioutil.WriteFile(templateOperatorYAML, []byte(operatorGroupString), 0644)
	return
}

func writeSubscription(p packagemanifest) (templateSubscriptionYAML string) {
	isvBaseDir := exutil.FixturePath("testdata", "isv")
	subscriptionYAML := filepath.Join(isvBaseDir, "subscription.yaml")
	fileSubscription, _ := os.Open(subscriptionYAML)
	subscription, _ := ioutil.ReadAll(fileSubscription)
	subscriptionTemplate := string(subscription)

	templateSubscriptionYAML = strings.ReplaceAll(subscriptionYAML, "subscription.yaml", "subscription_"+p.csvVersion+"_.yaml")
	operatorSubscription := strings.ReplaceAll(subscriptionTemplate, "$OPERATOR_PACKAGE_NAME", p.name)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CHANNEL", "\""+p.defaultChannel+"\"")
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_NAMESPACE", p.namespace)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_SOURCE", p.catalogSource)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CATALOG_NAMESPACE", p.catalogSourceNamespace)
	operatorSubscription = strings.ReplaceAll(operatorSubscription, "$OPERATOR_CURRENT_CSV_VERSION", p.csvVersion)
	ioutil.WriteFile(templateSubscriptionYAML, []byte(operatorSubscription), 0644)
	e2e.Logf("Subscription: %s", operatorSubscription)
	return
}
func checkDeployment(p packagemanifest, oc *exutil.CLI) {
	poolErr := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		msg, _ := oc.SetNamespace(p.namespace).AsAdmin().Run("get").Args("csv", p.csvVersion, "-o=jsonpath={.status.phase}").Output()
		if strings.Contains(msg, "Succeeded") {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		removeOperatorDependencies(p, oc, false)
		g.Fail("Could not obtain CSV:" + p.csvVersion)
	}
}

func removeOperatorDependencies(p packagemanifest, oc *exutil.CLI, checkDeletion bool) {
	ip, _ := oc.SetNamespace(p.namespace).AsAdmin().Run("get").Args("sub", p.name, "-o=jsonpath={.status.installplan.name}").Output()
	e2e.Logf("IP: %s", ip)
	if len(strings.TrimSpace(ip)) > 0 {
		msg, _ := oc.SetNamespace(p.namespace).AsAdmin().Run("get").Args("ip", ip, "-o=jsonpath={.spec.clusterServiceVersionNames}").Output()
		msg = strings.ReplaceAll(msg, "[", "")
		msg = strings.ReplaceAll(msg, "]", "")
		e2e.Logf("CSVS: %s", msg)
		csvs := strings.Split(msg, " ")
		for i := range csvs {
			e2e.Logf("CSV_: %s", csvs[i])
			msg, err := oc.SetNamespace(p.namespace).AsAdmin().Run("delete").Args("csv", csvs[i]).Output()
			if checkDeletion {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(msg).To(o.ContainSubstring("deleted"))
			}
		}

		subsOutput, _ := oc.SetNamespace(p.namespace).AsAdmin().Run("get").Args("subs", "-o=jsonpath={range .items[?(.status.installplan.name=='"+ip+"')].metadata}{.name}{' '}").Output()
		e2e.Logf("SUBS OUTPUT: %s", subsOutput)
		if len(strings.TrimSpace(subsOutput)) > 0 {
			subs := strings.Split(subsOutput, " ")
			e2e.Logf("SUBS: %s", subs)
			for i := range subs {
				e2e.Logf("SUB_: %s", subs[i])
				msg, err := oc.SetNamespace(p.namespace).AsAdmin().Run("delete").Args("subs", subs[i]).Output()
				if checkDeletion {
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(msg).To(o.ContainSubstring("deleted"))
				}
			}
		}
	}
}
