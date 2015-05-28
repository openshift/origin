/*
Copyright 2014 Google Inc. All rights reserved.

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

package admission

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/project/cache"
)

// TODO: modify the upstream plug-in so this can be collapsed
// need ability to specify a RESTMapper on upstream version
func init() {
	admission.RegisterPlugin("OriginNamespaceLifecycle", func(client client.Interface, config io.Reader) (admission.Interface, error) {
		return NewLifecycle()
	})
}

type lifecycle struct {
}

// Admit enforces that a namespace must exist in order to associate content with it.
// Admit enforces that a namespace that is terminating cannot accept new content being associated with it.
func (e *lifecycle) Admit(a admission.Attributes) (err error) {
	if len(a.GetNamespace()) == 0 {
		return nil
	}
	defaultVersion, kind, err := latest.RESTMapper.VersionAndKindForResource(a.GetResource())
	if err != nil {
		glog.V(4).Infof("Ignoring life-cycle enforcement for resource %v; no associated default version and kind could be found.", a.GetResource())
		return nil
	}
	mapping, err := latest.RESTMapper.RESTMapping(kind, defaultVersion)
	if err != nil {
		return err
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

	projects, err := cache.GetProjectCache()
	if err != nil {
		return err
	}
	namespace, err := projects.GetNamespaceObject(a.GetNamespace())
	if err != nil {
		return apierrors.NewForbidden(kind, name, err)
	}

	if a.GetOperation() != "CREATE" {
		return nil
	}

	if namespace.Status.Phase != kapi.NamespaceTerminating {
		return nil
	}

	return apierrors.NewForbidden(kind, name, fmt.Errorf("namespace %s is terminating", a.GetNamespace()))
}

func (e *lifecycle) Handles(operation admission.Operation) bool {
	return true
}

func NewLifecycle() (admission.Interface, error) {
	return &lifecycle{}, nil
}
