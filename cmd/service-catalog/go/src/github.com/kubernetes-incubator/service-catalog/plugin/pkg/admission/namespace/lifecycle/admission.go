/*
Copyright 2017 The Kubernetes Authors.

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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/pkg/api/v1"

	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
)

const (
	// PluginName is name of admission plug-in
	PluginName = "KubernetesNamespaceLifecycle"
	// how long to wait for a missing namespace before re-checking the cache (and then doing a live lookup)
	// this accomplishes two things:
	// 1. It allows a watch-fed cache time to observe a namespace creation event
	// 2. It allows time for a namespace creation to distribute to members of a storage cluster,
	//    so the live lookup has a better chance of succeeding even if it isn't performed against the leader.
	missingNamespaceWait = 50 * time.Millisecond
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) {
		return NewLifecycle()
	})
}

// lifecycle is an implementation of admission.Interface.
// It enforces life-cycle constraints around a Namespace depending on its Phase
type lifecycle struct {
	*admission.Handler
	client          kubeclientset.Interface
	namespaceLister corev1.NamespaceLister
}

type forceLiveLookupEntry struct {
	expiry time.Time
}

var _ = scadmission.WantsKubeInformerFactory(&lifecycle{})
var _ = scadmission.WantsKubeClientSet(&lifecycle{})

func (l *lifecycle) Admit(a admission.Attributes) error {
	// we need to wait for our caches to warm
	if !l.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	var (
		exists bool
		err    error
	)

	namespace, err := l.namespaceLister.Get(a.GetNamespace())
	if err != nil {
		if !errors.IsNotFound(err) {
			return errors.NewInternalError(err)
		}
	} else {
		exists = true
	}

	// we only care about namespaced resources
	if len(a.GetNamespace()) == 0 {
		return nil
	}

	if !exists && a.GetOperation() == admission.Create {
		// give the cache time to observe the namespace before rejecting a create.
		// this helps when creating a namespace and immediately creating objects within it.
		time.Sleep(missingNamespaceWait)
		namespace, err = l.namespaceLister.Get(a.GetNamespace())
		switch {
		case errors.IsNotFound(err):
			// no-op
		case err != nil:
			return errors.NewInternalError(err)
		default:
			exists = true
		}
		if exists {
			glog.V(4).Infof("found %s in cache after waiting", a.GetNamespace())
		}
	}

	// refuse to operate on non-existent namespaces
	if !exists {
		// as a last resort, make a call directly to storage
		namespace, err = l.client.Core().Namespaces().Get(a.GetNamespace(), metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			return err
		case err != nil:
			return errors.NewInternalError(err)
		}
		glog.V(4).Infof("found %s via storage lookup", a.GetNamespace())
	}

	// ensure that we're not trying to create objects in terminating namespaces
	if a.GetOperation() == admission.Create {
		if namespace.Status.Phase != v1.NamespaceTerminating {
			return nil
		}

		return admission.NewForbidden(a, fmt.Errorf("unable to create new content in namespace %s because it is being terminated", a.GetNamespace()))
	}

	return nil
}

// NewLifecycle creates a new namespace lifecycle admission control handler
func NewLifecycle() (admission.Interface, error) {
	return &lifecycle{
		Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
	}, nil
}

func (l *lifecycle) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	namespaceInformer := f.Core().V1().Namespaces()
	l.namespaceLister = namespaceInformer.Lister()
	l.SetReadyFunc(namespaceInformer.Informer().HasSynced)
}

func (l *lifecycle) SetKubeClientSet(client kubeclientset.Interface) {
	l.client = client
}

func (l *lifecycle) Validate() error {
	if l.namespaceLister == nil {
		return fmt.Errorf("missing namespaceLister")
	}
	if l.client == nil {
		return fmt.Errorf("missing client")
	}
	return nil
}
