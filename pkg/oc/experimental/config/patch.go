package config

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/categories"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapiinstall "github.com/openshift/origin/pkg/cmd/server/apis/config/install"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const PatchRecommendedName = "patch"

var patchTypes = map[string]types.PatchType{"json": types.JSONPatchType, "merge": types.MergePatchType, "strategic": types.StrategicMergePatchType}

// PatchOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type PatchOptions struct {
	Filename  string
	Patch     string
	PatchType types.PatchType

	Builder *resource.Builder
	Printer kprinters.ResourcePrinter

	Out io.Writer
}

var (
	patch_long    = templates.LongDesc(`Patch the master-config.yaml or node-config.yaml`)
	patch_example = templates.Examples(`
		# Set the auditConfig.enabled value to true
		%[1]s openshift.local.config/master/master-config.yaml --patch='{"auditConfig": {"enabled": true}}'`)
)

func NewCmdPatch(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &PatchOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " FILENAME -p PATCH",
		Short:   "Update field(s) of a resource using a patch.",
		Long:    patch_long,
		Example: patch_example,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunPatch())
		},
	}
	cmd.Flags().StringVarP(&o.Patch, "patch", "p", "", "The patch to be applied to the resource JSON file.")
	cmd.MarkFlagRequired("patch")
	cmd.Flags().String("type", "strategic", fmt.Sprintf("The type of patch being provided; one of %v", sets.StringKeySet(patchTypes).List()))
	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o *PatchOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one FILENAME is allowed: %v", args)
	}
	o.Filename = args[0]

	patchTypeString := strings.ToLower(cmdutil.GetFlagString(cmd, "type"))
	ok := false
	o.PatchType, ok = patchTypes[patchTypeString]
	if !ok {
		return cmdutil.UsageErrorf(cmd, fmt.Sprintf("--type must be one of %v, not %q", sets.StringKeySet(patchTypes).List(), patchTypeString))
	}

	o.Builder = resource.NewBuilder(
		&resource.Mapper{
			RESTMapper:   configapiinstall.NewRESTMapper(),
			ObjectTyper:  configapi.Scheme,
			ClientMapper: resource.DisabledClientForMapping{},
			Decoder:      configapi.Codecs.LegacyCodec(),
		},
		&resource.Mapper{
			RESTMapper:   configapiinstall.NewRESTMapper(),
			ObjectTyper:  configapi.Scheme,
			ClientMapper: resource.DisabledClientForMapping{},
			Decoder:      unstructured.UnstructuredJSONScheme,
		},
		categories.SimpleCategoryExpander{},
	)

	var err error
	mapper, typer := f.Object()
	decoders := []runtime.Decoder{f.Decoder(true), unstructured.UnstructuredJSONScheme}
	printOpts := cmdutil.ExtractCmdPrintOptions(cmd, false)
	printOpts.OutputFormatType = "yaml"

	o.Printer, err = kprinters.GetStandardPrinter(
		mapper, typer, nil, decoders, *printOpts,
	)
	if err != nil {
		return err
	}

	return nil
}

func (o *PatchOptions) Validate() error {
	if len(o.Patch) == 0 {
		return errors.New("must specify -p to patch")
	}
	if len(o.Filename) == 0 {
		return errors.New("filename is required")
	}

	return nil
}

func (o *PatchOptions) RunPatch() error {
	patchBytes, err := yaml.ToJSON([]byte(o.Patch))
	if err != nil {
		return fmt.Errorf("unable to parse %q: %v", o.Patch, err)
	}

	r := o.Builder.
		Internal().
		FilenameParam(false, &resource.FilenameOptions{Recursive: false, Filenames: []string{o.Filename}}).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if len(infos) > 1 {
		return fmt.Errorf("multiple resources provided")
	}
	info := infos[0]

	originalObjJS, err := runtime.Encode(configapi.Codecs.LegacyCodec(info.Mapping.GroupVersionKind.GroupVersion()), info.Object.(runtime.Object))
	if err != nil {
		return err
	}
	patchedObj := info.Object.DeepCopyObject()
	originalPatchedObjJS, err := getPatchedJS(o.PatchType, originalObjJS, patchBytes, patchedObj.(runtime.Object))
	if err != nil {
		return err
	}

	rawExtension := &runtime.Unknown{
		Raw: originalPatchedObjJS,
	}

	if err := o.Printer.PrintObj(rawExtension, o.Out); err != nil {
		return err
	}

	return nil
}

func getPatchedJS(patchType types.PatchType, originalJS, patchJS []byte, obj runtime.Object) ([]byte, error) {
	switch patchType {
	case types.JSONPatchType:
		patchObj, err := jsonpatch.DecodePatch(patchJS)
		if err != nil {
			return nil, err
		}
		return patchObj.Apply(originalJS)

	case types.MergePatchType:
		return jsonpatch.MergePatch(originalJS, patchJS)

	case types.StrategicMergePatchType:
		return strategicpatch.StrategicMergePatch(originalJS, patchJS, obj)

	default:
		// only here as a safety net - go-restful filters content-type
		return nil, fmt.Errorf("unknown Content-Type header for patch: %v", patchType)
	}
}
