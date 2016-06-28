package lifecycle

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api"
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
	creatableResources sets.String
}

var recommendedCreatableResources = sets.NewString("resourceaccessreviews", "localresourceaccessreviews")
var _ = oadmission.WantsProjectCache(&lifecycle{})
var _ = oadmission.Validator(&lifecycle{})

// Admit enforces that a namespace must exist in order to associate content with it.
// Admit enforces that a namespace that is terminating cannot accept new content being associated with it.
func (e *lifecycle) Admit(a admission.Attributes) (err error) {
	if len(a.GetNamespace()) == 0 {
		return nil
	}
	// always allow a SAR request through, the SAR will return information about
	// the ability to take action on the object, no need to verify it here.
	if isSubjectAccessReview(a) {
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

	// we want to allow someone to delete something in case it was phantom created somehow
	if a.GetOperation() == "DELETE" {
		return nil
	}

	name := "Unknown"
	obj := a.GetObject()
	if obj != nil {
		name, _ = meta.NewAccessor().Name(obj)
	}

	if !e.cache.Running() {
		return admission.NewForbidden(a, err)
	}

	namespace, err := e.cache.GetNamespace(a.GetNamespace())
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	if a.GetOperation() != "CREATE" {
		return nil
	}

	if namespace.Status.Phase == kapi.NamespaceTerminating && !e.creatableResources.Has(strings.ToLower(a.GetResource().Resource)) {
		return apierrors.NewForbidden(a.GetResource().GroupResource(), name, fmt.Errorf("Namespace %s is terminating", a.GetNamespace()))
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
	return true
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

func NewLifecycle(client clientset.Interface, creatableResources sets.String) (admission.Interface, error) {
	return &lifecycle{
		client:             client,
		creatableResources: creatableResources,
	}, nil
}

var (
	sar  = api.Kind("SubjectAccessReview")
	lsar = api.Kind("LocalSubjectAccessReview")
)

func isSubjectAccessReview(a admission.Attributes) bool {
	return a.GetKind().GroupKind() == sar || a.GetKind().GroupKind() == lsar
}
