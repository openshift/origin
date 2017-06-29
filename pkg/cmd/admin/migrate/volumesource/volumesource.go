package volumesource

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	appsapi "k8s.io/kubernetes/pkg/apis/apps/v1beta1"
	batchapi "k8s.io/kubernetes/pkg/apis/batch/v1"
	batchapi2 "k8s.io/kubernetes/pkg/apis/batch/v2alpha1"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	settingsapi "k8s.io/kubernetes/pkg/apis/settings/v1alpha1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/admin/migrate"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps/v1"

	"github.com/spf13/cobra"
)

var (
	internalMigrateVolumeSourceLong = templates.LongDesc(`
		Confirm that all objects that contain volumeSource.metadata do not have it set

		volumeSource.metadata is deprecated and will no longer be available in the next release.
		Thus all objects that contain a non-nil volumeSource.metadata must be manually converted
		to prevent data loss.

		The following resource types are checked by this command:

		* cronjobs.batch
		* daemonsets.extensions
		* deploymentconfigs.apps.openshift.io
		* deployments.extensions
		* jobs.batch
		* podpresets.settings.k8s.io
		* pods
		* podtemplates
		* replicasets.extensions
		* replicationcontrollers
		* statefulsets.apps

		No resources are mutated.`)

	errVolumeSourceMetadataIsNotNil = errors.New("volumeSource.metadata is not nil")
)

type MigrateVolumeSourceOptions struct {
	migrate.ResourceOptions
}

func NewCmdMigrateVolumeSource(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &MigrateVolumeSourceOptions{
		ResourceOptions: migrate.ResourceOptions{
			In:            in,
			Out:           out,
			ErrOut:        errout,
			AllNamespaces: true,
			Include: []string{
				"cronjobs.batch",
				"daemonsets.extensions",
				"deploymentconfigs.apps.openshift.io",
				"deployments.extensions",
				"jobs.batch",
				"podpresets.settings.k8s.io",
				"pods",
				"podtemplates",
				"replicasets.extensions",
				"replicationcontrollers",
				"statefulsets.apps",
			},
		},
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Confirm that all objects that contain volumeSource.metadata do not have it set",
		Long:  internalMigrateVolumeSourceLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(name, f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	return cmd
}

func (o *MigrateVolumeSourceOptions) Complete(name string, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no positional arguments", name)
	}

	return o.ResourceOptions.Complete(f, c)
}

func (o MigrateVolumeSourceOptions) Validate() error {
	return o.ResourceOptions.Validate()
}

func (o MigrateVolumeSourceOptions) Run() error {
	return o.ResourceOptions.Visitor().Visit(func(info *resource.Info) (migrate.Reporter, error) {
		// get the object as we want the external type without any transformations
		versions := &runtime.VersionedObjects{}
		if err := info.Client.Get().
			Resource(info.Mapping.Resource).
			NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
			Name(info.Name).Do().Into(versions); err != nil {
			return migrate.NotChanged, err
		}
		return checkVolumeSourceMetadataIsNil(versions.First()) // check the object as it is serialized on the wire
	})
}

// checkVolumeSourceMetadataIsNil confirms that all objects that contain volumeSource.metadata do not have it set.
// It returns an error any volumeSource.metadata is not nil.
func checkVolumeSourceMetadataIsNil(obj runtime.Object) (migrate.Reporter, error) {
	var (
		err  error
		spec *kapi.PodTemplateSpec
	)

	switch t := obj.(type) {
	case *kapi.Pod:
		spec = &kapi.PodTemplateSpec{Spec: t.Spec} // fake a PodTemplateSpec using the Pod's Spec
	case *settingsapi.PodPreset:
		spec = &kapi.PodTemplateSpec{Spec: kapi.PodSpec{Volumes: t.Spec.Volumes}} // fake a PodTemplateSpec using the PodPreset's Volumes
	case *kapi.PodTemplate:
		spec = &t.Template
	case *kapi.ReplicationController:
		spec = t.Spec.Template
	case *extensionsapi.DaemonSet:
		spec = &t.Spec.Template
	case *extensionsapi.Deployment:
		spec = &t.Spec.Template
	case *extensionsapi.ReplicaSet:
		spec = &t.Spec.Template
	case *appsapi.StatefulSet:
		spec = &t.Spec.Template
	case *batchapi.Job:
		spec = &t.Spec.Template
	case *deployapi.DeploymentConfig:
		spec = t.Spec.Template
	case *batchapi2.CronJob:
		spec = &t.Spec.JobTemplate.Spec.Template
	default:
		return nil, nil // indicate that we ignored the object
	}

	if specHasVolumeSourceMetadata(spec) {
		err = errVolumeSourceMetadataIsNotNil
	}

	return migrate.NotChanged, err // we only perform read operations
}

func specHasVolumeSourceMetadata(spec *kapi.PodTemplateSpec) bool {
	for _, volume := range spec.Spec.Volumes {
		if volume.VolumeSource.Metadata != nil {
			return true
		}
	}
	return false
}
