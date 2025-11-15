package compat_otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// DeleteLabelsFromSpecificResource deletes the custom labels from the specific resource
func DeleteLabelsFromSpecificResource(oc *exutil.CLI, resourceKindAndName string, resourceNamespace string, labelNames ...string) (string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName)
	cargs = append(cargs, StringsSliceElementsAddSuffix(labelNames, "-")...)
	return oc.AsAdmin().WithoutNamespace().Run("label").Args(cargs...).Output()
}

// AddLabelsToSpecificResource adds the custom labels to the specific resource
func AddLabelsToSpecificResource(oc *exutil.CLI, resourceKindAndName string, resourceNamespace string, labels ...string) (string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName)
	cargs = append(cargs, labels...)
	cargs = append(cargs, "--overwrite")
	return oc.AsAdmin().WithoutNamespace().Run("label").Args(cargs...).Output()
}

// GetResourceSpecificLabelValue gets the specified label value from the resource and label name
func GetResourceSpecificLabelValue(oc *exutil.CLI, resourceKindAndName string, resourceNamespace string, labelName string) (string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName, "-o=jsonpath={.metadata.labels."+labelName+"}")
	return oc.AsAdmin().WithoutNamespace().Run("get").Args(cargs...).Output()
}

// AddAnnotationsToSpecificResource adds the custom annotations to the specific resource
func AddAnnotationsToSpecificResource(oc *exutil.CLI, resourceKindAndName, resourceNamespace string, annotations ...string) (string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName)
	cargs = append(cargs, annotations...)
	cargs = append(cargs, "--overwrite")
	return determineExecCLI(oc).WithoutNamespace().Run("annotate").Args(cargs...).Output()
}

// RemoveAnnotationFromSpecificResource removes the specified annotation from the resource
func RemoveAnnotationFromSpecificResource(oc *exutil.CLI, resourceKindAndName, resourceNamespace string, annotationName string) (string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName)
	cargs = append(cargs, annotationName+"-")
	return determineExecCLI(oc).WithoutNamespace().Run("annotate").Args(cargs...).Output()
}

// GetAnnotationsFromSpecificResource gets the annotations from the specific resource
func GetAnnotationsFromSpecificResource(oc *exutil.CLI, resourceKindAndName, resourceNamespace string) ([]string, error) {
	var cargs []string
	if resourceNamespace != "" {
		cargs = append(cargs, "-n", resourceNamespace)
	}
	cargs = append(cargs, resourceKindAndName, "--list")
	annotationsStr, getAnnotationsErr := determineExecCLI(oc).WithoutNamespace().Run("annotate").Args(cargs...).Output()
	if getAnnotationsErr != nil {
		e2e.Logf(`Failed to get annotations from /%s in namespace %s: "%v"`, resourceKindAndName, resourceNamespace, getAnnotationsErr)
	}
	return strings.Fields(annotationsStr), getAnnotationsErr
}

// IsSpecifiedAnnotationKeyExist judges whether the specified annotationKey exist on the resource
func IsSpecifiedAnnotationKeyExist(oc *exutil.CLI, resourceKindAndName, resourceNamespace, annotationKey string) bool {
	resourceAnnotations, getResourceAnnotationsErr := GetAnnotationsFromSpecificResource(oc, resourceKindAndName, resourceNamespace)
	o.Expect(getResourceAnnotationsErr).NotTo(o.HaveOccurred())
	isAnnotationKeyExist, _ := StringsSliceElementsHasPrefix(resourceAnnotations, annotationKey+"=", true)
	return isAnnotationKeyExist
}

// StringsSliceContains judges whether the strings Slice contains specific element, return bool and the first matched index
// If no matched return (false, 0)
func StringsSliceContains(stringsSlice []string, element string) (bool, int) {
	for index, strElement := range stringsSlice {
		if strElement == element {
			return true, index
		}
	}
	return false, 0
}

// StringsSliceElementsHasPrefix judges whether the strings Slice contains an element which has the specific prefix
// returns bool and the first matched index
// sequential order: -> sequentialFlag: "true"
// reverse order:    -> sequentialFlag: "false"
// If no matched return (false, 0)
func StringsSliceElementsHasPrefix(stringsSlice []string, elementPrefix string, sequentialFlag bool) (bool, int) {
	if len(stringsSlice) == 0 {
		return false, 0
	}
	if sequentialFlag {
		for index, strElement := range stringsSlice {
			if strings.HasPrefix(strElement, elementPrefix) {
				return true, index
			}
		}
	} else {
		for i := len(stringsSlice) - 1; i >= 0; i-- {
			if strings.HasPrefix(stringsSlice[i], elementPrefix) {
				return true, i
			}
		}
	}
	return false, 0
}

// StringsSliceElementsAddSuffix returns a new string slice all elements with the specific suffix added
func StringsSliceElementsAddSuffix(stringsSlice []string, suffix string) []string {
	if len(stringsSlice) == 0 {
		return []string{}
	}
	var newStringsSlice = make([]string, 0, 10)
	for _, element := range stringsSlice {
		newStringsSlice = append(newStringsSlice, element+suffix)
	}
	return newStringsSlice
}

const (
	AsAdmin          = true
	AsUser           = false
	WithoutNamespace = true
	WthNamespace     = false
	Immediately      = true
	NotImmediately   = false
	AllowEmpty       = true
	NotAllowEmpty    = false
	Appear           = true
	Disappear        = false
)

// GetFieldWithJsonpath gets the field of the resource per jsonpath
// interval and timeout is the inveterl and timeout of Poll
// immediately means if it wait first interval and then get
// allowEmpty means if the result allow empty string
// asAdmin means oc.AsAdmin() or not
// withoutNamespace means oc.WithoutNamespace() or not.
// for example, it is to get clusterresource
// GetFieldWithJsonpath(oc, 3*time.Second, 150*time.Second, exutil.NotImmediately, exutil.NotAllowEmpty, exutil.AsAdmin, exutil.WithoutNamespace, "operator", name, "-o", "jsonpath={.status}")
// if you want to get ns resource, could be
// GetFieldWithJsonpath(oc, 3*time.Second, 150*time.Second, exutil.NotImmediately, exutil.NotAllowEmpty, exutil.AsAdmin, exutil.WithoutNamespace, "-n", ns, "pod", name, "-o", "jsonpath={.status}")
// or if the ns is same to oc.Namespace, could be
// GetFieldWithJsonpath(oc, 3*time.Second, 150*time.Second, exutil.NotImmediately, exutil.AllowEmpty, exutil.AsAdmin, exutil.WithoutNamespace, "pod", name, "-o", "jsonpath={.status}")
func GetFieldWithJsonpath(oc *exutil.CLI, interval, timeout time.Duration, immediately, allowEmpty, asAdmin, withoutNamespace bool, parameters ...string) (string, error) {
	var result string
	var err error
	usingJsonpath := false
	for _, parameter := range parameters {
		if strings.Contains(parameter, "jsonpath") {
			usingJsonpath = true
		}
	}
	if !usingJsonpath {
		return "", fmt.Errorf("you do not use jsonpath to get field")
	}
	errWait := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, immediately, func(ctx context.Context) (bool, error) {
		result, err = ocAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil || (!allowEmpty && strings.TrimSpace(result) == "") {
			e2e.Logf("output is %v, error is %v, and try next", result, err)
			return false, nil
		}
		return true, nil
	})
	e2e.Logf("$oc get %v, the returned resource:%v", parameters, result)
	// replace errWait because it is always timeout if it happned with wait.Poll
	if errWait != nil {
		errWait = fmt.Errorf("can not get resource with %v", parameters)
	}
	return result, errWait
}

// CheckAppearance check if the resource appears or not.
// interval and timeout is the inveterl and timeout of Poll
// immediately means if it wait first interval and then check
// asAdmin means oc.AsAdmin() or not
// withoutNamespace means oc.WithoutNamespace() or not.
// appear means expect appear or not
// for example, expect pod in ns appear
// CheckAppearance(oc, 4*time.Second, 200*time.Second, exutil.NotImmediately, exutil.AsAdmin, exutil.WithoutNamespace, exutil.Appear, "-n", ns, "pod" name)
// if you expect pod in ns disappear, could be
// CheckAppearance(oc, 4*time.Second, 200*time.Second, exutil.NotImmediately, exutil.AsAdmin, exutil.WithoutNamespace, exutil.Disappear, "-n", ns, "pod" name)
func CheckAppearance(oc *exutil.CLI, interval, timeout time.Duration, immediately, asAdmin, withoutNamespace, appear bool, parameters ...string) bool {
	if !appear {
		parameters = append(parameters, "--ignore-not-found")
	}
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, immediately, func(ctx context.Context) (bool, error) {
		output, err := ocAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil {
			e2e.Logf("the get error is %v, and try next", err)
			return false, nil
		}
		e2e.Logf("output: %v", output)
		if !appear && strings.Compare(output, "") == 0 {
			return true, nil
		}
		if appear && strings.Compare(output, "") != 0 && !strings.Contains(strings.ToLower(output), "no resources found") {
			return true, nil
		}
		return false, nil
	})
	return err == nil
}

// CleanupResource cleanup one resouce and check if it is not found.
// interval and timeout is the inveterl and timeout of Poll to check if it is not found
// asAdmin means oc.AsAdmin() or not
// withoutNamespace means oc.WithoutNamespace() or not.
// for example, cleanup cluster level resource
// CleanupResource(oc, 4*time.Second, 160*time.Second, exutil.AsAdmin, exutil.WithoutNamespace, "operator.operators.operatorframework.io", operator.Name)
// cleanup ns resource
// CleanupResource(oc, 4*time.Second, 160*time.Second, exutil.AsAdmin, exutil.WithoutNamespace, "-n", ns, "pod" name)
func CleanupResource(oc *exutil.CLI, interval, timeout time.Duration, asAdmin, withoutNamespace bool, parameters ...string) {
	output, err := ocAction(oc, "delete", asAdmin, withoutNamespace, parameters...)
	if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
		e2e.Logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.PollUntilContextTimeout(context.TODO(), interval, timeout, false, func(ctx context.Context) (bool, error) {
		output, err := ocAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil && (strings.Contains(output, "NotFound") || strings.Contains(output, "No resources found")) {
			e2e.Logf("the resource is delete successfully")
			return true, nil
		}
		return false, nil
	})
	AssertWaitPollNoErr(err, fmt.Sprintf("can not remove %v", parameters))
}

// ocAction basical executes oc command
func ocAction(oc *exutil.CLI, action string, asAdmin, withoutNamespace bool, parameters ...string) (string, error) {
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

// WaitForResourceUpdate waits for the resourceVersion of a resource to be updated
func WaitForResourceUpdate(ctx context.Context, oc *exutil.CLI, interval, timeout time.Duration, kindAndName, namespace, oldResourceVersion string) error {
	args := []string{kindAndName}
	if len(namespace) > 0 {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-o=jsonpath={.metadata.resourceVersion}")
	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (done bool, err error) {
		resourceVersion, _, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Outputs()
		if err != nil {
			e2e.Logf("Error getting current resourceVersion: %v", err)
			return false, nil
		}
		if len(resourceVersion) == 0 {
			return false, errors.New("obtained empty resourceVersion")
		}
		if resourceVersion == oldResourceVersion {
			e2e.Logf("resourceVersion unchanged, keep polling")
			return false, nil
		}
		return true, nil
	})
}
