package operators

import (
	"fmt"
	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"math/rand"
	"strings"
	"sync"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	asUser           = false
	withoutNamespace = true
	withNamespace    = false
	compare          = true
	contain          = false
	requireNS        = true
	notRequireNS     = false
	present          = true
	notPresent       = false
	ok               = true
	nok              = false
)

type csvDescription struct {
	name      string
	namespace string
}

// the method is to delete csv.
func (csv csvDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("remove %s, ns %s", csv.name, csv.namespace)
	dr.getIr(itName).remove(csv.name, "csv", csv.namespace)
}

type subscriptionDescription struct {
	SubName                string `json:"name"`
	Namespace              string `json:"namespace"`
	Channel                string `json:"channel"`
	IpApproval             string `json:"installPlanApproval"`
	OperatorPackage        string `json:"spec.name"`
	CatalogSourceName      string `json:"source"`
	CatalogSourceNamespace string `json:"sourceNamespace"`
	StartingCSV            string `json:"startingCSV,omitempty"`
	CurrentCSV             string
	InstalledCSV           string
	Template               string
	SingleNamespace        bool
	IpCsv                  string
}

// tbuskey@redhat.com for OCP-21080
type PrometheusQueryResult struct {
	Data struct {
		Result []struct {
			Metric struct {
				Name      string `json:"__name__"`
				Approval  string `json:"approval"`
				Channel   string `json:"channel"`
				Container string `json:"container"`
				Endpoint  string `json:"endpoint"`
				Installed string `json:"installed"`
				Instance  string `json:"instance"`
				Job       string `json:"job"`
				SrcName   string `json:"name"`
				Namespace string `json:"namespace"`
				Package   string `json:"package"`
				Pod       string `json:"pod"`
				Service   string `json:"service"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
		ResultType string `json:"resultType"`
	} `json:"data"`
	Status string `json:"status"`
}

//the method is to create sub, and save the sub resrouce into dr. and more create csv possible depending on sub.ipApproval
//if sub.ipApproval is Automatic, it will wait the sub's state become AtLatestKnown and get installed csv as sub.installedCSV, and save csv into dr
//if sub.ipApproval is not Automatic, it will just wait sub's state become UpgradePending
func (sub *subscriptionDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	// for most operator subscription failure, the reason is that there is a left cluster-scoped CSV.
	// I'd like to print all CSV before create it.
	// allCSVs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "--all-namespaces").Output()
	// if err != nil {
	// 	e2e.Failf("!!! Couldn't get all CSVs:%v\n", err)
	// }
	// e2e.Logf("!!! Get all CSVs in this cluster:\n%s\n", allCSVs)

	sub.createWithoutCheck(oc, itName, dr)
	if strings.Compare(sub.IpApproval, "Automatic") == 0 {
		sub.findInstalledCSV(oc, itName, dr)
	} else {
		newCheck("expect", asAdmin, withoutNamespace, compare, "UpgradePending", ok, []string{"sub", sub.SubName, "-n", sub.Namespace, "-o=jsonpath={.status.state}"}).check(oc)
	}
}

//the method is to just create sub, and save it to dr, do not check its state.
func (sub *subscriptionDescription) createWithoutCheck(oc *exutil.CLI, itName string, dr describerResrouce) {
	//isAutomatic := strings.Compare(sub.ipApproval, "Automatic") == 0

	//startingCSV is not necessary. And, if there are multi same package from different CatalogSource, it will lead to error.
	//if strings.Compare(sub.currentCSV, "") == 0 {
	//	sub.currentCSV = getResource(oc, asAdmin, withoutNamespace, "packagemanifest", sub.operatorPackage, fmt.Sprintf("-o=jsonpath={.status.channels[?(@.name==\"%s\")].currentCSV}", sub.channel))
	//	o.Expect(sub.currentCSV).NotTo(o.BeEmpty())
	//}

	//if isAutomatic {
	//	sub.startingCSV = sub.currentCSV
	//} else {
	//	o.Expect(sub.startingCSV).NotTo(o.BeEmpty())
	//}

	// for most operator subscription failure, the reason is that there is a left cluster-scoped CSV.
	// I'd like to print all CSV before create it.
	// It prints many lines which descrease the exact match for RP, and increase log size.
	// So, change it to one line with neccessary information csv name and namespace.
	allCSVs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "--all-namespaces", "-o=jsonpath={range .items[*]}{@.metadata.name}{\",\"}{@.metadata.namespace}{\":\"}{end}").Output()
	if err != nil {
		e2e.Failf("!!! Couldn't get all CSVs:%v\n", err)
	}
	csvMap := make(map[string][]string)
	csvList := strings.Split(allCSVs, ":")
	for _, csv := range csvList {
		if strings.Compare(csv, "") == 0 {
			continue
		}
		name := strings.Split(csv, ",")[0]
		ns := strings.Split(csv, ",")[1]
		val, ok := csvMap[name]
		if ok {
			if strings.HasPrefix(ns, "openshift-") {
				alreadyOpenshiftDefaultNS := false
				for _, v := range val {
					if strings.Contains(v, "openshift-") {
						alreadyOpenshiftDefaultNS = true // normally one default operator exists in all openshift- ns, like elasticsearch-operator
						// only add one openshift- ns to indicate. to save log size and line size. Or else one line
						// will be greater than 3k
						break
					}
				}
				if !alreadyOpenshiftDefaultNS {
					val = append(val, ns)
					csvMap[name] = val
				}
			} else {
				val = append(val, ns)
				csvMap[name] = val
			}
		} else {
			nsSlice := make([]string, 20)
			nsSlice[1] = ns
			csvMap[name] = nsSlice
		}
	}
	for name, ns := range csvMap {
		e2e.Logf("getting csv is %v, the related NS is %v", name, ns)
	}

	e2e.Logf("create sub %s", sub.SubName)
	err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.Template, "-p", "SUBNAME="+sub.SubName, "SUBNAMESPACE="+sub.Namespace, "CHANNEL="+sub.Channel,
		"APPROVAL="+sub.IpApproval, "OPERATORNAME="+sub.OperatorPackage, "SOURCENAME="+sub.CatalogSourceName, "SOURCENAMESPACE="+sub.CatalogSourceNamespace, "STARTINGCSV="+sub.StartingCSV)

	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "sub", sub.SubName, requireNS, sub.Namespace))
}

//the method is to check if the sub's state is AtLatestKnown.
//if it is AtLatestKnown, get installed csv from sub and save it to dr.
//if it is not AtLatestKnown, raise error.
func (sub *subscriptionDescription) findInstalledCSV(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (bool, error) {
		state := getResource(oc, asAdmin, withoutNamespace, "sub", sub.SubName, "-n", sub.Namespace, "-o=jsonpath={.status.state}")
		if strings.Compare(state, "AtLatestKnown") == 0 {
			return true, nil
		}
		e2e.Logf("sub %s state is %s, not AtLatestKnown", sub.SubName, state)
		return false, nil
	})
	if err != nil {
		output := getResource(oc, asAdmin, withoutNamespace, "sub", sub.SubName, "-n", sub.Namespace, "-o", "yaml")
		e2e.Logf(output)
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("sub %s stat is not AtLatestKnown", sub.SubName))

	installedCSV := getResource(oc, asAdmin, withoutNamespace, "sub", sub.SubName, "-n", sub.Namespace, "-o=jsonpath={.status.installedCSV}")
	o.Expect(installedCSV).NotTo(o.BeEmpty())
	if strings.Compare(sub.InstalledCSV, installedCSV) != 0 {
		sub.InstalledCSV = installedCSV
		dr.getIr(itName).add(newResource(oc, "csv", sub.InstalledCSV, requireNS, sub.Namespace))
	}
	e2e.Logf("the installed CSV name is %s", sub.InstalledCSV)
}

//the method is to construct one csv object.
func (sub *subscriptionDescription) getCSV() csvDescription {
	e2e.Logf("csv is %s, namespace is %s", sub.InstalledCSV, sub.Namespace)
	return csvDescription{sub.InstalledCSV, sub.Namespace}
}

//the method is to delete sub which is saved when calling sub.create() or sub.createWithoutCheck()
func (sub *subscriptionDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("remove sub %s, ns is %s", sub.SubName, sub.Namespace)
	dr.getIr(itName).remove(sub.SubName, "sub", sub.Namespace)
}

type configMapDescription struct {
	name      string
	namespace string
	template  string
}

//the method is to create cm with template and save it to dr
func (cm *configMapDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cm.template, "-p", "NAME="+cm.name, "NAMESPACE="+cm.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "cm", cm.name, requireNS, cm.namespace))
	e2e.Logf("create cm %s SUCCESS", cm.name)
}

type catalogSourceDescription struct {
	name          string
	namespace     string
	displayName   string
	publisher     string
	sourceType    string
	address       string
	template      string
	secret        string
	interval      string
	imageTemplate string
}

//the method is to create catalogsource with template, and save it to dr.
func (catsrc *catalogSourceDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	if strings.Compare(catsrc.interval, "") == 0 {
		catsrc.interval = "10m0s"
		e2e.Logf("set interval to be 10m0s")
	}
	err := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", catsrc.template,
		"-p", "NAME="+catsrc.name, "NAMESPACE="+catsrc.namespace, "ADDRESS="+catsrc.address, "SECRET="+catsrc.secret,
		"DISPLAYNAME="+"\""+catsrc.displayName+"\"", "PUBLISHER="+"\""+catsrc.publisher+"\"", "SOURCETYPE="+catsrc.sourceType,
		"INTERVAL="+catsrc.interval, "IMAGETEMPLATE="+catsrc.imageTemplate)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "catsrc", catsrc.name, requireNS, catsrc.namespace))
	e2e.Logf("create catsrc %s SUCCESS", catsrc.name)
}

//the method is to delete catalogsource.
func (catsrc *catalogSourceDescription) delete(itName string, dr describerResrouce) {
	e2e.Logf("delete carsrc %s, ns is %s", catsrc.name, catsrc.namespace)
	dr.getIr(itName).remove(catsrc.name, "catsrc", catsrc.namespace)
}

type operatorGroupDescription struct {
	name               string
	namespace          string
	multinslabel       string
	template           string
	serviceAccountName string
}

//the method is to create og and save it to dr
//if og.multinslabel is not set, it will create og with ownnamespace or allnamespace depending on template
//if og.multinslabel is set, it will create og with multinamespace.
func (og *operatorGroupDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	var err error
	if strings.Compare(og.multinslabel, "") != 0 && strings.Compare(og.serviceAccountName, "") != 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "MULTINSLABEL="+og.multinslabel, "SERVICE_ACCOUNT_NAME="+og.serviceAccountName)
	} else if strings.Compare(og.multinslabel, "") == 0 && strings.Compare(og.serviceAccountName, "") == 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
	} else if strings.Compare(og.multinslabel, "") != 0 {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "MULTINSLABEL="+og.multinslabel)
	} else {
		err = applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace, "SERVICE_ACCOUNT_NAME="+og.serviceAccountName)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "og", og.name, requireNS, og.namespace))
	e2e.Logf("create og %s success", og.name)
}

type projectDescription struct {
	name            string
	targetNamespace string
}

//the method is to delete project with name if exist. and then create it with name, and back to project with targetNamespace
func (p *projectDescription) create(oc *exutil.CLI, itName string, dr describerResrouce) {
	removeResource(oc, asAdmin, withoutNamespace, "project", p.name)
	_, err := doAction(oc, "new-project", asAdmin, withoutNamespace, p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
	dr.getIr(itName).add(newResource(oc, "project", p.name, notRequireNS, ""))
	_, err = doAction(oc, "project", asAdmin, withoutNamespace, p.targetNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to label project
func (p *projectDescription) label(oc *exutil.CLI, label string) {
	_, err := doAction(oc, "label", asAdmin, withoutNamespace, "ns", p.name, "env="+label)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//the method is to delete project
func (p *projectDescription) delete(oc *exutil.CLI) {
	_, err := doAction(oc, "delete", asAdmin, withoutNamespace, "project", p.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type checkDescription struct {
	method          string
	executor        bool
	inlineNamespace bool
	expectAction    bool
	expectContent   string
	expect          bool
	resource        []string
}

//the method is to make newCheck object.
//the method paramter is expect, it will check something is expceted or not
//the method paramter is present, it will check something exists or not
//the executor is asAdmin, it will exectue oc with Admin
//the executor is asUser, it will exectue oc with User
//the inlineNamespace is withoutNamespace, it will execute oc with WithoutNamespace()
//the inlineNamespace is withNamespace, it will execute oc with WithNamespace()
//the expectAction take effective when method is expect, if it is contain, it will check if the strings contain substring with expectContent parameter
//                                                       if it is compare, it will check the strings is samme with expectContent parameter
//the expectContent is the content we expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
//the expect is ok, resource existing is OK for method == present, no error raise. if resource not existing, error raise
//the expect is nok, resource not existing is OK for method == present, no error raise. if resource existing, error raise
func newCheck(method string, executor bool, inlineNamespace bool, expectAction bool,
	expectContent string, expect bool, resource []string) checkDescription {
	return checkDescription{
		method:          method,
		executor:        executor,
		inlineNamespace: inlineNamespace,
		expectAction:    expectAction,
		expectContent:   expectContent,
		expect:          expect,
		resource:        resource,
	}
}

//the method is to check the resource per definition of the above described newCheck.
func (ck checkDescription) check(oc *exutil.CLI) {
	switch ck.method {
	case "present":
		ok := isPresentResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.resource...)
		o.Expect(ok).To(o.BeTrue())
	case "expect":
		err := expectedResource(oc, ck.executor, ck.inlineNamespace, ck.expectAction, ck.expectContent, ck.expect, ck.resource...)
		exutil.AssertWaitPollNoErr(err, fmt.Sprintf("expected content %s not found by %v", ck.expectContent, ck.resource))
	default:
		err := fmt.Errorf("unknown method")
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

//it is the check list so that all the check are done in parallel.
type checkList []checkDescription

//the method is to add one check
func (cl checkList) add(ck checkDescription) {
	cl = append(cl, ck)
}

//the method is to make check list empty.
func (cl checkList) empty() {
	cl = cl[0:0]
}

//the method is to execute all the check in parallel.
func (cl checkList) check(oc *exutil.CLI) {
	var wg sync.WaitGroup
	for _, ck := range cl {
		wg.Add(1)
		go func(ck checkDescription) {
			defer g.GinkgoRecover()
			defer wg.Done()
			ck.check(oc)
		}(ck)
	}
	wg.Wait()
}

type resourceDescription struct {
	oc               *exutil.CLI
	asAdmin          bool
	withoutNamespace bool
	kind             string
	name             string
	requireNS        bool
	namespace        string
}

//the method is to construc one resource so that it can be deleted with itResource and describerResrouce
//oc is the oc client
//asAdmin means when deleting resource, we take admin role
//withoutNamespace means when deleting resource, we take WithoutNamespace
//kind is the kind of resource
//name is the name of resource
//namespace is the namesapce of resoruce. it is "" for cluster level resource
//if requireNS is requireNS, need to add "-n" parameter. used for project level resource
//if requireNS is notRequireNS, no need to add "-n". used for cluster level resource
func newResource(oc *exutil.CLI, kind string, name string, nsflag bool, namespace string) resourceDescription {
	return resourceDescription{
		oc:               oc,
		asAdmin:          asAdmin,
		withoutNamespace: withoutNamespace,
		kind:             kind,
		name:             name,
		requireNS:        nsflag,
		namespace:        namespace,
	}
}

//the method is to delete resource.
func (r resourceDescription) delete() {
	if r.withoutNamespace && r.requireNS {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name, "-n", r.namespace)
	} else {
		removeResource(r.oc, r.asAdmin, r.withoutNamespace, r.kind, r.name)
	}
}

//the struct to save the resource created in g.It, and it take name+kind+namespace as key to save resoruce of g.It.
type itResource map[string]resourceDescription

func (ir itResource) add(r resourceDescription) {
	ir[r.name+r.kind+r.namespace] = r
}

func (ir itResource) remove(name string, kind string, namespace string) {
	rKey := name + kind + namespace
	if r, ok := ir[rKey]; ok {
		r.delete()
		delete(ir, rKey)
	}
}
func (ir itResource) cleanup() {
	for _, r := range ir {
		e2e.Logf("cleanup resource %s,   %s", r.kind, r.name)
		ir.remove(r.name, r.kind, r.namespace)
	}
}

//the struct is to save g.It in g.Describe, and map the g.It name to itResource so that it can get all resource of g.Describe per g.It.
type describerResrouce map[string]itResource

func (dr describerResrouce) addIr(itName string) {
	dr[itName] = itResource{}
}
func (dr describerResrouce) getIr(itName string) itResource {
	ir, ok := dr[itName]
	if !ok {
		e2e.Logf("!!! couldn't find the itName:%s, print the describerResrouce:%v", itName, dr)
	}
	o.Expect(ok).To(o.BeTrue())
	return ir
}
func (dr describerResrouce) rmIr(itName string) {
	delete(dr, itName)
}

//the method is to get random string with length 8.
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

//the method is to create one resource with template
func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "olm-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

//the method is to check the presence of the resource
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//present means if you expect the resource presence or not. if it is ok, expect presence. if it is nok, expect not present.
func isPresentResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, present bool, parameters ...string) bool {
	parameters = append(parameters, "--ignore-not-found")
	err := wait.Poll(3*time.Second, 70*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		if !present && strings.Compare(output, "") == 0 {
			return true, nil
		}
		if present && strings.Compare(output, "") != 0 {
			return true, nil
		}
		return false, nil
	})

	return err == nil
}

//the method is to get something from resource. it is "oc get xxx" actaully
//asAdmin means if taking admin to get it
//withoutNamespace means if take WithoutNamespace() to get it.
func getResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	err = wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not get %v", parameters))
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	return result
}

//the method is to check one resource's attribution is expected or not.
//asAdmin means if taking admin to check it
//withoutNamespace means if take WithoutNamespace() to check it.
//isCompare means if containing or exactly comparing. if it is contain, it check result contain content. if it is compare, it compare the result with content exactly.
//content is the substing to be expected
//the expect is ok, contain or compare result is OK for method == expect, no error raise. if not OK, error raise
//the expect is nok, contain or compare result is NOK for method == expect, no error raise. if OK, error raise
func expectedResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, isCompare bool, content string, expect bool, parameters ...string) error {
	expectMap := map[bool]string{
		true:  "do",
		false: "do not",
	}

	cc := func(a, b string, ic bool) bool {
		bs := strings.Split(b, "+2+")
		ret := false
		for _, s := range bs {
			if (ic && strings.Compare(a, s) == 0) || (!ic && strings.Contains(a, s)) {
				ret = true
			}
		}
		return ret
	}
	e2e.Logf("Running: oc get asAdmin(%t) withoutNamespace(%t) %s", asAdmin, withoutNamespace, strings.Join(parameters, " "))
	return wait.Poll(3*time.Second, 150*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("---> we %v expect value: %s, in returned value: %s", expectMap[expect], content, output)
		if isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s matches one of the content %s, expected", output, content)
			return true, nil
		}
		if isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not matche the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && expect && cc(output, content, isCompare) {
			e2e.Logf("the output %s contains one of the content %s, expected", output, content)
			return true, nil
		}
		if !isCompare && !expect && !cc(output, content, isCompare) {
			e2e.Logf("the output %s does not contain the content %s, expected", output, content)
			return true, nil
		}
		e2e.Logf("---> Not as expected! Return false")
		return false, nil
	})
}

//the method is to remove resource
//asAdmin means if taking admin to remove it
//withoutNamespace means if take WithoutNamespace() to remove it.
func removeResource(oc *exutil.CLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	output, err := doAction(oc, "delete", asAdmin, withoutNamespace, parameters...)
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		output, err := doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
			e2e.Logf("the resource is delete successfully")
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("can not remove %v", parameters))
}

//the method is to do something with oc.
//asAdmin means if taking admin to do it
//withoutNamespace means if take WithoutNamespace() to do it.
func doAction(oc *exutil.CLI, action string, asAdmin bool, withoutNamespace bool, parameters ...string) (string, error) {
	if asAdmin && withoutNamespace {
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if asAdmin && !withoutNamespace {
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && withoutNamespace {
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	}
	if !asAdmin && !withoutNamespace {
		return oc.Run(action).Args(parameters...).Output()
	}
	return "", nil
}

func GetDirPath(filePathStr string, filePre string) string {
	if !strings.Contains(filePathStr, "/") || filePathStr == "/" {
		return ""
	}
	dir, file := filepath.Split(filePathStr)
	if strings.HasPrefix(file, filePre) {
		return filePathStr
	} else {
		return GetDirPath(filepath.Dir(dir), filePre)
	}
}

func DeleteDir(filePathStr string, filePre string) bool {
	filePathToDelete := GetDirPath(filePathStr, filePre)
	if filePathToDelete == "" || !strings.Contains(filePathToDelete, filePre) {
		e2e.Logf("there is no such dir %s", filePre)
		return false
	} else {
		e2e.Logf("remove dir %s", filePathToDelete)
		os.RemoveAll(filePathToDelete)
		if _, err := os.Stat(filePathToDelete); err == nil {
			e2e.Logf("delele dir %s failed", filePathToDelete)
			return false
		}
		return true
	}
}

func CheckUpgradeStatus(oc *exutil.CLI, expectedStatus string) {
	e2e.Logf("Check the Upgradeable status of the OLM, expected: %s", expectedStatus)
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		upgradeable, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "operator-lifecycle-manager", "-o=jsonpath={.status.conditions[?(@.type==\"Upgradeable\")].status}").Output()
		if err != nil {
			e2e.Failf("Fail to get the Upgradeable status of the OLM: %v", err)
		}
		if upgradeable != expectedStatus {
			return false, nil
		}
		e2e.Logf("The Upgraableable status should be %s, and get %s", expectedStatus, upgradeable)
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Upgradeable status of the OLM %s is not expected", expectedStatus))
}
