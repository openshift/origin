package clientcmd

import (
	"io"

	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
)

// PrintResourceInfos receives a list of resource infos and prints versioned objects if a generic output format was specified
// otherwise, it iterates through info objects, printing each resource with a unique printer for its mapping
func PrintResourceInfos(printer printers.ResourcePrinter, infos []*resource.Info, out io.Writer) error {
	allErrs := []error{}
	for i := range infos {
		if err := printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(infos[i].Object, infos[i].Mapping), out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func ConvertInteralPodSpecToExternal(inFn func(*kapi.PodSpec) error) func(*corev1.PodSpec) error {
	return func(specToMutate *corev1.PodSpec) error {
		internalPodSpec := &kapi.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, internalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(internalPodSpec); err != nil {
			return err
		}
		externalPodSpec := &corev1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(internalPodSpec, externalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *externalPodSpec
		return nil
	}
}
