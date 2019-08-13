package convert

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	scheme "k8s.io/kubernetes/pkg/api/legacyscheme"
)

// containsOpenShiftTypes iterates over objects and shortcircuits the execution when
// find OpenShift types. Returns true if OpenShift resources where identified
// and error if there was a problem.
func (o *ConvertOptions) containsOpenShiftTypes() (bool, error) {
	b := o.builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.local)
	if !o.local {
		schema, err := o.validator()
		if err != nil {
			return false, err
		}
		b.Schema(schema)
	}

	r := b.NamespaceParam(o.Namespace).
		ContinueOnError().
		FilenameParam(false, &o.FilenameOptions).
		Flatten().
		Do()

	err := r.Err()
	if err != nil {
		return false, err
	}

	singleItemImplied := false
	infos, err := r.IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return false, err
	}

	openshiftTypes := false
	printableList := &corev1.List{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "List"},
	}
	for _, info := range infos {
		if info.Object == nil {
			continue
		}
		gv := info.Object.GetObjectKind().GroupVersionKind().GroupVersion()
		// if we find at least one OpenShift type (they are already converted to groupped,
		// see shim_kubectl.go openshiftpatch.OAPIToGroupified) just print all
		// resources
		if strings.Contains(gv.String(), "openshift.io") {
			openshiftTypes = true
		}
		printableList.Items = append(printableList.Items, runtime.RawExtension{
			Object: info.Object,
		})
	}

	return openshiftTypes, o.Printer.PrintObj(printableList, o.Out)
}
