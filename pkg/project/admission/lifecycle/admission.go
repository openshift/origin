package lifecycle

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/project/cache"
	projectutil "github.com/openshift/origin/pkg/project/util"
)

// TODO: modify the upstream plug-in so this can be collapsed
// need ability to specify a RESTMapper on upstream version
func init() {
	admission.RegisterPlugin("OriginNamespaceLifecycle", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewLifecycle(client, recommendedCreatableResources)
	})
}

type lifecycle struct {
	client clientset.Interface
	cache  *cache.ProjectCache

	// creatableResources is a set of resources that can be created even if the namespace is terminating
	creatableResources map[unversioned.GroupResource]bool
}

var recommendedCreatableResources = map[unversioned.GroupResource]bool{
	authorizationapi.Resource("resourceaccessreviews"):      true,
	authorizationapi.Resource("localresourceaccessreviews"): true,
	authorizationapi.Resource("subjectaccessreviews"):       true,
	authorizationapi.Resource("localsubjectaccessreviews"):  true,
	authorizationapi.Resource("selfsubjectrulesreviews"):    true,
	authorizationapi.Resource("subjectrulesreviews"):        true,
}
var _ = oadmission.WantsProjectCache(&lifecycle{})
var _ = oadmission.Validator(&lifecycle{})

// Admit enforces that a namespace must have the openshift finalizer associated with it in order to create origin API objects within it
func (e *lifecycle) Admit(a admission.Attributes) (err error) {
	if len(a.GetNamespace()) == 0 {
		return nil
	}
	// only pay attention to origin resources
	if !latest.OriginKind(a.GetKind()) {
		return nil
	}
	// always allow creatable resources through.  These requests should always be allowed.
	if e.creatableResources[a.GetResource().GroupResource()] {
		return nil
	}

	groupMeta, err := registered.Group(a.GetKind().Group)
	if err != nil {
		return err
	}
	mapping, err := groupMeta.RESTMapper.RESTMapping(a.GetKind().GroupKind())
	if err != nil {
		glog.V(4).Infof("Ignoring life-cycle enforcement for resource %v; no associated default version and kind could be found.", a.GetResource())
		return nil
	}
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		return nil
	}

	if !e.cache.Running() {
		return admission.NewForbidden(a, err)
	}

	namespace, err := e.cache.GetNamespace(a.GetNamespace())
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// in case of concurrency issues, we will retry this logic
	numRetries := 10
	interval := time.Duration(rand.Int63n(90)+int64(10)) * time.Millisecond
	for retry := 1; retry <= numRetries; retry++ {

		// associate this namespace with openshift
		_, err = projectutil.Associate(e.client, namespace)
		if err == nil {
			break
		}

		// we have exhausted all reasonable efforts to retry so give up now
		if retry == numRetries {
			return admission.NewForbidden(a, err)
		}

		// get the latest namespace for the next pass in case of resource version updates
		time.Sleep(interval)

		// it's possible the namespace actually was deleted, so just forbid if this occurs
		namespace, err = e.client.Core().Namespaces().Get(a.GetNamespace())
		if err != nil {
			return admission.NewForbidden(a, err)
		}
	}
	return nil
}

func (e *lifecycle) Handles(operation admission.Operation) bool {
	return operation == admission.Create
}

func (e *lifecycle) SetProjectCache(c *cache.ProjectCache) {
	e.cache = c
}

func (e *lifecycle) Validate() error {
	if e.cache == nil {
		return fmt.Errorf("project lifecycle plugin needs a project cache")
	}
	return nil
}

func NewLifecycle(client clientset.Interface, creatableResources map[unversioned.GroupResource]bool) (admission.Interface, error) {
	return &lifecycle{
		client:             client,
		creatableResources: creatableResources,
	}, nil
}
