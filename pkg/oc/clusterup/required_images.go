package clusterup

import (
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	dockerutil "github.com/openshift/origin/pkg/oc/clusterup/docker/util"
)

// OpenShiftImages specifies a list of images cluster up require to pull in order to bootstrap a cluster.
var OpenShiftImages = Images{
	// OpenShift Images
	{Name: "cluster-kube-apiserver-operator"},
	{Name: "cluster-kube-controller-manager-operator"},
	{Name: "control-plane"},
	{Name: "cli"},
	{Name: "hyperkube"},
	{Name: "hypershift"},
	{Name: "node"},
	{Name: "pod"},

	// External images
	{Name: "bootkube", PullSpec: "quay.io/coreos/bootkube:v0.13.0"},
	{Name: "etcd", PullSpec: "quay.io/coreos/etcd:v3.2.24"},
}

type Image struct {
	Name string

	// PullSpec if specified is used instead of expanding the name via template. Used for non-openshift images.
	PullSpec string
}

type PullSpec string

func (i *Image) ToPullSpec(tpl variable.ImageTemplate) PullSpec {
	if len(i.PullSpec) > 0 {
		return PullSpec(i.PullSpec)
	}
	return PullSpec(tpl.ExpandOrDie(i.Name))
}

func (s PullSpec) Pull(puller *dockerutil.Helper, forcePull bool) error {
	return puller.CheckAndPullImage(string(s), forcePull, os.Stdout)
}

func (s PullSpec) String() string {
	return string(s)
}

type Images []Image

func (i Images) EnsurePulled(puller *dockerutil.Helper, tpl variable.ImageTemplate, forcePull bool) error {
	errors := []error{}
	for _, image := range i {
		if err := image.ToPullSpec(tpl).Pull(puller, forcePull); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) == 0 {
		return nil
	}
	msgs := []string{}
	for _, err := range errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("some images failed to pull: %s", strings.Join(msgs, ","))
}

func (i Images) Get(name string) *Image {
	for _, image := range i {
		if image.Name == name {
			return &image
		}
	}
	return nil
}
