package component

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/evanphx/json-patch"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	"github.com/openshift/origin/pkg/template/generator"
)

const RecommendedName = "component"

var long = templates.LongDesc(`
	Apply component configuration onto the cluster

	`)

func NewCmd(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &Options{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Apply component configuration",
		Long:  long,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().StringSliceVar(&options.Options, "option", options.Options, "One or more optional subdirectories to include for the given component.")
	cmd.Flags().BoolVar(&options.Patch, "patch", options.Patch, "Apply external patches instead of printing their commands.")

	return cmd
}

type Options struct {
	Out io.Writer
	Err io.Writer

	Component string
	Filenames []string
	Output    string
	Namespace string
	Options   []string
	Patch     bool

	NewBuilder         func() *resource.Builder
	ShortOutput        bool
	Mapper             *resource.Mapper
	UnstructuredMapper *resource.Mapper
	PrintObject        func([]*resource.Info) error
}

func (o *Options) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, true, infos, o.Out)
	}

	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"

	var err error
	o.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	dynamicMapper, dynamicTyper, err := f.UnstructuredObject()
	if err != nil {
		return err
	}
	o.Mapper = &resource.Mapper{
		RESTMapper:   mapper,
		ObjectTyper:  typer,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
		Decoder:      f.Decoder(true),
	}
	o.UnstructuredMapper = &resource.Mapper{
		RESTMapper:   relaxedMapper{dynamicMapper},
		ObjectTyper:  dynamicTyper,
		ClientMapper: resource.ClientMapperFunc(f.UnstructuredClientForMapping),
	}
	o.NewBuilder = func() *resource.Builder {
		clientMapperFunc := resource.ClientMapperFunc(f.UnstructuredClientForMapping)
		mapper, typer, err := f.UnstructuredObject()
		if err != nil {
			panic(err)
		}
		mapper = relaxedMapper{mapper}
		categoryExpander := f.CategoryExpander()
		return resource.NewBuilder(mapper, categoryExpander, typer, clientMapperFunc, unstructured.UnstructuredJSONScheme)
	}

	switch len(args) {
	case 1:
		o.Component = args[0]
	case 0:
	default:
		return fmt.Errorf("you may only pass one component name at a time as an argument")
	}

	return nil
}

func (o *Options) Validate() error {
	if len(o.Component) == 0 && len(o.Filenames) == 0 {
		return fmt.Errorf("you must specify a component name or one or more input files")
	}
	if ok, err := regexp.MatchString("^[a-zA-Z0-9-]*$", o.Component); !ok || err != nil {
		return fmt.Errorf("component name must be letters, numbers, and dashes only")
	}
	return nil
}

type parameterSource struct {
	parameter templateapi.Parameter
	sources   []*resource.Info
}

func (o *Options) Run() error {
	b := o.NewBuilder()
	if len(o.Component) > 0 {
		assets, optionals, ok := loadComponentAssetInfo(o.Component)
		if !ok {
			all, _ := bootstrap.AssetDir("contrib/components")
			return fmt.Errorf("component %q was not found - available components are: %s", o.Component, strings.Join(all, ", "))
		}
		glog.V(4).Infof("Found component parts: %v %v", assets, optionals)

		if !optionals.HasAll(o.Options...) {
			return fmt.Errorf("the requested options are not available: %s", strings.Join(sets.NewString(o.Options...).Difference(optionals).List(), ", "))
		}
		for _, option := range o.Options {
			optionalAssets, _, ok := loadComponentAssetInfo(fmt.Sprintf("%s/%s", o.Component, option))
			if !ok {
				return fmt.Errorf("unable to load option %s for component %s", option, o.Component)
			}
			assets.Insert(optionalAssets.UnsortedList()...)
		}
		for _, asset := range assets.List() {
			b.Stream(bytes.NewBuffer(bootstrap.MustAsset(asset)), asset)
		}
	}
	b.FilenameParam(false, &resource.FilenameOptions{Filenames: o.Filenames})
	infos, err := b.ContinueOnError().Flatten().Do().Infos()
	if err != nil {
		return fmt.Errorf("unable to load component: %v", err)
	}

	objects, patches := extractPatches(infos, o.Mapper)
	infos = objects

	allParameters := extractParameters(infos)
	unfilled := sets.NewString()
	for name, param := range allParameters {
		if param.parameter.Required && len(param.parameter.Value) == 0 && len(param.parameter.Generate) == 0 {
			switch name {
			case "NAMESPACE":
				param.parameter.Value = o.Namespace
				continue
			}
			unfilled.Insert(name)
		}
	}
	if len(unfilled) > 0 {
		return fmt.Errorf("the following parameters require values: %s", strings.Join(unfilled.List(), ", "))
	}
	infos, err = processTemplates(infos, allParameters, o.Mapper, o.UnstructuredMapper)
	if err != nil {
		return err
	}

	objects, templatizedPatches := extractPatches(infos, o.Mapper)
	infos = objects
	patches = append(patches, templatizedPatches...)

	infos = reduceDuplicates(infos)
	externalPatches, err := applyPatches(infos, patches, o.Mapper)
	if err != nil {
		return err
	}

	if err := o.PrintObject(infos); err != nil {
		return err
	}

	// print or execute external patches
	// TODO: output in another form?
	for _, patch := range externalPatches {
		gvk := schema.FromAPIVersionAndKind(patch.Target.APIVersion, patch.Target.Kind)
		var versions []string
		if len(gvk.Version) > 0 {
			versions = append(versions, gvk.Version)
		}
		mapping, err := o.Mapper.RESTMapping(gvk.GroupKind(), versions...)
		if err != nil {
			fmt.Fprintf(o.Err, "error: the patch targets an object %s that the server does not recognize: %v\n", jsonOrDie(patch.Target), err)
			continue
		}
		gvr := mapping.GroupVersionKind.GroupVersion().WithResource(mapping.Resource)
		resourceArg := gvr.Resource + "." + gvr.Version
		if len(gvr.Group) > 0 {
			resourceArg = resourceArg + "." + gvr.Group
		}
		var patchType string
		switch patch.PatchType {
		case types.StrategicMergePatchType:
			patchType = "strategic"
		case types.MergePatchType:
			patchType = "merge"
		case types.JSONPatchType:
			patchType = "json"
		default:
			fmt.Fprintf(o.Err, "error: the patch is a type that the client does not recognize: %s\n", patch.PatchType)
			continue
		}
		var namespace string
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			namespace := patch.Target.Namespace
			if len(namespace) == 0 {
				namespace = o.Namespace
			}
			if len(namespace) == 0 {
				fmt.Fprintf(o.Err, "error: the patch points to a namespaced type, but has no namespace and the current namespace is empty\n")
				continue
			}
		}

		if o.Patch {
			client, err := o.Mapper.ClientForMapping(mapping)
			if err != nil {
				fmt.Fprintf(o.Err, "error: unable to get a client for updating %q: %v", resourceArg, err)
				continue
			}
			if _, err := resource.NewHelper(client, mapping).Patch(namespace, patch.Target.Name, patch.PatchType, patch.Patch.Raw); err != nil {
				fmt.Fprintf(o.Err, "error: unable to patch %s: %v", jsonOrDie(&patch.Target), err)
				continue
			}
		} else {
			if len(namespace) > 0 {
				fmt.Fprintf(o.Err, "info: oc patch %q %q -n %q --type=%s --patch=%q\n", resourceArg, patch.Target.Name, namespace, patchType, string(patch.Patch.Raw))
			} else {
				fmt.Fprintf(o.Err, "info: oc patch %q %q --type=%s --patch=%q\n", resourceArg, patch.Target.Name, patchType, string(patch.Patch.Raw))
			}
		}
	}

	return nil
}

func loadComponentAssetInfo(component string) (assets sets.String, optional sets.String, ok bool) {
	assets, optional = sets.NewString(), sets.NewString()
	prefix := fmt.Sprintf("contrib/components/%s/", component)
	for _, s := range bootstrap.AssetNames() {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		path := strings.TrimPrefix(s, prefix)
		if !strings.Contains(path, "/") {
			assets.Insert(s)
			continue
		}
		opt := path[0:strings.LastIndex(path, "/")]
		optional.Insert(opt)
	}
	return assets, optional, len(assets) > 0 || len(optional) > 0
}

func extractParameters(infos []*resource.Info) map[string]*parameterSource {
	params := make(map[string]*parameterSource)
	for _, info := range infos {
		switch t := info.Object.(type) {
		case *templateapi.Template:
			for i := range t.Parameters {
				param := t.Parameters[i]
				glog.V(3).Infof("Template %s parameter %s", info.Source, param.Name)
				p, ok := params[param.Name]
				if !ok {
					params[param.Name] = &parameterSource{parameter: param, sources: []*resource.Info{info}}
					continue
				}
				glog.V(4).Infof("Component parameter %s used more than once", param.Name)
				// accumulate parameters where possible
				if len(param.Generate) > 0 && len(p.parameter.Generate) == 0 {
					p.parameter = param
				}
				if param.Required {
					p.parameter.Required = true
				}
				if len(param.Value) > 0 && len(p.parameter.Value) == 0 {
					p.parameter.Value = param.Value
				}
				p.sources = append(p.sources, info)
			}
		}
	}
	return params
}

func processTemplates(infos []*resource.Info, params map[string]*parameterSource, mapper, unstructuredMapper *resource.Mapper) ([]*resource.Info, error) {
	var results []*resource.Info
	for _, info := range infos {
		switch t := info.Object.(type) {
		case *templateapi.Template:
			// update any parameters in this template with the common definition if needed
			for i, param := range t.Parameters {
				p, ok := params[param.Name]
				if !ok {
					// coding error
					return nil, fmt.Errorf("no parameter %s loaded", param.Name)
				}
				t.Parameters[i] = p.parameter
			}

			if err := processTemplateLocally(t); err != nil {
				return nil, fmt.Errorf("error processing template %s: %v", info.Source, err)
			}
			for i, obj := range t.Objects {
				source := fmt.Sprintf("%s#%d", info.Source, i)
				info, err := infoForObject(source, obj, mapper, unstructuredMapper)
				if err != nil {
					return nil, err
				}
				results = append(results, info)
			}

			// update parameters with any generated values afterwards
			for _, param := range t.Parameters {
				p := params[param.Name]
				p.parameter.Value = param.Value
			}
		default:
			results = append(results, info)
		}
	}
	return results, nil
}

// processTemplateLocally applies the same logic that a remote call would make but makes no
// connection to the server.
func processTemplateLocally(tpl *templateapi.Template) error {
	if errs := templatevalidation.ValidateProcessedTemplate(tpl); len(errs) > 0 {
		return errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}
	processor := template.NewProcessor(map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	})
	if errs := processor.Process(tpl); len(errs) > 0 {
		return errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}
	return nil
}

type resourceKey struct {
	Kind      schema.GroupVersionKind
	Namespace string
	Name      string
}

func reduceDuplicates(infos []*resource.Info) []*resource.Info {
	var results []*resource.Info
	exists := make(map[resourceKey]struct{})
	// use only the latest instance of an object
	for i := len(infos) - 1; i >= 0; i-- {
		key := resourceKey{
			Kind:      infos[i].Mapping.GroupVersionKind,
			Namespace: infos[i].Namespace,
			Name:      infos[i].Name,
		}
		if _, ok := exists[key]; ok {
			continue
		}
		results = append(results, infos[i])
	}
	// reverse order
	for i := 0; i < len(results)/2; i++ {
		j := len(results) - i - 1
		results[i], results[j] = results[j], results[i]
	}
	return results
}

func extractPatches(infos []*resource.Info, mapper *resource.Mapper) ([]*resource.Info, []*resource.Info) {
	var objects, patches []*resource.Info
	patchGVK := schema.GroupVersionKind{Group: "meta.k8s.io", Version: "v1", Kind: "Patch"}
	for _, info := range infos {
		if info.Mapping.GroupVersionKind == patchGVK {
			patches = append(patches, info)
		} else {
			// check if the object is a valid template and swap out the object if necessary
			if structured := unstructuredAsObject(info, mapper); structured != nil {
				if _, ok := structured.Object.(*templateapi.Template); ok {
					info = structured
				}
			}
			objects = append(objects, info)
		}
	}
	return objects, patches
}

func unstructuredAsObject(info *resource.Info, mapper *resource.Mapper) *resource.Info {
	obj, ok := info.Object.(runtime.Unstructured)
	if !ok {
		return nil
	}
	data, err := json.Marshal(obj.UnstructuredContent())
	if err != nil {
		return nil
	}
	if info, err := mapper.InfoForData(data, info.Source); err == nil {
		return info
	}
	return nil
}

func fieldAsObject(field interface{}, obj interface{}) error {
	data, err := json.Marshal(field)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// +optional
	Kind string `json:"kind,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

func isReferencedObject(ref *ObjectReference, info *resource.Info) bool {
	if ref.Kind != info.Mapping.GroupVersionKind.Kind {
		return false
	}
	if ref.Name != info.Name || ref.Namespace != info.Namespace {
		return false
	}
	gv, _ := schema.ParseGroupVersion(ref.APIVersion)
	return gv == info.Mapping.GroupVersionKind.GroupVersion()
}

func applyPatches(infos, patches []*resource.Info, mapper *resource.Mapper) ([]Patch, error) {
	var externalPatches []Patch
	for _, patch := range patches {
		obj, ok := patch.Object.(runtime.Unstructured)
		if !ok {
			return nil, fmt.Errorf("the patch is not the right type %T: %s", patch.Object, patch.Source)
		}
		content := obj.UnstructuredContent()
		patchType, ok := content["patchType"].(string)
		if !ok {
			return nil, fmt.Errorf("the patchType field must be a string: %s", patch.Source)
		}
		patchJS, err := json.Marshal(content["patch"])
		if err != nil {
			return nil, fmt.Errorf("patch %s does not have a valid body: %v", patch.Source, err)
		}
		ref := &ObjectReference{}
		if err := fieldAsObject(content["target"], ref); err != nil {
			return nil, fmt.Errorf("target object is not valid in %s: %v", patch.Source, err)
		}

		var info *resource.Info
		for _, object := range infos {
			if isReferencedObject(ref, object) {
				info = object
				break
			}
		}
		if info == nil {
			externalPatches = append(externalPatches, Patch{
				Target:    *ref,
				PatchType: types.PatchType(patchType),
				Patch:     runtime.RawExtension{Raw: patchJS},
			})
			continue
		}

		originalObjJS, err := json.Marshal(info.Object)
		if err != nil {
			return nil, fmt.Errorf("unable to get original JSON for target object %s: %v", patch.Source, err)
		}

		var patchedObjJS []byte
		switch types.PatchType(patchType) {
		case types.JSONPatchType:
			patchObj, err := jsonpatch.DecodePatch(patchJS)
			if err != nil {
				return nil, err
			}
			if patchedObjJS, err = patchObj.Apply(originalObjJS); err != nil {
				return nil, err
			}
		case types.MergePatchType:
			if patchedObjJS, err = jsonpatch.MergePatch(originalObjJS, patchJS); err != nil {
				return nil, err
			}
		case types.StrategicMergePatchType:
			versionedObj, _, err := mapper.Decode([]byte("{}"), &info.Mapping.GroupVersionKind, nil)
			if err != nil {
				return nil, fmt.Errorf("strategic merge patches can only be used on core API types - use %s and %s for objects of type %v", types.JSONPatchType, types.MergePatchType, info.Mapping.GroupVersionKind)
			}
			if patchedObjJS, err = strategicpatch.StrategicMergePatch(originalObjJS, patchJS, versionedObj); err != nil {
				return nil, err
			}
		default:
			// only here as a safety net - go-restful filters content-type
			return nil, fmt.Errorf("unknown patchType %s for patch %s", patchType, patch.Source)
		}
		patchedObj, _, err := unstructured.UnstructuredJSONScheme.Decode(patchedObjJS, &info.Mapping.GroupVersionKind, nil)
		if err != nil {
			return nil, fmt.Errorf("unable to load object after patching %s: %v", patch.Source, err)
		}
		info.Refresh(patchedObj, true)
	}

	return externalPatches, nil
}

// Patch represents a structured patch object.
type Patch struct {
	runtime.TypeMeta `json:",inline"`

	Target    ObjectReference      `json:"target"`
	Patch     runtime.RawExtension `json:"patch"`
	PatchType types.PatchType      `json:"patchType"`
}

// infoForObject makes a best effort attempt to turn an object into a resource.Info.
func infoForObject(source string, obj runtime.Object, mapper, unstructuredMapper *resource.Mapper) (*resource.Info, error) {
	m := mapper
	if _, ok := obj.(runtime.Unstructured); ok {
		// expected to be using a relaxedMapper
		m = unstructuredMapper
	}
	// TODO: why does my mapper not know about unstructured?
	// TODO: also want a mapper that can lie when disconnected and make a best effort rest mapping calculation
	info, err := m.InfoForObject(obj, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to load object from template %s (%T): %v", source, obj, err)
	}
	info.Source = source
	return info, nil

}

type relaxedMapper struct {
	meta.RESTMapper
}

func (m relaxedMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	mapping, err := m.RESTMapper.RESTMapping(gk, versions...)
	if err != nil && meta.IsNoMatchError(err) && len(versions) > 0 {
		return &meta.RESTMapping{
			GroupVersionKind: gk.WithVersion(versions[0]),
			MetadataAccessor: meta.NewAccessor(),
			Scope:            meta.RESTScopeRoot,
			ObjectConvertor:  identityConvertor{},
		}, nil
	}
	return mapping, err
}
func (m relaxedMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	mappings, err := m.RESTMapper.RESTMappings(gk, versions...)
	if err != nil && meta.IsNoMatchError(err) && len(versions) > 0 {
		return []*meta.RESTMapping{
			{
				GroupVersionKind: gk.WithVersion(versions[0]),
				MetadataAccessor: meta.NewAccessor(),
				Scope:            meta.RESTScopeRoot,
				ObjectConvertor:  identityConvertor{},
			},
		}, nil
	}
	return mappings, err
}

type identityConvertor struct{}

var _ runtime.ObjectConvertor = identityConvertor{}

func (c identityConvertor) Convert(in interface{}, out interface{}, context interface{}) error {
	return fmt.Errorf("unable to convert objects across pointers")
}

func (c identityConvertor) ConvertToVersion(in runtime.Object, gv runtime.GroupVersioner) (out runtime.Object, err error) {
	return in, nil
}

func (c identityConvertor) ConvertFieldLabel(version string, kind string, label string, value string) (string, string, error) {
	return "", "", fmt.Errorf("unable to convert field labels")
}

func jsonOrDie(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(data)
}
