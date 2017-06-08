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

package tpr

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/v1"
)

// listResource uses cl to get resources of the given kind from the given namespace, and decodes
// the resources into listObj.
func listResource(
	cl restclient.Interface,
	ns string,
	kind Kind,
	listObj runtime.Object,
	codec runtime.Codec,
) ([]runtime.Object, error) {
	req := cl.Get().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		kind.URLName(),
	)

	var unknown runtime.Unknown
	if err := req.Do().Into(&unknown); err != nil {
		glog.Errorf("doing request (%s)", err)
		return nil, err
	}

	if err := decode(codec, unknown.Raw, listObj); err != nil {
		return nil, err
	}
	objs, err := meta.ExtractList(listObj)
	if err != nil {
		glog.Errorf("extracting list items from the list object (%s)", err)
		return nil, err
	}
	return objs, nil
}

// getAllNamespaces uses cl to get all namespaces
func getAllNamespaces(cl restclient.Interface) (*v1.NamespaceList, error) {
	req := cl.Get().AbsPath("api", "v1", "namespaces")
	var nsList v1.NamespaceList
	if err := req.Do().Into(&nsList); err != nil {
		glog.Errorf("getting all namespaces (%s)", err)
		return nil, err
	}
	return &nsList, nil
}

// stripNamespacesFromList removes the namespaces from each object in the list represented by obj
func stripNamespacesFromList(obj runtime.Object) error {
	return meta.EachListItem(obj, removeNamespace)
}
