package emptydir

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/volume"
)

var _ volume.VolumePlugin = &EmptyDirQuotaPlugin{}
var _ volume.Builder = &emptyDirQuotaBuilder{}

// EmptyDirQuotaPlugin is a simple wrapper for the k8s empty dir plugin builder.
type EmptyDirQuotaPlugin struct {
	// wrapped is the actual k8s emptyDir volume plugin we will pass method calls to.
	Wrapped volume.VolumePlugin

	// The default quota to apply to each node:
	Quota resource.Quantity

	// QuotaApplicator is passed to actual volume builders so they can apply
	// quota for the supported filesystem.
	QuotaApplicator QuotaApplicator
}

func (plugin *EmptyDirQuotaPlugin) NewBuilder(spec *volume.Spec, pod *api.Pod, opts volume.VolumeOptions) (volume.Builder, error) {
	volBuilder, err := plugin.Wrapped.NewBuilder(spec, pod, opts)
	if err != nil {
		return volBuilder, err
	}

	// Because we cannot access several fields on the k8s emptyDir struct, and
	// we do not wish to modify k8s code for this, we have to grab a reference
	// to them ourselves.
	// This logic is the same as k8s.io/kubernetes/pkg/volume/empty_dir:
	medium := api.StorageMediumDefault
	if spec.Volume.EmptyDir != nil { // Support a non-specified source as EmptyDir.
		medium = spec.Volume.EmptyDir.Medium
	}

	// Wrap the builder object with our own to add quota functionality:
	wrapperEmptyDir := &emptyDirQuotaBuilder{
		wrapped:         volBuilder,
		pod:             pod,
		medium:          medium,
		quota:           plugin.Quota,
		quotaApplicator: plugin.QuotaApplicator,
	}
	return wrapperEmptyDir, err
}

func (plugin *EmptyDirQuotaPlugin) Init(host volume.VolumeHost) error {
	return plugin.Wrapped.Init(host)
}

func (plugin *EmptyDirQuotaPlugin) Name() string {
	return plugin.Wrapped.Name()
}

func (plugin *EmptyDirQuotaPlugin) CanSupport(spec *volume.Spec) bool {
	return plugin.Wrapped.CanSupport(spec)
}

func (plugin *EmptyDirQuotaPlugin) NewCleaner(volName string, podUID types.UID) (volume.Cleaner, error) {
	return plugin.Wrapped.NewCleaner(volName, podUID)
}

// emptyDirQuotaBuilder is a wrapper plugin builder for the k8s empty dir builder itself.
// This plugin just extends and adds the functionality to apply a
// quota for the pods FSGroup on an XFS filesystem.
type emptyDirQuotaBuilder struct {
	wrapped         volume.Builder
	pod             *api.Pod
	medium          api.StorageMedium
	quota           resource.Quantity
	quotaApplicator QuotaApplicator
}

// Must implement SetUp as well, otherwise the internal Builder.SetUp calls its
// own SetUpAt method, not the one we need.

func (edq *emptyDirQuotaBuilder) SetUp(fsGroup *int64) error {
	return edq.SetUpAt(edq.GetPath(), fsGroup)
}

func (edq *emptyDirQuotaBuilder) SetUpAt(dir string, fsGroup *int64) error {
	err := edq.wrapped.SetUpAt(dir, fsGroup)
	if err == nil {
		err = edq.quotaApplicator.Apply(dir, edq.medium, edq.pod, fsGroup, edq.quota)
	}
	return err
}

func (edq *emptyDirQuotaBuilder) GetAttributes() volume.Attributes {
	return edq.wrapped.GetAttributes()
}

func (edq *emptyDirQuotaBuilder) GetMetrics() (*volume.Metrics, error) {
	return edq.wrapped.GetMetrics()
}

func (edq *emptyDirQuotaBuilder) GetPath() string {
	return edq.wrapped.GetPath()
}
