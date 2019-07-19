package envresolve

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

// these are all from fieldpath.go

// FormatMap formats map[string]string to a string.
func FormatMap(m map[string]string) (fmtStr string) {
	// output with keys in sorted order to provide stable output
	keys := sets.NewString()
	for key := range m {
		keys.Insert(key)
	}
	for _, key := range keys.List() {
		fmtStr += fmt.Sprintf("%v=%q\n", key, m[key])
	}
	fmtStr = strings.TrimSuffix(fmtStr, "\n")

	return
}

// ExtractFieldPathAsString extracts the field from the given object
// and returns it as a string.  The object must be a pointer to an
// API type.
func ExtractFieldPathAsString(obj interface{}, fieldPath string) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", nil
	}

	if path, subscript, ok := SplitMaybeSubscriptedPath(fieldPath); ok {
		switch path {
		case "metadata.annotations":
			if errs := validation.IsQualifiedName(strings.ToLower(subscript)); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetAnnotations()[subscript], nil
		case "metadata.labels":
			if errs := validation.IsQualifiedName(subscript); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetLabels()[subscript], nil
		default:
			return "", fmt.Errorf("fieldPath %q does not support subscript", fieldPath)
		}
	}

	switch fieldPath {
	case "metadata.annotations":
		return FormatMap(accessor.GetAnnotations()), nil
	case "metadata.labels":
		return FormatMap(accessor.GetLabels()), nil
	case "metadata.name":
		return accessor.GetName(), nil
	case "metadata.namespace":
		return accessor.GetNamespace(), nil
	case "metadata.uid":
		return string(accessor.GetUID()), nil
	}

	return "", fmt.Errorf("unsupported fieldPath: %v", fieldPath)
}

// SplitMaybeSubscriptedPath checks whether the specified fieldPath is
// subscripted, and
//  - if yes, this function splits the fieldPath into path and subscript, and
//    returns (path, subscript, true).
//  - if no, this function returns (fieldPath, "", false).
//
// Example inputs and outputs:
//  - "metadata.annotations['myKey']" --> ("metadata.annotations", "myKey", true)
//  - "metadata.annotations['a[b]c']" --> ("metadata.annotations", "a[b]c", true)
//  - "metadata.labels['']"           --> ("metadata.labels", "", true)
//  - "metadata.labels"               --> ("metadata.labels", "", false)
func SplitMaybeSubscriptedPath(fieldPath string) (string, string, bool) {
	if !strings.HasSuffix(fieldPath, "']") {
		return fieldPath, "", false
	}
	s := strings.TrimSuffix(fieldPath, "']")
	parts := strings.SplitN(s, "['", 2)
	if len(parts) < 2 {
		return fieldPath, "", false
	}
	if len(parts[0]) == 0 {
		return fieldPath, "", false
	}
	return parts[0], parts[1], true
}

// these are from api/v1/helpers.go

// ExtractContainerResourceValue extracts the value of a resource
// in an already known container
func ExtractContainerResourceValue(fs *v1.ResourceFieldSelector, container *v1.Container) (string, error) {
	divisor := resource.Quantity{}
	if divisor.Cmp(fs.Divisor) == 0 {
		divisor = resource.MustParse("1")
	} else {
		divisor = fs.Divisor
	}

	switch fs.Resource {
	case "limits.cpu":
		return convertResourceCPUToString(container.Resources.Limits.Cpu(), divisor)
	case "limits.memory":
		return convertResourceMemoryToString(container.Resources.Limits.Memory(), divisor)
	case "limits.ephemeral-storage":
		return convertResourceEphemeralStorageToString(container.Resources.Limits.StorageEphemeral(), divisor)
	case "requests.cpu":
		return convertResourceCPUToString(container.Resources.Requests.Cpu(), divisor)
	case "requests.memory":
		return convertResourceMemoryToString(container.Resources.Requests.Memory(), divisor)
	case "requests.ephemeral-storage":
		return convertResourceEphemeralStorageToString(container.Resources.Requests.StorageEphemeral(), divisor)
	}

	return "", fmt.Errorf("Unsupported container resource : %v", fs.Resource)
}

// convertResourceCPUToString converts cpu value to the format of divisor and returns
// ceiling of the value.
func convertResourceCPUToString(cpu *resource.Quantity, divisor resource.Quantity) (string, error) {
	c := int64(math.Ceil(float64(cpu.MilliValue()) / float64(divisor.MilliValue())))
	return strconv.FormatInt(c, 10), nil
}

// convertResourceMemoryToString converts memory value to the format of divisor and returns
// ceiling of the value.
func convertResourceMemoryToString(memory *resource.Quantity, divisor resource.Quantity) (string, error) {
	m := int64(math.Ceil(float64(memory.Value()) / float64(divisor.Value())))
	return strconv.FormatInt(m, 10), nil
}

// convertResourceEphemeralStorageToString converts ephemeral storage value to the format of divisor and returns
// ceiling of the value.
func convertResourceEphemeralStorageToString(ephemeralStorage *resource.Quantity, divisor resource.Quantity) (string, error) {
	m := int64(math.Ceil(float64(ephemeralStorage.Value()) / float64(divisor.Value())))
	return strconv.FormatInt(m, 10), nil
}
