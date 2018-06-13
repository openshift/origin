/*
Copyright 2016 The Kubernetes Authors.

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

package v1

import (
	v1 "github.com/openshift/api/image/v1"
)

// The ImageStreamExpansion interface allows manually adding extra methods to the ImageStream interface.
type ImageStreamExpansion interface {
	Layers(name string) (*v1.ImageStreamLayers, error)
}

// Bind applies the provided binding to the named pod in the current namespace (binding.Namespace is ignored).
func (c *imageStreams) Layers(name string) (result *v1.ImageStreamLayers, err error) {
	result = &v1.ImageStreamLayers{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("imagestreams").
		Name(name).
		SubResource("layers").
		Do().
		Into(result)
	return
}
