package compat_otp

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"

	"github.com/ghodss/yaml"
	"github.com/tidwall/pretty"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ApplyClusterResourceFromTemplateWithError apply the changes to the cluster resource and return error if happned.
// For ex: ApplyClusterResourceFromTemplateWithError(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func ApplyClusterResourceFromTemplateWithError(oc *exutil.CLI, parameters ...string) error {
	return resourceFromTemplate(oc, false, true, "", parameters...)
}

// ApplyClusterResourceFromTemplate apply the changes to the cluster resource.
// For ex: ApplyClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func ApplyClusterResourceFromTemplate(oc *exutil.CLI, parameters ...string) {
	resourceFromTemplate(oc, false, false, "", parameters...)
}

// ApplyNsResourceFromTemplate apply changes to the ns resource.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: ApplyNsResourceFromTemplate(oc, "NAMESPACE", "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func ApplyNsResourceFromTemplate(oc *exutil.CLI, namespace string, parameters ...string) {
	resourceFromTemplate(oc, false, false, namespace, parameters...)
}

// CreateClusterResourceFromTemplateWithError create resource from the template and return error if happened.
// For ex: CreateClusterResourceFromTemplateWithError(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func CreateClusterResourceFromTemplateWithError(oc *exutil.CLI, parameters ...string) error {
	return resourceFromTemplate(oc, true, true, "", parameters...)
}

// CreateClusterResourceFromTemplate create resource from the template.
// For ex: CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func CreateClusterResourceFromTemplate(oc *exutil.CLI, parameters ...string) {
	resourceFromTemplate(oc, true, false, "", parameters...)
}

// CreateNsResourceFromTemplate create ns resource from the template.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: CreateNsResourceFromTemplate(oc, "NAMESPACE", "--ignore-unknown-parameters=true", "-f", "TEMPLATE LOCATION")
func CreateNsResourceFromTemplate(oc *exutil.CLI, namespace string, parameters ...string) {
	resourceFromTemplate(oc, true, false, namespace, parameters...)
}

func resourceFromTemplate(oc *exutil.CLI, create bool, returnError bool, namespace string, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		fileName := GetRandomString() + "config.json"
		stdout, _, err := oc.AsAdmin().Run("process").Args(parameters...).OutputsToFiles(fileName)
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}

		configFile = stdout
		return true, nil
	})
	if returnError && err != nil {
		e2e.Logf("fail to process %v", parameters)
		return err
	}
	AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)

	var resourceErr error
	if create {
		if namespace != "" {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile, "-n", namespace).Execute()
		} else {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		}
	} else {
		if namespace != "" {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile, "-n", namespace).Execute()
		} else {
			resourceErr = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
		}
	}
	if returnError && resourceErr != nil {
		e2e.Logf("fail to create/apply resource %v", resourceErr)
		return resourceErr
	}
	AssertWaitPollNoErr(resourceErr, fmt.Sprintf("fail to create/apply resource %v", resourceErr))
	return nil
}

// GetRandomString to create random string
func GetRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

// ApplyResourceFromTemplateWithNonAdminUser to as normal user to create resource from template
func ApplyResourceFromTemplateWithNonAdminUser(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.Run("process").Args(parameters...).OutputToFile(GetRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("the file of resource is %s", configFile)
	return oc.WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
}

// ProcessTemplate process template given file path and parameters
func ProcessTemplate(oc *exutil.CLI, parameters ...string) string {
	var configFile string

	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.Run("process").Args(parameters...).OutputToFile(GetRandomString() + "config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})

	AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))
	e2e.Logf("the file of resource is %s", configFile)
	return configFile
}

// ParameterizedTemplateByReplaceToFile parameterize template to new file
func ParameterizedTemplateByReplaceToFile(oc *exutil.CLI, parameters ...string) string {
	isParameterExist, pIndex := StringsSliceElementsHasPrefix(parameters, "-f", true)
	o.Expect(isParameterExist).Should(o.BeTrue())
	templateFileName := parameters[pIndex+1]
	templateContentByte, readFileErr := os.ReadFile(templateFileName)
	o.Expect(readFileErr).ShouldNot(o.HaveOccurred())
	templateContentStr := string(templateContentByte)
	isParameterExist, pIndex = StringsSliceElementsHasPrefix(parameters, "-p", true)
	o.Expect(isParameterExist).Should(o.BeTrue())
	for i := pIndex + 1; i < len(parameters); i++ {
		if strings.Contains(parameters[i], "=") {
			tempSlice := strings.Split(parameters[i], "=")
			o.Expect(tempSlice).Should(o.HaveLen(2))
			templateContentStr = strings.ReplaceAll(templateContentStr, "${"+tempSlice[0]+"}", tempSlice[1])
		}
	}
	templateContentJSON, convertErr := yaml.YAMLToJSON([]byte(templateContentStr))
	o.Expect(convertErr).NotTo(o.HaveOccurred())
	configFile := filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-"+GetRandomString()+"config.json")
	o.Expect(os.WriteFile(configFile, pretty.Pretty(templateContentJSON), 0644)).ShouldNot(o.HaveOccurred())
	return configFile
}
