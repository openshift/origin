package imagerepositorymapping

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
)

type REST struct {
	imageStreamMappingRegistry imagestreammapping.Registry
}

func NewREST(r imagestreammapping.Registry) *REST {
	return &REST{r}
}

func (r *REST) New() runtime.Object {
	return &api.ImageRepositoryMapping{}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	var mapping api.ImageStreamMapping
	irm := obj.(*api.ImageRepositoryMapping)
	if err := kapi.Scheme.Convert(irm, &mapping); err != nil {
		return nil, err
	}
	return r.imageStreamMappingRegistry.CreateImageStreamMapping(ctx, &mapping)
}
