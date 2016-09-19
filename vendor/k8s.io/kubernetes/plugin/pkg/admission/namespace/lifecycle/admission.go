/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
)

const PluginName = "NamespaceLifecycle"

// how long to wait for a missing namespace before re-checking the cache (and then doing a live lookup)
// this accomplishes two things:
// 1. It allows a watch-fed cache time to observe a namespace creation event
// 2. It allows time for a namespace creation to distribute to members of a storage cluster,
//    so the live lookup has a better chance of succeeding even if it isn't performed against the leader.
const missingNamespaceWait = 50 * time.Millisecond

func init() {
	admission.RegisterPlugin(PluginName, func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewLifecycle(client, sets.NewString(api.NamespaceDefault, api.NamespaceSystem)), nil
	})
}

// lifecycle is an implementation of admission.Interface.
// It enforces life-cycle constraints around a Namespace depending on its Phase
type lifecycle struct {
	*admission.Handler
	client             clientset.Interface
	store              cache.Store
	immortalNamespaces sets.String
}

func (l *lifecycle) Admit(a admission.Attributes) (err error) {
	// prevent deletion of immortal namespaces
	if a.GetOperation() == admission.Delete && a.GetKind().GroupKind() == api.Kind("Namespace") && l.immortalNamespaces.Has(a.GetName()) {
		return errors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), fmt.Errorf("this namespace may not be deleted"))
	}

	// if we're here, then we've already passed authentication, so we're allowed to do what we're trying to do
	// if we're here, then the API server has found a route, which means that if we have a non-empty namespace
	// its a namespaced resource.
	if len(a.GetNamespace()) == 0 || a.GetKind().GroupKind() == api.Kind("Namespace") {
		// if a namespace is deleted, we want to prevent all further creates into it
		// while it is undergoing termination.  to reduce incidences where the cache
		// is slow to update, we forcefully remove the namespace from our local cache.
		// this will cause a live lookup of the namespace to get its latest state even
		// before the watch notification is received.
		if a.GetOperation() == admission.Delete {
			l.store.Delete(&api.Namespace{
				ObjectMeta: api.ObjectMeta{
					Name: a.GetName(),
				},
			})
		}
		return nil
	}

	// always allow access review checks.  Returning status about the namespace would be leaking information
	if isAccessReview(a) {
		return nil
	}

	namespaceObj, exists, err := l.store.Get(&api.Namespace{ObjectMeta: api.ObjectMeta{Name: a.GetNamespace()}})
	if err != nil {
		return errors.NewInternalError(err)
	}

	if !exists && a.GetOperation() == admission.Create {
		// give the cache time to observe the namespace before rejecting a create.
		// this helps when creating a namespace and immediately creating objects within it.
		time.Sleep(missingNamespaceWait)
		namespaceObj, exists, err = l.store.Get(&api.Namespace{ObjectMeta: api.ObjectMeta{Name: a.GetNamespace()}})
		if err != nil {
			return errors.NewInternalError(err)
		}
		if exists {
			glog.V(4).Infof("found %s in cache after waiting", a.GetNamespace())
		}
	}

	// refuse to operate on non-existent namespaces
	if !exists {
		// as a last resort, make a call directly to storage
		// this also benefits from the Sleep() above allowing for propagation in HA storage cases
		namespaceObj, err = l.client.Core().Namespaces().Get(a.GetNamespace())
		if err != nil {
			if errors.IsNotFound(err) {
				return err
			}
			return errors.NewInternalError(err)
		}
		glog.V(4).Infof("found %s via storage lookup", a.GetNamespace())
	}

	// ensure that we're not trying to create objects in terminating namespaces
	if a.GetOperation() == admission.Create {
		namespace := namespaceObj.(*api.Namespace)
		if namespace.Status.Phase != api.NamespaceTerminating {
			return nil
		}

		// TODO: This should probably not be a 403
		return admission.NewForbidden(a, fmt.Errorf("Unable to create new content in namespace %s because it is being terminated.", a.GetNamespace()))
	}

	return nil
}

// NewLifecycle creates a new namespace lifecycle admission control handler
func NewLifecycle(c clientset.Interface, immortalNamespaces sets.String) admission.Interface {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return c.Core().Namespaces().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return c.Core().Namespaces().Watch(options)
			},
		},
		&api.Namespace{},
		store,
		5*time.Minute,
	)
	reflector.Run()
	return &lifecycle{
		Handler:            admission.NewHandler(admission.Create, admission.Update, admission.Delete),
		client:             c,
		store:              store,
		immortalNamespaces: immortalNamespaces,
	}
}

// TODO move this upstream once they have namespaced access review checks
var accessReviewResources = map[unversioned.GroupResource]bool{
	unversioned.GroupResource{Group: "", Resource: "subjectaccessreviews"}:       true,
	unversioned.GroupResource{Group: "", Resource: "localsubjectaccessreviews"}:  true,
	unversioned.GroupResource{Group: "", Resource: "resourceaccessreviews"}:      true,
	unversioned.GroupResource{Group: "", Resource: "localresourceaccessreviews"}: true,
	unversioned.GroupResource{Group: "", Resource: "selfsubjectrulesreviews"}:    true,
}

func isAccessReview(a admission.Attributes) bool {
	return accessReviewResources[a.GetResource().GroupResource()]
}
