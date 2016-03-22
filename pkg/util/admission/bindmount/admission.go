package bindmount

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/util/admission/bindmount/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func init() {
	admission.RegisterPlugin("BinaryBindMount", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return NewBinaryBindMount(pluginConfig), nil
	})
}

func readConfig(reader io.Reader) (*api.BinaryBindMountConfig, error) {
	obj, err := configlatest.ReadYAML(reader)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*api.BinaryBindMountConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object %#v", obj)
	}
	return config, nil
}

// NewBinaryBindMount creates a new BinaryBindMount admission control plugin using
// the given configuration
func NewBinaryBindMount(config *api.BinaryBindMountConfig) admission.Interface {
	var mountsByImage map[string]*api.ImageBindMountSpec
	if config != nil {
		mountsByImage = map[string]*api.ImageBindMountSpec{}
		for i := range config.Images {
			mountsByImage[config.Images[i].Image] = &config.Images[i]
		}
	}
	return &binaryBindMount{
		Handler:       admission.NewHandler(admission.Create),
		mountsByImage: mountsByImage,
	}
}

type binaryBindMount struct {
	*admission.Handler
	mountsByImage map[string]*api.ImageBindMountSpec
}

// Admit will add volume mounts for pods including containers that use the
// images specified in the configuration
func (b *binaryBindMount) Admit(attributes admission.Attributes) error {
	switch {
	case b.mountsByImage == nil,
		attributes.GetResource() != kapi.Resource("pods"),
		len(attributes.GetSubresource()) > 0:
		return nil
	}
	pod, ok := attributes.GetObject().(*kapi.Pod)
	if !ok {
		return admission.NewForbidden(attributes, fmt.Errorf("unexpected object: %#v", attributes.GetObject()))
	}
	namer := mountNamer(0)
	for i := range pod.Spec.Containers {
		if mounts, hasImage := b.mountsByImage[pod.Spec.Containers[i].Image]; hasImage {
			setupMounts(mounts, &pod.Spec.Containers[i], pod, &namer)
		}
	}

	return nil
}

type mountNamer int

func (n *mountNamer) Increment() {
	(*n)++
}

func (n *mountNamer) Volume() string {
	return fmt.Sprintf("bindmount-volume-%d", *n)
}

func setupMounts(spec *api.ImageBindMountSpec, container *kapi.Container, pod *kapi.Pod, namer *mountNamer) {
	for _, mount := range spec.Mounts {
		namer.Increment()
		volume := kapi.Volume{
			Name: namer.Volume(),
			VolumeSource: kapi.VolumeSource{
				HostPath: &kapi.HostPathVolumeSource{
					Path: mount.Source,
				},
			},
		}

		volumeMount := kapi.VolumeMount{
			Name:      namer.Volume(),
			MountPath: mount.Destination,
		}

		pod.Spec.Volumes = append(pod.Spec.Volumes,
			volume)
		container.VolumeMounts =
			append(container.VolumeMounts,
				volumeMount)
	}
}
