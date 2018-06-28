package clientcmd

import (
	"io"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
)

// PrintResourceInfos receives a list of resource infos and prints versioned objects if a generic output format was specified
// otherwise, it iterates through info objects, printing each resource with a unique printer for its mapping
func PrintResourceInfos(cmd *cobra.Command, infos []*resource.Info, out io.Writer) error {
	// mirrors PrintResourceInfoForCommand upstream
	opts := kcmdutil.ExtractCmdPrintOptions(cmd, false)
	printer, err := kcmdutil.PrinterForOptions(opts)
	if err != nil {
		return nil
	}
	if len(infos) == 1 {
		return printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(infos[0].Object, infos[0].Mapping), out)
	}

	list := &kapi.List{}
	for i := range infos {
		list.Items = append(list.Items, kcmdutil.AsDefaultVersionedOrOriginal(infos[i].Object, nil))
	}
	return printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(list, nil), out)
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
