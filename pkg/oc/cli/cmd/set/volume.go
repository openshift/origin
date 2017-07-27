package set

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
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
	DefaultNamespace       string
	ExplicitNamespace      bool
	Out                    io.Writer
	Err                    io.Writer
	Mapper                 meta.RESTMapper
	Typer                  runtime.ObjectTyper
	CategoryExpander       resource.CategoryExpander
	RESTClientFactory      func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	UpdatePodSpecForObject func(obj runtime.Object, fn func(*kapi.PodSpec) error) (bool, error)
	Client                 kcoreclient.PersistentVolumeClaimsGetter
	Encoder                runtime.Encoder
	Cmd                    *cobra.Command

	// Resource selection
	Selector  string
	All       bool
	Filenames []string

	// Operations
	Add    bool
	Remove bool
	List   bool

	// Common optional params
	Name        string
	Containers  string
	Confirm     bool
	Local       bool
	Output      string
	PrintObject func([]*resource.Info) error

	// Add op params
	AddOpts *AddVolumeOptions
}

type AddVolumeOptions struct {
	Type          string
	MountPath     string
	DefaultMode   string
	Overwrite     bool
	Path          string
	ConfigMapName string
	SecretName    string
	Source        string

	CreateClaim bool
	ClaimName   string
	ClaimSize   string
	ClaimMode   string
	ClaimClass  string

	TypeChanged bool
}

func NewCmdVolume(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	addOpts := &AddVolumeOptions{}
	opts := &VolumeOptions{AddOpts: addOpts}
	cmd := &cobra.Command{
		Use:     "volumes RESOURCE/NAME --add|--remove|--list",
		Short:   "Update volumes on a pod template",
		Long:    volumeLong,
		Example: fmt.Sprintf(volumeExample, fullName),
		Aliases: []string{"volume"},
		Run: func(cmd *cobra.Command, args []string) {
			addOpts.TypeChanged = cmd.Flag("type").Changed

			err := opts.Validate(cmd, args)
			if err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			err = opts.Complete(f, cmd, out, errOut)
			kcmdutil.CheckErr(err)

			err = opts.RunVolume(args)
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&opts.Selector, "selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&opts.All, "all", false, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().StringSliceVarP(&opts.Filenames, "filename", "f", opts.Filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().BoolVar(&opts.Add, "add", false, "If true, add volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&opts.Remove, "remove", false, "If true, remove volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&opts.List, "list", false, "If true, list volumes and volume mounts for containers")
	cmd.Flags().BoolVar(&opts.Local, "local", false, "If true, set image will NOT contact api-server but run locally.")

	cmd.Flags().StringVar(&opts.Name, "name", "", "Name of the volume. If empty, auto generated for add operation")
	cmd.Flags().StringVarP(&opts.Containers, "containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().BoolVar(&opts.Confirm, "confirm", false, "If true, confirm that you really want to remove multiple volumes")

	cmd.Flags().StringVarP(&addOpts.Type, "type", "t", "", "Type of the volume source for add operation. Supported options: emptyDir, hostPath, secret, configmap, persistentVolumeClaim")
	cmd.Flags().StringVarP(&addOpts.MountPath, "mount-path", "m", "", "Mount path inside the container. Optional param for --add or --remove")
	cmd.Flags().StringVarP(&addOpts.DefaultMode, "default-mode", "", "", "The default mode bits to create files with. Can be between 0000 and 0777. Defaults to 0644.")
	cmd.Flags().BoolVar(&addOpts.Overwrite, "overwrite", false, "If true, replace existing volume source with the provided name and/or volume mount for the given resource")
	cmd.Flags().StringVar(&addOpts.Path, "path", "", "Host path. Must be provided for hostPath volume type")
	cmd.Flags().StringVar(&addOpts.ConfigMapName, "configmap-name", "", "Name of the persisted config map. Must be provided for configmap volume type")
	cmd.Flags().StringVar(&addOpts.SecretName, "secret-name", "", "Name of the persisted secret. Must be provided for secret volume type")
	cmd.Flags().StringVar(&addOpts.ClaimName, "claim-name", "", "Persistent volume claim name. Must be provided for persistentVolumeClaim volume type")
	cmd.Flags().StringVar(&addOpts.ClaimClass, "claim-class", "", "StorageClass to use for the persistent volume claim")
	cmd.Flags().StringVar(&addOpts.ClaimSize, "claim-size", "", "If specified along with a persistent volume type, create a new claim with the given size in bytes. Accepts SI notation: 10, 10G, 10Gi")
	cmd.Flags().StringVar(&addOpts.ClaimMode, "claim-mode", "ReadWriteOnce", "Set the access mode of the claim to be created. Valid values are ReadWriteOnce (rwo), ReadWriteMany (rwm), or ReadOnlyMany (rom)")
	cmd.Flags().StringVar(&addOpts.Source, "source", "", "Details of volume source as json string. This can be used if the required volume type is not supported by --type option. (e.g.: '{\"gitRepo\": {\"repository\": <git-url>, \"revision\": <commit-hash>}}')")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	// deprecate --list option
	cmd.Flags().MarkDeprecated("list", "Volumes and volume mounts can be listed by providing a resource with no additional options.")

	return cmd
}

func (v *VolumeOptions) Validate(cmd *cobra.Command, args []string) error {
	if len(v.Selector) > 0 {
		if _, err := labels.Parse(v.Selector); err != nil {
			return errors.New("--selector=<selector> must be a valid label selector")
		}
		if v.All {
			return errors.New("you may specify either --selector or --all but not both")
		}
	}
	if len(v.Filenames) == 0 && len(args) < 1 {
		return errors.New("provide one or more resources to add, list, or delete volumes on as TYPE/NAME")
	}

	numOps := 0
	if v.Add {
		numOps++
	}
	if v.Remove {
		numOps++
	}
	if v.List {
		numOps++
	}

	switch {
	case numOps == 0:
		v.List = true
	case numOps > 1:
		return errors.New("you may only specify one operation at a time")
	}

	output := kcmdutil.GetFlagString(cmd, "output")
	if v.List && len(output) > 0 {
		return errors.New("--list and --output may not be specified together")
	}

	err := v.AddOpts.Validate(v.Add)
	if err != nil {
		return err
	}
	// Removing all volumes for the resource type needs confirmation
	if v.Remove && len(v.Name) == 0 && !v.Confirm {
		return errors.New("must provide --confirm for removing more than one volume")
	}
	return nil
}

func (a *AddVolumeOptions) Validate(isAddOp bool) error {
	if isAddOp {
		if len(a.Type) == 0 && (len(a.ClaimName) > 0 || len(a.ClaimSize) > 0) {
			a.Type = "persistentvolumeclaim"
			a.TypeChanged = true
		}
		if len(a.Type) == 0 && (len(a.SecretName) > 0) {
			a.Type = "secret"
			a.TypeChanged = true
		}
		if len(a.Type) == 0 && (len(a.ConfigMapName) > 0) {
			a.Type = "configmap"
			a.TypeChanged = true
		}
		if len(a.Type) == 0 && (len(a.Path) > 0) {
			a.Type = "hostpath"
			a.TypeChanged = true
		}
		if len(a.Type) == 0 {
			a.Type = "emptydir"
		}

		if len(a.Type) == 0 && len(a.Source) == 0 {
			return errors.New("must provide --type or --source for --add operation")
		} else if a.TypeChanged && len(a.Source) > 0 {
			return errors.New("either specify --type or --source but not both for --add operation")
		}

		if len(a.Type) > 0 {
			switch strings.ToLower(a.Type) {
			case "emptydir":
				if len(a.DefaultMode) > 0 {
					return errors.New("--default-mode is only available for secrets and configmaps")
				}
			case "hostpath":
				if len(a.Path) == 0 {
					return errors.New("must provide --path for --type=hostPath")
				}
				if len(a.DefaultMode) > 0 {
					return errors.New("--default-mode is only available for secrets and configmaps")
				}
			case "secret":
				if len(a.SecretName) == 0 {
					return errors.New("must provide --secret-name for --type=secret")
				}
				if len(a.DefaultMode) > 0 {
					if ok, _ := regexp.MatchString(`\b0?[0-7]{3}\b`, a.DefaultMode); !ok {
						return errors.New("--default-mode must be between 0000 and 0777")
					}
				}
			case "configmap":
				if len(a.ConfigMapName) == 0 {
					return errors.New("must provide --configmap-name for --type=configmap")
				}
				if len(a.DefaultMode) > 0 {
					if ok, _ := regexp.MatchString(`\b0?[0-7]{3}\b`, a.DefaultMode); !ok {
						return errors.New("--default-mode must be between 0000 and 0777")
					}
				}
			case "persistentvolumeclaim", "pvc":
				if len(a.ClaimName) == 0 && len(a.ClaimSize) == 0 {
					return errors.New("must provide --claim-name or --claim-size (to create a new claim) for --type=pvc")
				}
				if len(a.DefaultMode) > 0 {
					return errors.New("--default-mode is only available for secrets and configmaps")
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
	} else if len(a.Source) > 0 || len(a.Path) > 0 || len(a.SecretName) > 0 || len(a.ConfigMapName) > 0 || len(a.ClaimName) > 0 || a.Overwrite {
		return errors.New("--type|--path|--configmap-name|--secret-name|--claim-name|--source|--overwrite are only valid for --add operation")
	}
	return nil
}

func (v *VolumeOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, out, errOut io.Writer) error {
	_, kc, err := f.Clients()
	if err != nil {
		return err
	}
	v.Client = kc.Core()

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	mapper, typer := f.Object()

	v.Output = kcmdutil.GetFlagString(cmd, "output")
	v.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, infos, v.Out)
	}

	v.Cmd = cmd
	v.DefaultNamespace = cmdNamespace
	v.ExplicitNamespace = explicit
	v.Out = out
	v.Err = errOut
	v.Mapper = mapper
	v.Typer = typer
	v.CategoryExpander = f.CategoryExpander()
	v.RESTClientFactory = f.ClientForMapping
	v.UpdatePodSpecForObject = f.UpdatePodSpecForObject
	v.Encoder = f.JSONEncoder()

	// In case of volume source ignore the default volume type
	if len(v.AddOpts.Source) > 0 {
		v.AddOpts.Type = ""
	}
	if len(v.AddOpts.ClaimSize) > 0 {
		v.AddOpts.CreateClaim = true
		if len(v.AddOpts.ClaimName) == 0 {
			v.AddOpts.ClaimName = names.SimpleNameGenerator.GenerateName("pvc-")
		}
		q, err := kresource.ParseQuantity(v.AddOpts.ClaimSize)
		if err != nil {
			return fmt.Errorf("--claim-size is not valid: %v", err)
		}
		v.AddOpts.ClaimSize = q.String()
	}
	if len(v.AddOpts.DefaultMode) == 0 {
		v.AddOpts.DefaultMode = "644"
	}
	switch strings.ToLower(v.AddOpts.ClaimMode) {
	case strings.ToLower(string(kapi.ReadOnlyMany)), "rom":
		v.AddOpts.ClaimMode = string(kapi.ReadOnlyMany)
	case strings.ToLower(string(kapi.ReadWriteOnce)), "rwo":
		v.AddOpts.ClaimMode = string(kapi.ReadWriteOnce)
	case strings.ToLower(string(kapi.ReadWriteMany)), "rwm":
		v.AddOpts.ClaimMode = string(kapi.ReadWriteMany)
	case "":
	default:
		return errors.New("--claim-mode must be one of ReadWriteOnce (rwo), ReadWriteMany (rwm), or ReadOnlyMany (rom)")
	}
	return nil
}

func (v *VolumeOptions) RunVolume(args []string) error {
	mapper := resource.ClientMapperFunc(v.RESTClientFactory)
	b := resource.NewBuilder(v.Mapper, v.CategoryExpander, v.Typer, mapper, kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(v.DefaultNamespace).DefaultNamespace().
		FilenameParam(v.ExplicitNamespace, &resource.FilenameOptions{Recursive: false, Filenames: v.Filenames}).
		Flatten()

	if !v.Local {
		b = b.
			SelectorParam(v.Selector).
			ResourceTypeOrNameArgs(v.All, args...)
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	if v.List {
		listingErrors := v.printVolumes(infos)
		if len(listingErrors) > 0 {
			return cmdutil.ErrExit
		}
		return nil
	}

	updateInfos := []*resource.Info{}
	// if a claim should be created, generate the info we'll add to the flow
	if v.Add && v.AddOpts.CreateClaim {
		claim := v.AddOpts.createClaim()
		m, err := v.Mapper.RESTMapping(kapi.Kind("PersistentVolumeClaim"))
		if err != nil {
			return err
		}
		client, err := mapper.ClientForMapping(m)
		if err != nil {
			return err
		}
		info := &resource.Info{
			Mapping:   m,
			Client:    client,
			Namespace: v.DefaultNamespace,
			Object:    claim,
		}
		infos = append(infos, info)
		updateInfos = append(updateInfos, info)
	}

	patches, patchError := v.getVolumeUpdatePatches(infos, singleItemImplied)

	if patchError != nil {
		return patchError
	}
	if len(v.Output) > 0 || v.Local || kcmdutil.GetDryRunFlag(v.Cmd) {
		return v.PrintObject(infos)
	}

	failed := false
	for _, info := range updateInfos {
		var obj runtime.Object
		if len(info.ResourceVersion) == 0 {
			obj, err = resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, false, info.Object)
		} else {
			obj, err = resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, true, info.Object)
		}
		if err != nil {
			handlePodUpdateError(v.Err, err, "volume")
			failed = true
			continue
		}
		info.Refresh(obj, true)
		fmt.Fprintf(v.Out, "%s/%s\n", info.Mapping.Resource, info.Name)
	}
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			failed = true
			fmt.Fprintf(v.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err)
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(v.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		glog.V(4).Infof("Calculated patch %s", patch.Patch)

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			handlePodUpdateError(v.Err, err, "volume")
			failed = true
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(v.Mapper, false, v.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return cmdutil.ErrExit
	}
	return nil
}

func (v *VolumeOptions) getVolumeUpdatePatches(infos []*resource.Info, singleItemImplied bool) ([]*Patch, error) {
	skipped := 0
	patches := CalculatePatches(infos, v.Encoder, func(info *resource.Info) (bool, error) {
		transformed := false
		ok, err := v.UpdatePodSpecForObject(info.Object, func(spec *kapi.PodSpec) error {
			var e error
			switch {
			case v.Add:
				e = v.addVolumeToSpec(spec, info, singleItemImplied)
				transformed = true
			case v.Remove:
				e = v.removeVolumeFromSpec(spec, info)
				transformed = true
			}
			return e
		})
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

func (v *VolumeOptions) printVolumes(infos []*resource.Info) []error {
	listingErrors := []error{}
	for _, info := range infos {
		_, err := v.UpdatePodSpecForObject(info.Object, func(spec *kapi.PodSpec) error {
			return v.listVolumeForSpec(spec, info)
		})
		if err != nil {
			listingErrors = append(listingErrors, err)
			fmt.Fprintf(v.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
		}
	}
	return listingErrors
}

func (v *AddVolumeOptions) createClaim() *kapi.PersistentVolumeClaim {
	pvc := &kapi.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: v.ClaimName,
		},
		Spec: kapi.PersistentVolumeClaimSpec{
			AccessModes: []kapi.PersistentVolumeAccessMode{kapi.PersistentVolumeAccessMode(v.ClaimMode)},
			Resources: kapi.ResourceRequirements{
				Requests: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceStorage): kresource.MustParse(v.ClaimSize),
				},
			},
		},
	}
	if len(v.ClaimClass) > 0 {
		pvc.Annotations = map[string]string{
			storageAnnClass: v.ClaimClass,
		}
	}
	return pvc
}

func (v *VolumeOptions) setVolumeSource(kv *kapi.Volume) error {
	var err error
	opts := v.AddOpts
	if len(opts.Type) > 0 {
		err = setVolumeSourceByType(kv, opts)
	} else if len(opts.Source) > 0 {
		err = json.Unmarshal([]byte(opts.Source), &kv.VolumeSource)
	}
	return err
}

func (v *VolumeOptions) setVolumeMount(spec *kapi.PodSpec, info *resource.Info) error {
	opts := v.AddOpts
	containers, _ := selectContainers(spec.Containers, v.Containers)
	if len(containers) == 0 && v.Containers != "*" {
		fmt.Fprintf(v.Err, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, v.Containers)
		return nil
	}

	for _, c := range containers {
		for _, m := range c.VolumeMounts {
			if path.Clean(m.MountPath) == path.Clean(opts.MountPath) && m.Name != v.Name {
				return fmt.Errorf("volume mount '%s' already exists for container '%s'", opts.MountPath, c.Name)
			}
		}
		for i, m := range c.VolumeMounts {
			if m.Name == v.Name {
				c.VolumeMounts = append(c.VolumeMounts[:i], c.VolumeMounts[i+1:]...)
				break
			}
		}
		volumeMount := &kapi.VolumeMount{
			Name:      v.Name,
			MountPath: path.Clean(opts.MountPath),
		}
		c.VolumeMounts = append(c.VolumeMounts, *volumeMount)
	}
	return nil
}

func (v *VolumeOptions) getVolumeName(spec *kapi.PodSpec, singleResource bool) (string, error) {
	opts := v.AddOpts
	if opts.Overwrite {
		// Multiple resources can have same mount-path for different volumes,
		// so restrict it for single resource to uniquely find the volume
		if !singleResource {
			return "", fmt.Errorf("you must specify --name for the volume name when dealing with multiple resources")
		}
		if len(opts.MountPath) > 0 {
			containers, _ := selectContainers(spec.Containers, v.Containers)
			var name string
			matchCount := 0
			for _, c := range containers {
				for _, m := range c.VolumeMounts {
					if path.Clean(m.MountPath) == path.Clean(opts.MountPath) {
						name = m.Name
						matchCount += 1
						break
					}
				}
			}

			switch matchCount {
			case 0:
				return "", fmt.Errorf("unable to find the volume for mount-path: %s", opts.MountPath)
			case 1:
				return name, nil
			default:
				return "", fmt.Errorf("found multiple volumes with same mount-path: %s", opts.MountPath)
			}
		} else {
			return "", fmt.Errorf("ambiguous --overwrite, specify --name or --mount-path")
		}
	} else { // Generate volume name
		name := names.SimpleNameGenerator.GenerateName(volumePrefix)
		if len(v.Output) == 0 {
			fmt.Fprintf(v.Err, "info: Generated volume name: %s\n", name)
		}
		return name, nil
	}
}

func (v *VolumeOptions) addVolumeToSpec(spec *kapi.PodSpec, info *resource.Info, singleResource bool) error {
	opts := v.AddOpts
	if len(v.Name) == 0 {
		var err error
		v.Name, err = v.getVolumeName(spec, singleResource)
		if err != nil {
			return err
		}
	}
	newVolume := &kapi.Volume{
		Name: v.Name,
	}
	setSource := true
	vNameFound := false
	for i, vol := range spec.Volumes {
		if v.Name == vol.Name {
			vNameFound = true
			if !opts.Overwrite {
				return fmt.Errorf("volume '%s' already exists. Use --overwrite to replace", v.Name)
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
	if !vNameFound && opts.Overwrite && len(v.Output) == 0 {
		fmt.Fprintf(v.Err, "warning: volume %q did not previously exist and was not overriden. A new volume with this name has been created instead.", v.Name)
	}

	if setSource {
		err := v.setVolumeSource(newVolume)
		if err != nil {
			return err
		}
	}
	spec.Volumes = append(spec.Volumes, *newVolume)

	if len(opts.MountPath) > 0 {
		err := v.setVolumeMount(spec, info)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VolumeOptions) removeSpecificVolume(spec *kapi.PodSpec, containers, skippedContainers []*kapi.Container) error {
	for _, c := range containers {
		for i, m := range c.VolumeMounts {
			if v.Name == m.Name {
				c.VolumeMounts = append(c.VolumeMounts[:i], c.VolumeMounts[i+1:]...)
				break
			}
		}
	}

	// Remove volume if no container is using it
	found := false
	for _, c := range skippedContainers {
		for _, m := range c.VolumeMounts {
			if v.Name == m.Name {
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
			if v.Name == vol.Name {
				spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
				foundVolume = true
				break
			}
		}
		if !foundVolume {
			return fmt.Errorf("volume '%s' not found", v.Name)
		}
	}
	return nil
}

func (v *VolumeOptions) removeVolumeFromSpec(spec *kapi.PodSpec, info *resource.Info) error {
	containers, skippedContainers := selectContainers(spec.Containers, v.Containers)
	if len(containers) == 0 && v.Containers != "*" {
		fmt.Fprintf(v.Err, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, v.Containers)
		return nil
	}

	if len(v.Name) == 0 {
		for _, c := range containers {
			c.VolumeMounts = []kapi.VolumeMount{}
		}
		spec.Volumes = []kapi.Volume{}
	} else {
		err := v.removeSpecificVolume(spec, containers, skippedContainers)
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
	default:
		return "unknown"
	}
}

func (v *VolumeOptions) listVolumeForSpec(spec *kapi.PodSpec, info *resource.Info) error {
	containers, _ := selectContainers(spec.Containers, v.Containers)
	if len(containers) == 0 && v.Containers != "*" {
		fmt.Fprintf(v.Err, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, v.Containers)
		return nil
	}

	fmt.Fprintf(v.Out, "%s/%s\n", info.Mapping.Resource, info.Name)
	checkName := (len(v.Name) > 0)
	found := false
	for _, vol := range spec.Volumes {
		if checkName && v.Name != vol.Name {
			continue
		}
		found = true

		refInfo := ""
		if vol.VolumeSource.PersistentVolumeClaim != nil {
			claimName := vol.VolumeSource.PersistentVolumeClaim.ClaimName
			claim, err := v.Client.PersistentVolumeClaims(info.Namespace).Get(claimName, metav1.GetOptions{})
			switch {
			case err == nil:
				refInfo = fmt.Sprintf("(%s)", describePersistentVolumeClaim(claim))
			case apierrs.IsNotFound(err):
				refInfo = "(does not exist)"
			default:
				fmt.Fprintf(v.Err, "error: unable to retrieve persistent volume claim %s referenced in %s/%s: %v", claimName, info.Mapping.Resource, info.Name, err)
			}
		}
		if len(refInfo) > 0 {
			refInfo = " " + refInfo
		}

		fmt.Fprintf(v.Out, "  %s%s as %s\n", describeVolumeSource(&vol.VolumeSource), refInfo, vol.Name)
		for _, c := range containers {
			for _, m := range c.VolumeMounts {
				if vol.Name != m.Name {
					continue
				}
				if len(spec.Containers) == 1 {
					fmt.Fprintf(v.Out, "    mounted at %s\n", m.MountPath)
				} else {
					fmt.Fprintf(v.Out, "    mounted at %s in container %s\n", m.MountPath, c.Name)
				}
			}
		}
	}
	if checkName && !found {
		return fmt.Errorf("volume %q not found", v.Name)
	}

	return nil
}
