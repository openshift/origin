package validation

import (
	"fmt"
	"os"
	"runtime/debug"

	unversionedvalidation "k8s.io/kubernetes/pkg/api/unversioned/validation"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

func ValidateClusterResourceQuota(clusterquota *quotaapi.ClusterResourceQuota) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&clusterquota.ObjectMeta, false, validation.ValidateResourceQuotaName, field.NewPath("metadata"))

	hasSelectionCriteria := (clusterquota.Spec.Selector.LabelSelector != nil && len(clusterquota.Spec.Selector.LabelSelector.MatchLabels)+len(clusterquota.Spec.Selector.LabelSelector.MatchExpressions) > 0) ||
		(len(clusterquota.Spec.Selector.AnnotationSelector) > 0)

	if !hasSelectionCriteria {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "selector"), "must restrict the selected projects"))
	}
	if clusterquota.Spec.Selector.LabelSelector != nil {
		allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(clusterquota.Spec.Selector.LabelSelector, field.NewPath("spec", "selector", "labels"))...)
		if len(clusterquota.Spec.Selector.LabelSelector.MatchLabels)+len(clusterquota.Spec.Selector.LabelSelector.MatchExpressions) == 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "selector", "labels"), clusterquota.Spec.Selector.LabelSelector, "must restrict the selected projects"))
		}
	}
	if clusterquota.Spec.Selector.AnnotationSelector != nil {
		allErrs = append(allErrs, validation.ValidateAnnotations(clusterquota.Spec.Selector.AnnotationSelector, field.NewPath("spec", "selector", "annotations"))...)
	}

	allErrs = append(allErrs, validation.ValidateResourceQuotaSpec(&clusterquota.Spec.Quota, field.NewPath("spec", "quota"))...)
	allErrs = append(allErrs, validation.ValidateResourceQuotaStatus(&clusterquota.Status.Total, field.NewPath("status", "overall"))...)

	for e := clusterquota.Status.Namespaces.OrderedKeys().Front(); e != nil; e = e.Next() {
		namespace := e.Value.(string)
		used, _ := clusterquota.Status.Namespaces.Get(namespace)

		fldPath := field.NewPath("status", "namespaces").Key(namespace)
		for k, v := range used.Used {
			resPath := fldPath.Key(string(k))
			allErrs = append(allErrs, validation.ValidateResourceQuotaResourceName(string(k), resPath)...)
			allErrs = append(allErrs, validation.ValidateResourceQuantityValue(string(k), v, resPath)...)
		}
	}

	CheckTotals(clusterquota)

	return allErrs
}

func CheckTotals(q *quotaapi.ClusterResourceQuota) {
	namespaces := map[string]int{}
	namespacesTotal := 0

	for e := q.Status.Namespaces.OrderedKeys().Front(); e != nil; e = e.Next() {
		namespace := e.Value.(string)
		used, _ := q.Status.Namespaces.Get(namespace)

		s := used.Used["secrets"]
		namespaces[namespace] = int(s.Value())
		namespacesTotal = namespacesTotal + int(s.Value())
	}

	totalUse := q.Status.Total.Used["secrets"]
	total := int(totalUse.Value())

	if total != namespacesTotal {
		// for i := 1; i < 5; i++ {
		// 	_, file, line, _ := runtime.Caller(i)
		// 	fmt.Printf("%s:%d\n", file, line)
		// }
		fmt.Printf("   !!! set total to %d when namespaces were %d (%v)\n", total, namespacesTotal, namespaces)
		debug.PrintStack()
		os.Exit(1)
	} else {
		// _, file, line, _ := runtime.Caller(1)
		// fmt.Printf("%s:%d\n", file, line)
		// fmt.Printf("   set total to %d when namespaces were %d (%v)\n", total, namespacesTotal, namespaces)
	}
}

func ValidateClusterResourceQuotaUpdate(clusterquota, oldClusterResourceQuota *quotaapi.ClusterResourceQuota) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&clusterquota.ObjectMeta, &oldClusterResourceQuota.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClusterResourceQuota(clusterquota)...)

	return allErrs
}

func ValidateAppliedClusterResourceQuota(clusterquota *quotaapi.AppliedClusterResourceQuota) field.ErrorList {
	return ValidateClusterResourceQuota(quotaapi.ConvertAppliedClusterResourceQuotaToClusterResourceQuota(clusterquota))
}

func ValidateAppliedClusterResourceQuotaUpdate(clusterquota, oldClusterResourceQuota *quotaapi.AppliedClusterResourceQuota) field.ErrorList {
	return ValidateClusterResourceQuotaUpdate(
		quotaapi.ConvertAppliedClusterResourceQuotaToClusterResourceQuota(clusterquota),
		quotaapi.ConvertAppliedClusterResourceQuotaToClusterResourceQuota(oldClusterResourceQuota))
}
