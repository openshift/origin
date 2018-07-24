package set

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/origin/pkg/oc/util/clientcmd"
)

const (
	volumePrefix    = "volume-"
	storageAnnClass = "volume.beta.kubernetes.io/storage-class"
)

var (
	volumeLong = templates.LongDesc(`
		Update volumes on a pod template

		This command can add, update or remove volumes from containers for any object
		that has a pod template (deployment configs, replication controllers, or pods).
		You can list volumes in pod or any object that has a pod template. You can
		specify a single object or multiple, and alter volumes on all containers or
		just those that match a given name.

		If you alter a volume setting on a deployment config, a deployment will be
		triggered. Changing a replication controller will not affect running pods, and
		you cannot change a pod's volumes once it has been created.

		Volume types include:

		* emptydir (empty directory) *default*: A directory allocated when the pod is
		  created on a local host, is removed when the pod is deleted and is not copied
			across servers
		* hostdir (host directory): A directory with specific path on any host
		 (requires elevated privileges)
		* persistentvolumeclaim or pvc (persistent volume claim): Link the volume
		  directory in the container to a persistent volume claim you have allocated by
			name - a persistent volume claim is a request to allocate storage. Note that
			if your claim hasn't been bound, your pods will not start.
		* secret (mounted secret): Secret volumes mount a named secret to the provided
		  directory.

		For descriptions on other volume types, see https://docs.openshift.com`)

	volumeExample = templates.Examples(`
		# List volumes defined on all deployment configs in the current project
	  %[1]s volume dc --all

	  # Add a new empty dir volume to deployment config (dc) 'registry' mounted under
	  # /var/lib/registry
	  %[1]s volume dc/registry --add --mount-path=/var/lib/registry

	  # Use an existing persistent volume claim (pvc) to overwrite an existing volume 'v1'
	  %[1]s volume dc/registry --add --name=v1 -t pvc --claim-name=pvc1 --overwrite

	  # Remove volume 'v1' from deployment config 'registry'
	  %[1]s volume dc/registry --remove --name=v1

	  # Create a new persistent volume claim that overwrites an existing volume 'v1'
	  %[1]s volume dc/registry --add --name=v1 -t pvc --claim-size=1G --overwrite

	  # Change the mount point for volume 'v1' to /data
	  %[1]s volume dc/registry --add --name=v1 -m /data --overwrite

	  # Modify the deployment config by removing volume mount "v1" from container "c1"
	  # (and by removing the volume "v1" if no other containers have volume mounts that reference it)
	  %[1]s volume dc/registry --remove --name=v1 --containers=c1

	  # Add new volume based on a more complex volume source (Git repo, AWS EBS, GCE PD,
	  # Ceph, Gluster, NFS, ISCSI, ...)
	  %[1]s volume dc/registry --add -m /repo --source=<json-string>`)
)

type VolumeOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	DefaultNamespace       string
	ExplicitNamespace      bool
	Mapper                 meta.RESTMapper
	Client                 kcoreclient.CoreInterface
	UpdatePodSpecForObject polymorphichelpers.UpdatePodSpecForObjectFunc
	Encoder                runtime.Encoder
	Builder                func() *resource.Builder

	// Resource selection
	Selector string
	All      bool

	// Operations
	Add    bool
	Remove bool
	List   bool

	// Common optional params
	Name       string
	Containers string
	Confirm    bool
	Local      bool
	Args       []string
	Printer    printers.ResourcePrinter
	DryRun     bool

	// Add op params
	AddOpts *AddVolumeOptions

	resource.FilenameOptions
	genericclioptions.IOStreams
}

type AddVolumeOptions struct {
	Type          string
	MountPath     string
	SubPath       string
	DefaultMode   string
	Overwrite     bool
	Path          string
	ConfigMapName string
	SecretName    string
	Source        string

	ReadOnly    bool
	CreateClaim bool
	ClaimName   string
	ClaimSize   string
	ClaimMode   string
	ClaimClass  string

	TypeChanged bool
}

func NewVolumeOptions(streams genericclioptions.IOStreams) *VolumeOptions {
	return &VolumeOptions{
		PrintFlags: genericclioptions.NewPrintFlags("volume updated").WithTypeSetter(scheme.Scheme),

		Containers: "*",
		AddOpts: &AddVolumeOptions{
			ClaimMode: "ReadWriteOnce",
		},
		IOStreams: streams,
	}
}

func NewCmdVolume(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewVolumeOptions(streams)
	cmd := &cobra.Command{
		Use:     "volumes RESOURCE/NAME --add|--remove|--list",
		Short:   "Update volumes on a pod template",
		Long:    volumeLong,
		Example: fmt.Sprintf(volumeExample, fullName),
		Aliases: []string{"volume"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunVolume())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().BoolVar(&o.Add, "add", o.Add, "If true, add volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&o.Remove, "remove", o.Remove, "If true, remove volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&o.List, "list", o.List, "If true, list volumes and volume mounts for containers")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set image will NOT contact api-server but run locally.")
	cmd.Flags().StringVar(&o.Name, "name", o.Name, "Name of the volume. If empty, auto generated for add operation")
	cmd.Flags().StringVarP(&o.Containers, "containers", "c", o.Containers, "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().BoolVar(&o.Confirm, "confirm", o.Confirm, "If true, confirm that you really want to remove multiple volumes")

	cmd.Flags().StringVarP(&o.AddOpts.Type, "type", "t", o.AddOpts.Type, "Type of the volume source for add operation. Supported options: emptyDir, hostPath, secret, configmap, persistentVolumeClaim")
	cmd.Flags().StringVarP(&o.AddOpts.MountPath, "mount-path", "m", o.AddOpts.MountPath, "Mount path inside the container. Optional param for --add or --remove")
	cmd.Flags().StringVar(&o.AddOpts.SubPath, "sub-path", o.AddOpts.SubPath, "Path within the local volume from which the container's volume should be mounted. Optional param for --add or --remove")
	cmd.Flags().StringVar(&o.AddOpts.DefaultMode, "default-mode", o.AddOpts.DefaultMode, "The default mode bits to create files with. Can be between 0000 and 0777. Defaults to 0644.")
	cmd.Flags().BoolVar(&o.AddOpts.ReadOnly, "read-only", o.AddOpts.ReadOnly, "Mount volume as ReadOnly. Optional param for --add or --remove")
	cmd.Flags().BoolVar(&o.AddOpts.Overwrite, "overwrite", o.AddOpts.Overwrite, "If true, replace existing volume source with the provided name and/or volume mount for the given resource")
	cmd.Flags().StringVar(&o.AddOpts.Path, "path", o.AddOpts.Path, "Host path. Must be provided for hostPath volume type")
	cmd.Flags().StringVar(&o.AddOpts.ConfigMapName, "configmap-name", o.AddOpts.ConfigMapName, "Name of the persisted config map. Must be provided for configmap volume type")
	cmd.Flags().StringVar(&o.AddOpts.SecretName, "secret-name", o.AddOpts.SecretName, "Name of the persisted secret. Must be provided for secret volume type")
	cmd.Flags().StringVar(&o.AddOpts.ClaimName, "claim-name", o.AddOpts.ClaimName, "Persistent volume claim name. Must be provided for persistentVolumeClaim volume type")
	cmd.Flags().StringVar(&o.AddOpts.ClaimClass, "claim-class", o.AddOpts.ClaimClass, "StorageClass to use for the persistent volume claim")
	cmd.Flags().StringVar(&o.AddOpts.ClaimSize, "claim-size", o.AddOpts.ClaimSize, "If specified along with a persistent volume type, create a new claim with the given size in bytes. Accepts SI notation: 10, 10G, 10Gi")
	cmd.Flags().StringVar(&o.AddOpts.ClaimMode, "claim-mode", o.AddOpts.ClaimMode, "Set the access mode of the claim to be created. Valid values are ReadWriteOnce (rwo), ReadWriteMany (rwm), or ReadOnlyMany (rom)")
	cmd.Flags().StringVar(&o.AddOpts.Source, "source", o.AddOpts.Source, "Details of volume source as json string. This can be used if the required volume type is not supported by --type option. (e.g.: '{\"gitRepo\": {\"repository\": <git-url>, \"revision\": <commit-hash>}}')")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	// deprecate --list option
	cmd.Flags().MarkDeprecated("list", "Volumes and volume mounts can be listed by providing a resource with no additional options.")

	return cmd
}

func (o *VolumeOptions) Validate() error {
	if len(o.Selector) > 0 {
		if _, err := labels.Parse(o.Selector); err != nil {
			return errors.New("--selector=<selector> must be a valid label selector")
		}
		if o.All {
			return errors.New("you may specify either --selector or --all but not both")
		}
	}
	if len(o.Filenames) == 0 && len(o.Args) < 1 {
		return errors.New("provide one or more resources to add, list, or delete volumes on as TYPE/NAME")
	}

	if o.List && o.PrintFlags.OutputFormat != nil && len(*o.PrintFlags.OutputFormat) > 0 {
		return errors.New("--list and --output may not be specified together")
	}

	if o.Add {
		err := o.AddOpts.Validate()
		if err != nil {
			return err
		}
	} else if len(o.AddOpts.Source) > 0 || len(o.AddOpts.Path) > 0 || len(o.AddOpts.SecretName) > 0 ||
		len(o.AddOpts.ConfigMapName) > 0 || len(o.AddOpts.ClaimName) > 0 || len(o.AddOpts.DefaultMode) > 0 ||
		o.AddOpts.Overwrite {
		return errors.New("--type|--path|--configmap-name|--secret-name|--claim-name|--source|--default-mode|--overwrite are only valid for --add operation")
	}
	// Removing all volumes for the resource type needs confirmation
	if o.Remove && len(o.Name) == 0 && !o.Confirm {
		return errors.New("must provide --confirm for removing more than one volume")
	}
	return nil
}

func (a *AddVolumeOptions) Validate() error {
	if len(a.Type) == 0 && len(a.Source) == 0 {
		return errors.New("must provide --type or --source for --add operation")
	} else if a.TypeChanged && len(a.Source) > 0 {
		return errors.New("either specify --type or --source but not both for --add operation")
	}

	if len(a.Type) > 0 {
		switch strings.ToLower(a.Type) {
		case "emptydir":
		case "hostpath":
			if len(a.Path) == 0 {
				return errors.New("must provide --path for --type=hostPath")
			}
		case "secret":
			if len(a.SecretName) == 0 {
				return errors.New("must provide --secret-name for --type=secret")
			}
			if ok, _ := regexp.MatchString(`\b0?[0-7]{3}\b`, a.DefaultMode); !ok {
				return errors.New("--default-mode must be between 0000 and 0777")
			}
		case "configmap":
			if len(a.ConfigMapName) == 0 {
				return errors.New("must provide --configmap-name for --type=configmap")
			}
			if ok, _ := regexp.MatchString(`\b0?[0-7]{3}\b`, a.DefaultMode); !ok {
				return errors.New("--default-mode must be between 0000 and 0777")
			}
		case "persistentvolumeclaim", "pvc":
			if len(a.ClaimName) == 0 && len(a.ClaimSize) == 0 {
				return errors.New("must provide --claim-name or --claim-size (to create a new claim) for --type=pvc")
			}
		default:
			return errors.New("invalid volume type. Supported types: emptyDir, hostPath, secret, persistentVolumeClaim")
		}
	} else if len(a.Path) > 0 || len(a.SecretName) > 0 || len(a.ClaimName) > 0 {
		return errors.New("--path|--secret-name|--claim-name are only valid for --type option")
	}

	if len(a.Source) > 0 {
		var source map[string]interface{}
		err := json.Unmarshal([]byte(a.Source), &source)
		if err != nil {
			return err
		}
		if len(source) > 1 {
			return errors.New("must provide only one volume for --source")
		}

		var vs kapi.VolumeSource
		err = json.Unmarshal([]byte(a.Source), &vs)
		if err != nil {
			return err
		}
	}
	if len(a.ClaimClass) > 0 {
		selectedLowerType := strings.ToLower(a.Type)
		if selectedLowerType != "persistentvolumeclaim" && selectedLowerType != "pvc" {
			return errors.New("must provide --type as persistentVolumeClaim")
		}
		if len(a.ClaimSize) == 0 {
			return errors.New("must provide --claim-size to create new pvc with claim-class")
		}
	}
	return nil
}

func (o *VolumeOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	kc, err := f.ClientSet()
	if err != nil {
		return err
	}
	o.Client = kc.Core()

	o.DefaultNamespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	numOps := 0
	if o.Add {
		numOps++
	}
	if o.Remove {
		numOps++
	}
	if o.List {
		numOps++
	}

	switch {
	case numOps == 0:
		o.List = true
	case numOps > 1:
		return errors.New("you may only specify one operation at a time")
	}

	o.Args = args
	o.Builder = f.NewBuilder
	o.Encoder = kcmdutil.InternalVersionJSONEncoder()
	o.UpdatePodSpecForObject = polymorphichelpers.UpdatePodSpecForObjectFn

	o.AddOpts.TypeChanged = cmd.Flag("type").Changed

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	// Complete AddOpts
	if o.Add {
		if err := o.AddOpts.Complete(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AddVolumeOptions) Complete() error {
	if len(a.Type) == 0 {
		switch {
		case len(a.ClaimName) > 0 || len(a.ClaimSize) > 0:
			a.Type = "persistentvolumeclaim"
			a.TypeChanged = true
		case len(a.SecretName) > 0:
			a.Type = "secret"
			a.TypeChanged = true
		case len(a.ConfigMapName) > 0:
			a.Type = "configmap"
			a.TypeChanged = true
		case len(a.Path) > 0:
			a.Type = "hostpath"
			a.TypeChanged = true
		default:
			a.Type = "emptydir"
		}
	}
	if a.Type == "configmap" || a.Type == "secret" {
		if len(a.DefaultMode) == 0 {
			a.DefaultMode = "644"
		}
	} else {
		if len(a.DefaultMode) != 0 {
			return errors.New("--default-mode is only available for secrets and configmaps")
		}
	}

	// In case of volume source ignore the default volume type
	if len(a.Source) > 0 {
		a.Type = ""
	}
	if len(a.ClaimSize) > 0 {
		a.CreateClaim = true
		if len(a.ClaimName) == 0 {
			a.ClaimName = names.SimpleNameGenerator.GenerateName("pvc-")
		}
		q, err := kresource.ParseQuantity(a.ClaimSize)
		if err != nil {
			return fmt.Errorf("--claim-size is not valid: %v", err)
		}
		a.ClaimSize = q.String()
	}
	switch strings.ToLower(a.ClaimMode) {
	case strings.ToLower(string(kapi.ReadOnlyMany)), "rom":
		a.ClaimMode = string(kapi.ReadOnlyMany)
	case strings.ToLower(string(kapi.ReadWriteOnce)), "rwo":
		a.ClaimMode = string(kapi.ReadWriteOnce)
	case strings.ToLower(string(kapi.ReadWriteMany)), "rwm":
		a.ClaimMode = string(kapi.ReadWriteMany)
	case "":
	default:
		return errors.New("--claim-mode must be one of ReadWriteOnce (rwo), ReadWriteMany (rwm), or ReadOnlyMany (rom)")
	}

	return nil
}

func (o *VolumeOptions) RunVolume() error {
	b := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.DefaultNamespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()

	if !o.Local {
		b = b.
			LabelSelectorParam(o.Selector).
			ResourceTypeOrNameArgs(o.All, o.Args...).
			Latest()
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}
	if o.List {
		if listingErrors := o.printVolumes(infos); len(listingErrors) > 0 {
			return kcmdutil.ErrExit
		}
		return nil
	}

	updateInfos := []*resource.Info{}
	// if a claim should be created, generate the info we'll add to the flow
	if o.Add && o.AddOpts.CreateClaim {
		claim := o.AddOpts.createClaim()
		m, err := o.Mapper.RESTMapping(kapi.Kind("PersistentVolumeClaim"))
		if err != nil {
			return err
		}
		info := &resource.Info{
			Mapping:   m,
			Client:    o.Client.RESTClient(),
			Namespace: o.DefaultNamespace,
			Object:    claim,
		}
		infos = append(infos, info)
		updateInfos = append(updateInfos, info)
	}

	patches, patchError := o.getVolumeUpdatePatches(infos, singleItemImplied)

	if patchError != nil {
		return patchError
	}
	if o.Local || o.DryRun {
		for _, info := range infos {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				return err
			}
		}
	}

	allErrs := []error{}
	for _, info := range updateInfos {
		var obj runtime.Object
		if len(info.ResourceVersion) == 0 {
			obj, err = resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, false, info.Object)
		} else {
			obj, err = resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, true, info.Object)
		}
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch volume update to pod template: %v\n", err))
			continue
		}
		info.Refresh(obj, true)
	}
	for _, patch := range patches {
		info := patch.Info
		name := getObjectName(info)
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s %v\n", name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			glog.V(1).Infof("info: %s was not changed\n", name)
			continue
		}

		if o.Local || o.DryRun {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch volume update to pod template: %v\n", err))
			continue
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o *VolumeOptions) getVolumeUpdatePatches(infos []*resource.Info, singleItemImplied bool) ([]*Patch, error) {
	skipped := 0
	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
		transformed := false
		ok, err := o.UpdatePodSpecForObject(info.Object, clientcmd.ConvertInteralPodSpecToExternal(func(spec *kapi.PodSpec) error {
			var e error
			switch {
			case o.Add:
				e = o.addVolumeToSpec(spec, info, singleItemImplied)
				transformed = true
			case o.Remove:
				e = o.removeVolumeFromSpec(spec, info)
				transformed = true
			}
			return e
		}))
		if !ok {
			skipped++
		}
		return transformed, err
	})
	if singleItemImplied && skipped == len(infos) {
		patchError := fmt.Errorf("the %s %s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
		return patches, patchError
	}
	return patches, nil
}

func setVolumeSourceByType(kv *kapi.Volume, opts *AddVolumeOptions) error {
	switch strings.ToLower(opts.Type) {
	case "emptydir":
		kv.EmptyDir = &kapi.EmptyDirVolumeSource{}
	case "hostpath":
		kv.HostPath = &kapi.HostPathVolumeSource{
			Path: opts.Path,
		}
	case "secret":
		defaultMode, err := strconv.ParseUint(opts.DefaultMode, 8, 32)
		if err != nil {
			return err
		}
		defaultMode32 := int32(defaultMode)
		kv.Secret = &kapi.SecretVolumeSource{
			SecretName:  opts.SecretName,
			DefaultMode: &defaultMode32,
		}
	case "configmap":
		defaultMode, err := strconv.ParseUint(opts.DefaultMode, 8, 32)
		if err != nil {
			return err
		}
		defaultMode32 := int32(defaultMode)
		kv.ConfigMap = &kapi.ConfigMapVolumeSource{
			LocalObjectReference: kapi.LocalObjectReference{
				Name: opts.ConfigMapName,
			},
			DefaultMode: &defaultMode32,
		}
	case "persistentvolumeclaim", "pvc":
		kv.PersistentVolumeClaim = &kapi.PersistentVolumeClaimVolumeSource{
			ClaimName: opts.ClaimName,
		}
	default:
		return fmt.Errorf("invalid volume type: %s", opts.Type)
	}
	return nil
}

func (o *VolumeOptions) printVolumes(infos []*resource.Info) []error {
	listingErrors := []error{}
	for _, info := range infos {
		_, err := o.UpdatePodSpecForObject(info.Object, clientcmd.ConvertInteralPodSpecToExternal(func(spec *kapi.PodSpec) error {
			return o.listVolumeForSpec(spec, info)
		}))
		if err != nil {
			listingErrors = append(listingErrors, err)
			fmt.Fprintf(o.ErrOut, "error: %s %v\n", getObjectName(info), err)
		}
	}
	return listingErrors
}

func (a *AddVolumeOptions) createClaim() *kapi.PersistentVolumeClaim {
	pvc := &kapi.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.ClaimName,
		},
		Spec: kapi.PersistentVolumeClaimSpec{
			AccessModes: []kapi.PersistentVolumeAccessMode{kapi.PersistentVolumeAccessMode(a.ClaimMode)},
			Resources: kapi.ResourceRequirements{
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceStorage): kresource.MustParse(a.ClaimSize),
				},
			},
		},
	}
	if len(a.ClaimClass) > 0 {
		pvc.Annotations = map[string]string{
			storageAnnClass: a.ClaimClass,
		}
	}
	return pvc
}

func (o *VolumeOptions) setVolumeSource(kv *kapi.Volume) error {
	var err error
	opts := o.AddOpts
	if len(opts.Type) > 0 {
		err = setVolumeSourceByType(kv, opts)
	} else if len(opts.Source) > 0 {
		err = json.Unmarshal([]byte(opts.Source), &kv.VolumeSource)
	}
	return err
}

func (o *VolumeOptions) setVolumeMount(spec *kapi.PodSpec, info *resource.Info) error {
	opts := o.AddOpts
	containers, _ := selectContainers(spec.Containers, o.Containers)
	if len(containers) == 0 && o.Containers != "*" {
		fmt.Fprintf(o.ErrOut, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource.Resource, info.Name, o.Containers)
		return nil
	}

	for _, c := range containers {
		for _, m := range c.VolumeMounts {
			if path.Clean(m.MountPath) == path.Clean(opts.MountPath) && m.Name != o.Name {
				return fmt.Errorf("volume mount '%s' already exists for container '%s'", opts.MountPath, c.Name)
			}
		}
		for i, m := range c.VolumeMounts {
			if m.Name == o.Name && opts.Overwrite {
				c.VolumeMounts = append(c.VolumeMounts[:i], c.VolumeMounts[i+1:]...)
				break
			}
		}
		volumeMount := &kapi.VolumeMount{
			Name:      o.Name,
			MountPath: path.Clean(opts.MountPath),
			ReadOnly:  opts.ReadOnly,
		}
		if len(opts.SubPath) > 0 {
			volumeMount.SubPath = path.Clean(opts.SubPath)
		}
		c.VolumeMounts = append(c.VolumeMounts, *volumeMount)
	}
	return nil
}

func (o *VolumeOptions) getVolumeName(spec *kapi.PodSpec, singleResource bool) (string, bool, error) {
	opts := o.AddOpts
	if opts.Overwrite {
		// Multiple resources can have same mount-path for different volumes,
		// so restrict it for single resource to uniquely find the volume
		if !singleResource {
			return "", false, fmt.Errorf("you must specify --name for the volume name when dealing with multiple resources")
		}
		if len(opts.MountPath) > 0 {
			containers, _ := selectContainers(spec.Containers, o.Containers)
			var name string
			matchCount := 0
			for _, c := range containers {
				for _, m := range c.VolumeMounts {
					if path.Clean(m.MountPath) == path.Clean(opts.MountPath) {
						name = m.Name
						matchCount++
						break
					}
				}
			}

			switch matchCount {
			case 0:
				return "", false, fmt.Errorf("unable to find the volume for mount-path: %s", opts.MountPath)
			case 1:
				return name, false, nil
			default:
				return "", false, fmt.Errorf("found multiple volumes with same mount-path: %s", opts.MountPath)
			}
		}
		return "", false, fmt.Errorf("ambiguous --overwrite, specify --name or --mount-path")
	}

	oldName, claimFound := o.checkForExistingClaim(spec)

	if claimFound {
		return oldName, true, nil
	}
	// Generate volume name
	name := names.SimpleNameGenerator.GenerateName(volumePrefix)
	if o.PrintFlags.OutputFormat == nil || len(*o.PrintFlags.OutputFormat) == 0 {
		fmt.Fprintf(o.ErrOut, "info: Generated volume name: %s\n", name)
	}
	return name, false, nil
}

func (o *VolumeOptions) checkForExistingClaim(spec *kapi.PodSpec) (string, bool) {
	for _, vol := range spec.Volumes {
		oldSource := vol.VolumeSource.PersistentVolumeClaim
		if oldSource != nil && o.AddOpts.ClaimName == oldSource.ClaimName {
			return vol.Name, true
		}
	}
	return "", false
}

func (o *VolumeOptions) addVolumeToSpec(spec *kapi.PodSpec, info *resource.Info, singleResource bool) error {
	opts := o.AddOpts
	claimFound := false
	if len(o.Name) == 0 {
		var err error
		o.Name, claimFound, err = o.getVolumeName(spec, singleResource)
		if err != nil {
			return err
		}
	} else {
		_, claimFound = o.checkForExistingClaim(spec)
	}

	newVolume := &kapi.Volume{
		Name: o.Name,
	}
	setSource := true
	vNameFound := false
	for i, vol := range spec.Volumes {
		if o.Name == vol.Name {
			vNameFound = true
			if !opts.Overwrite && !claimFound {
				return fmt.Errorf("volume '%s' already exists. Use --overwrite to replace", o.Name)
			}
			if !opts.TypeChanged && len(opts.Source) == 0 {
				newVolume.VolumeSource = vol.VolumeSource
				setSource = false
			}
			spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}
	// if --overwrite was passed, but volume did not previously
	// exist, log a warning that no volumes were overwritten
	if !vNameFound && opts.Overwrite && (o.PrintFlags.OutputFormat == nil || len(*o.PrintFlags.OutputFormat) == 0) {
		fmt.Fprintf(o.ErrOut, "warning: volume %q did not previously exist and was not overriden. A new volume with this name has been created instead.", o.Name)
	}

	if setSource {
		err := o.setVolumeSource(newVolume)
		if err != nil {
			return err
		}
	}
	spec.Volumes = append(spec.Volumes, *newVolume)

	if len(opts.MountPath) > 0 {
		err := o.setVolumeMount(spec, info)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *VolumeOptions) removeSpecificVolume(spec *kapi.PodSpec, containers, skippedContainers []*kapi.Container) error {
	for _, c := range containers {
		newMounts := c.VolumeMounts[:0]
		for _, m := range c.VolumeMounts {
			// Remove all volume mounts that match specified name
			if o.Name != m.Name {
				newMounts = append(newMounts, m)
			}
		}
		c.VolumeMounts = newMounts
	}

	// Remove volume if no container is using it
	found := false
	for _, c := range skippedContainers {
		for _, m := range c.VolumeMounts {
			if o.Name == m.Name {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		foundVolume := false
		for i, vol := range spec.Volumes {
			if o.Name == vol.Name {
				spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
				foundVolume = true
				break
			}
		}
		if !foundVolume {
			return fmt.Errorf("volume '%s' not found", o.Name)
		}
	}
	return nil
}

func (o *VolumeOptions) removeVolumeFromSpec(spec *kapi.PodSpec, info *resource.Info) error {
	containers, skippedContainers := selectContainers(spec.Containers, o.Containers)
	if len(containers) == 0 && o.Containers != "*" {
		fmt.Fprintf(o.ErrOut, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource.Resource, info.Name, o.Containers)
		return nil
	}

	if len(o.Name) == 0 {
		for _, c := range containers {
			c.VolumeMounts = []kapi.VolumeMount{}
		}
		spec.Volumes = []kapi.Volume{}
	} else {
		err := o.removeSpecificVolume(spec, containers, skippedContainers)
		if err != nil {
			return err
		}
	}
	return nil
}

func sourceAccessMode(readOnly bool) string {
	if readOnly {
		return " read-only"
	}
	return ""
}

func describePersistentVolumeClaim(claim *kapi.PersistentVolumeClaim) string {
	if len(claim.Spec.VolumeName) == 0 {
		// TODO: check for other dimensions of request - IOPs, etc
		if val, ok := claim.Spec.Resources.Requests[kapi.ResourceStorage]; ok {
			return fmt.Sprintf("waiting for %sB allocation", val.String())
		}
		return "waiting to allocate"
	}
	// TODO: check for other dimensions of capacity?
	if val, ok := claim.Status.Capacity[kapi.ResourceStorage]; ok {
		return fmt.Sprintf("allocated %sB", val.String())
	}
	return "allocated unknown size"
}

func describeVolumeSource(source *kapi.VolumeSource) string {
	switch {
	case source.AWSElasticBlockStore != nil:
		return fmt.Sprintf("AWS EBS %s type=%s partition=%d%s", source.AWSElasticBlockStore.VolumeID, source.AWSElasticBlockStore.FSType, source.AWSElasticBlockStore.Partition, sourceAccessMode(source.AWSElasticBlockStore.ReadOnly))
	case source.EmptyDir != nil:
		return "empty directory"
	case source.GCEPersistentDisk != nil:
		return fmt.Sprintf("GCE PD %s type=%s partition=%d%s", source.GCEPersistentDisk.PDName, source.GCEPersistentDisk.FSType, source.GCEPersistentDisk.Partition, sourceAccessMode(source.GCEPersistentDisk.ReadOnly))
	case source.GitRepo != nil:
		if len(source.GitRepo.Revision) == 0 {
			return fmt.Sprintf("Git repository %s", source.GitRepo.Repository)
		}
		return fmt.Sprintf("Git repository %s @ %s", source.GitRepo.Repository, source.GitRepo.Revision)
	case source.Glusterfs != nil:
		return fmt.Sprintf("GlusterFS %s:%s%s", source.Glusterfs.EndpointsName, source.Glusterfs.Path, sourceAccessMode(source.Glusterfs.ReadOnly))
	case source.HostPath != nil:
		return fmt.Sprintf("host path %s", source.HostPath.Path)
	case source.ISCSI != nil:
		return fmt.Sprintf("ISCSI %s target-portal=%s type=%s lun=%d%s", source.ISCSI.IQN, source.ISCSI.TargetPortal, source.ISCSI.FSType, source.ISCSI.Lun, sourceAccessMode(source.ISCSI.ReadOnly))
	case source.NFS != nil:
		return fmt.Sprintf("NFS %s:%s%s", source.NFS.Server, source.NFS.Path, sourceAccessMode(source.NFS.ReadOnly))
	case source.PersistentVolumeClaim != nil:
		return fmt.Sprintf("pvc/%s%s", source.PersistentVolumeClaim.ClaimName, sourceAccessMode(source.PersistentVolumeClaim.ReadOnly))
	case source.RBD != nil:
		return fmt.Sprintf("Ceph RBD %v type=%s image=%s pool=%s%s", source.RBD.CephMonitors, source.RBD.FSType, source.RBD.RBDImage, source.RBD.RBDPool, sourceAccessMode(source.RBD.ReadOnly))
	case source.Secret != nil:
		return fmt.Sprintf("secret/%s", source.Secret.SecretName)
	case source.ConfigMap != nil:
		return fmt.Sprintf("configMap/%s", source.ConfigMap.Name)
	default:
		return "unknown"
	}
}

func (o *VolumeOptions) listVolumeForSpec(spec *kapi.PodSpec, info *resource.Info) error {
	containers, _ := selectContainers(spec.Containers, o.Containers)
	if len(containers) == 0 && o.Containers != "*" {
		fmt.Fprintf(o.ErrOut, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource.Resource, info.Name, o.Containers)
		return nil
	}

	fmt.Fprintf(o.Out, "%s/%s\n", info.Mapping.Resource.Resource, info.Name)
	checkName := (len(o.Name) > 0)
	found := false
	for _, vol := range spec.Volumes {
		if checkName && o.Name != vol.Name {
			continue
		}
		found = true

		refInfo := ""
		if vol.VolumeSource.PersistentVolumeClaim != nil {
			claimName := vol.VolumeSource.PersistentVolumeClaim.ClaimName
			claim, err := o.Client.PersistentVolumeClaims(info.Namespace).Get(claimName, metav1.GetOptions{})
			switch {
			case err == nil:
				refInfo = fmt.Sprintf("(%s)", describePersistentVolumeClaim(claim))
			case apierrs.IsNotFound(err):
				refInfo = "(does not exist)"
			default:
				fmt.Fprintf(o.ErrOut, "error: unable to retrieve persistent volume claim %s referenced in %s/%s: %v", claimName, info.Mapping.Resource.Resource, info.Name, err)
			}
		}
		if len(refInfo) > 0 {
			refInfo = " " + refInfo
		}

		fmt.Fprintf(o.Out, "  %s%s as %s\n", describeVolumeSource(&vol.VolumeSource), refInfo, vol.Name)
		for _, c := range containers {
			for _, m := range c.VolumeMounts {
				if vol.Name != m.Name {
					continue
				}
				if len(spec.Containers) == 1 {
					fmt.Fprintf(o.Out, "    mounted at %s\n", m.MountPath)
				} else {
					fmt.Fprintf(o.Out, "    mounted at %s in container %s\n", m.MountPath, c.Name)
				}
			}
		}
	}
	if checkName && !found {
		return fmt.Errorf("volume %q not found", o.Name)
	}

	return nil
}
