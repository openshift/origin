package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	volumeLong = `Update volumes on a pod

This command can add, update, list or remove volumes from containers in pods or
any object that has a pod template (replication controllers or deployment configurations).
You can specify a single object or multiple, and alter volumes on all containers or
just those that match a wildcard.`

	volumeExample = `  // Add new volume of type 'emptyDir' for deployment config 'registry' and mount under /opt inside the containers
  // The volume name is auto generated
  $ %[1]s volume dc/registry --add --mount-path=/opt

  // Add new volume 'v1' with secret 'magic' for pod 'p1'
  $ %[1]s volume pod/p1 --add --name=v1 -m /etc --type=secret --secret-name=magic

  // Add new volume to pod 'p1' based on gitRepo (or other volume sources not supported by --type)
  $ %[1]s volume pod/p1 --add -m /repo --source=<json-string>

  // Add emptyDir volume 'v1' to a pod definition on disk and update the pod on the server
  $ %[1]s volume -f pod.json --add --name=v1

  // Create a new persistent volume and overwrite existing volume 'v1' for replication controller 'r1'
  $ %[1]s volume rc/r1 --add --name=v1 -t persistentVolumeClaim --claim-name=pvc1 --overwrite

  // Change pod 'p1' mount point to /data for volume v1
  $ %[1]s volume pod p1 --add --name=v1 -m /data --overwrite

  // Remove all volumes for pod 'p1'
  $ %[1]s volume pod/p1 --remove --confirm

  // Remove volume 'v1' from deployment config 'registry'  
  $ %[1]s volume dc/registry --remove --name=v1

  // Unmount volume v1 from container c1 on pod p1 and remove the volume v1 if it is not referenced by any containers on pod p1
  $ %[1]s volume pod/p1 --remove --name=v1 --containers=c1

  // List volumes defined on replication controller 'r1'
  $ %[1]s volume rc r1 --list

  // List volumes defined on all pods
  $ %[1]s volume pods --all --list

  // Output json object with volume info for pod 'p1' but don't alter the object on server
  $ %[1]s volume pod/p1 --add --name=v1 --mount=/opt -o json`
)

type VolumeOptions struct {
	DefaultNamespace       string
	Writer                 io.Writer
	Mapper                 meta.RESTMapper
	Typer                  runtime.ObjectTyper
	RESTClientFactory      func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	UpdatePodSpecForObject func(obj runtime.Object, fn func(*kapi.PodSpec) error) (bool, error)

	// Resource selection
	Selector  string
	All       bool
	Filenames kutil.StringList

	// Operations
	Add    bool
	Remove bool
	List   bool

	// Common optional params
	Name          string
	Containers    string
	Confirm       bool
	Output        string
	OutputVersion string

	// Add op params
	AddOpts *AddVolumeOptions
}

type AddVolumeOptions struct {
	Type       string
	MountPath  string
	Overwrite  bool
	Path       string
	SecretName string
	ClaimName  string
	Source     string

	TypeChanged bool
}

func NewCmdVolume(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	addOpts := &AddVolumeOptions{}
	opts := &VolumeOptions{AddOpts: addOpts}
	cmd := &cobra.Command{
		Use:     "volume RESOURCE/NAME --add|--remove|--list",
		Short:   "Update volume on a resource with a pod template",
		Long:    volumeLong,
		Example: fmt.Sprintf(volumeExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			addOpts.TypeChanged = cmd.Flag("type").Changed

			err := opts.Validate(args)
			if err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			err = opts.Complete(f, cmd, out)
			kcmdutil.CheckErr(err)

			err = opts.RunVolume(args)
			if err == errExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&opts.Selector, "selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&opts.All, "all", false, "select all resources in the namespace of the specified resource types")
	cmd.Flags().VarP(&opts.Filenames, "filename", "f", "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&opts.Add, "add", false, "Add volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&opts.Remove, "remove", false, "Remove volume and/or volume mounts for containers")
	cmd.Flags().BoolVar(&opts.List, "list", false, "List volumes and volume mounts for containers")

	cmd.Flags().StringVar(&opts.Name, "name", "", "Name of the volume. If empty, auto generated for add operation")
	cmd.Flags().StringVarP(&opts.Containers, "containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().BoolVar(&opts.Confirm, "confirm", false, "Confirm that you really want to remove multiple volumes")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Display the changed objects instead of updating them. One of: json|yaml")
	cmd.Flags().String("output-version", "", "Output the changed objects with the given version (default api-version).")

	cmd.Flags().StringVarP(&addOpts.Type, "type", "t", "emptyDir", "Type of the volume source for add operation. Supported options: emptyDir, hostPath, secret, persistentVolumeClaim")
	cmd.Flags().StringVarP(&addOpts.MountPath, "mount-path", "m", "", "Mount path inside the container. Optional param for --add or --remove op")
	cmd.Flags().BoolVar(&addOpts.Overwrite, "overwrite", false, "If true, replace existing volume source and/or volume mount for the given resource")
	cmd.Flags().StringVar(&addOpts.Path, "path", "", "Host path. Must be provided for hostPath volume type")
	cmd.Flags().StringVar(&addOpts.SecretName, "secret-name", "", "Name of the persisted secret. Must be provided for secret volume type")
	cmd.Flags().StringVar(&addOpts.ClaimName, "claim-name", "", "Persistent volume claim name. Must be provided for persistentVolumeClaim volume type")
	cmd.Flags().StringVar(&addOpts.Source, "source", "", "Details of volume source as json string. This can be used if the required volume type is not supported by --type option. (e.g.: '{\"gitRepo\": {\"repository\": <git-url>, \"revision\": <commit-hash>}}')")

	return cmd
}

func (v *VolumeOptions) Validate(args []string) error {
	errList := []error{}
	if len(v.Selector) > 0 {
		if _, err := labels.Parse(v.Selector); err != nil {
			errList = append(errList, errors.New("--selector=<selector> must be a valid label selector"))
		}
		if v.All {
			errList = append(errList, errors.New("either specify --selector or --all but not both"))
		}
	}
	if len(v.Filenames) == 0 && len(args) < 1 {
		errList = append(errList, errors.New("one or more resources must be specified as <resource> <name> or <resource>/<name>"))
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
		errList = append(errList, errors.New("must provide a volume operation. Valid values are --add, --remove and --list"))
	case numOps > 1:
		errList = append(errList, errors.New("you may only specify one operation at a time"))
	}

	if v.List && len(v.Output) > 0 {
		errList = append(errList, errors.New("--list and --output may not be specified together"))
	}

	err := v.AddOpts.Validate(v.Add)
	if err != nil {
		errList = append(errList, err)
	}
	// Removing all volumes for the resource type needs confirmation
	if v.Remove && len(v.Name) == 0 && !v.Confirm {
		errList = append(errList, errors.New("must provide --confirm for removing more than one volume"))
	}

	return kerrors.NewAggregate(errList)
}

func (a *AddVolumeOptions) Validate(isAddOp bool) error {
	errList := []error{}
	if isAddOp {
		if len(a.Type) == 0 && len(a.Source) == 0 {
			errList = append(errList, errors.New("must provide --type or --source for --add operation"))
		} else if a.TypeChanged && len(a.Source) > 0 {
			errList = append(errList, errors.New("either specify --type or --source but not both for --add operation"))
		}

		if len(a.Type) > 0 {
			switch strings.ToLower(a.Type) {
			case "emptydir":
			case "hostpath":
				if len(a.Path) == 0 {
					errList = append(errList, errors.New("must provide --path for --type=hostPath"))
				}
			case "secret":
				if len(a.SecretName) == 0 {
					errList = append(errList, errors.New("must provide --secret-name for --type=secret"))
				}
			case "persistentvolumeclaim":
				if len(a.ClaimName) == 0 {
					errList = append(errList, errors.New("must provide --claim-name for --type=persistentVolumeClaim"))
				}
			default:
				errList = append(errList, errors.New("invalid volume type. Supported types: emptyDir, hostPath, secret, persistentVolumeClaim"))
			}
		} else if len(a.Path) > 0 || len(a.SecretName) > 0 || len(a.ClaimName) > 0 {
			errList = append(errList, errors.New("--path|--secret-name|--claim-name are only valid for --type option"))
		}

		if len(a.Source) > 0 {
			var source map[string]interface{}
			err := json.Unmarshal([]byte(a.Source), &source)
			if err != nil {
				errList = append(errList, err)
			}
			if len(source) > 1 {
				errList = append(errList, errors.New("must provide only one volume for --source"))
			}

			var vs kapi.VolumeSource
			err = json.Unmarshal([]byte(a.Source), &vs)
			if err != nil {
				errList = append(errList, err)
			}
		}
	} else if len(a.Source) > 0 || len(a.Path) > 0 || len(a.SecretName) > 0 || len(a.ClaimName) > 0 || a.Overwrite {
		errList = append(errList, errors.New("--type|--path|--secret-name|--claim-name|--source|--overwrite are only valid for --add operation"))
	}

	return kerrors.NewAggregate(errList)
}

func (v *VolumeOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer) error {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	v.OutputVersion = kcmdutil.OutputVersion(cmd, clientConfig.Version)

	cmdNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	mapper, typer := f.Object()

	v.DefaultNamespace = cmdNamespace
	v.Writer = out
	v.Mapper = mapper
	v.Typer = typer
	v.RESTClientFactory = f.Factory.RESTClient
	v.UpdatePodSpecForObject = f.UpdatePodSpecForObject

	if v.Add && len(v.Name) == 0 {
		v.Name = string(kutil.NewUUID())
		if len(v.Output) == 0 {
			fmt.Fprintf(v.Writer, "Generated volume name: %s\n", v.Name)
		}
	}
	// In case of volume source ignore the default volume type
	if len(v.AddOpts.Source) > 0 {
		v.AddOpts.Type = ""
	}
	return nil
}

func (v *VolumeOptions) RunVolume(args []string) error {
	b := resource.NewBuilder(v.Mapper, v.Typer, resource.ClientMapperFunc(v.RESTClientFactory)).
		ContinueOnError().
		NamespaceParam(v.DefaultNamespace).DefaultNamespace().
		FilenameParam(v.Filenames...).
		SelectorParam(v.Selector).
		ResourceTypeOrNameArgs(v.All, args...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingular(&one).Infos()
	if err != nil {
		return err
	}

	skipped := 0
	for _, info := range infos {
		ok, err := v.UpdatePodSpecForObject(info.Object, func(spec *kapi.PodSpec) error {
			var e error
			switch {
			case v.Add:
				e = v.addVolumeToSpec(spec)
			case v.Remove:
				e = v.removeVolumeFromSpec(spec)
			case v.List:
				e = v.listVolumeForSpec(spec, info)
			}
			return e
		})
		if !ok {
			skipped++
			continue
		}
		if err != nil {
			fmt.Fprintf(v.Writer, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
			continue
		}
	}
	if one && skipped == len(infos) {
		return fmt.Errorf("the %s %s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
	}

	if v.List {
		return nil
	}

	objects, err := resource.AsVersionedObject(infos, false, v.OutputVersion)
	if err != nil {
		return err
	}

	if len(v.Output) != 0 {
		p, _, err := kubectl.GetPrinter(v.Output, "")
		if err != nil {
			return err
		}
		return p.PrintObj(objects, v.Writer)
	}

	failed := false
	for _, info := range infos {
		data, err := info.Mapping.Codec.Encode(info.Object)
		if err != nil {
			fmt.Fprintf(v.Writer, "Error: %v\n", err)
			failed = true
			continue
		}
		obj, err := resource.NewHelper(info.Client, info.Mapping).Update(info.Namespace, info.Name, true, data)
		if err != nil {
			handlePodUpdateError(v.Writer, err, "volume")
			failed = true
			continue
		}
		info.Refresh(obj, true)
		fmt.Fprintf(v.Writer, "%s/%s\n", info.Mapping.Resource, info.Name)
	}
	if failed {
		return errExit
	}
	return nil
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
		kv.Secret = &kapi.SecretVolumeSource{
			SecretName: opts.SecretName,
		}
	case "persistentvolumeclaim":
		kv.PersistentVolumeClaimVolumeSource = &kapi.PersistentVolumeClaimVolumeSource{
			ClaimName: opts.ClaimName,
		}
	default:
		return fmt.Errorf("invalid volume type: %s", opts.Type)
	}
	return nil
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

func (v *VolumeOptions) setVolumeMount(spec *kapi.PodSpec) {
	opts := v.AddOpts
	containers, _ := selectContainers(spec.Containers, v.Containers)
	for _, c := range containers {
		for i, m := range c.VolumeMounts {
			if m.Name == v.Name {
				c.VolumeMounts = append(c.VolumeMounts[:i], c.VolumeMounts[i+1:]...)
				break
			}
		}
		volumeMount := &kapi.VolumeMount{
			Name:      v.Name,
			MountPath: opts.MountPath,
		}
		c.VolumeMounts = append(c.VolumeMounts, *volumeMount)
	}
}

func (v *VolumeOptions) addVolumeToSpec(spec *kapi.PodSpec) error {
	opts := v.AddOpts
	newVolume := &kapi.Volume{
		Name: v.Name,
	}
	setSource := true
	for i, vol := range spec.Volumes {
		if v.Name == vol.Name {
			if !opts.Overwrite {
				return fmt.Errorf("Volume '%s' already exists. Use --overwrite to replace", v.Name)
			}
			if !opts.TypeChanged && len(opts.Source) == 0 {
				newVolume.VolumeSource = vol.VolumeSource
				setSource = false
			}
			spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	if setSource {
		err := v.setVolumeSource(newVolume)
		if err != nil {
			return err
		}
	}
	spec.Volumes = append(spec.Volumes, *newVolume)

	if len(opts.MountPath) > 0 {
		v.setVolumeMount(spec)
	}
	return nil
}

func (v *VolumeOptions) removeVolumeFromSpec(spec *kapi.PodSpec) error {
	containers, skippedContainers := selectContainers(spec.Containers, v.Containers)
	if len(v.Name) == 0 {
		for _, c := range containers {
			c.VolumeMounts = []kapi.VolumeMount{}
		}
		spec.Volumes = []kapi.Volume{}
	} else {
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
			for i, vol := range spec.Volumes {
				if v.Name == vol.Name {
					spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
					break
				}
			}
		}
	}
	return nil
}

func (v *VolumeOptions) listVolumeForSpec(spec *kapi.PodSpec, info *resource.Info) error {
	fmt.Fprintf(v.Writer, "# %s %s, volumes:\n", info.Mapping.Resource, info.Name)
	checkName := (len(v.Name) > 0)
	for _, vol := range spec.Volumes {
		if checkName && v.Name != vol.Name {
			continue
		}
		fmt.Fprintf(v.Writer, "%s\n", vol.Name)
	}

	containers, _ := selectContainers(spec.Containers, v.Containers)
	for _, c := range containers {
		fmt.Fprintf(v.Writer, "\t# container %s, volume mounts:\n", c.Name)
		for _, m := range c.VolumeMounts {
			if checkName && v.Name != m.Name {
				continue
			}
			fmt.Fprintf(v.Writer, "\t%s %s\n", m.Name, m.MountPath)
		}
		fmt.Fprintf(v.Writer, "\n")
	}
	fmt.Fprintf(v.Writer, "\n")

	return nil
}
