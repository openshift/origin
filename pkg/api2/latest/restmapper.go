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

package latest

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/api2"
	"github.com/openshift/origin/pkg/api2/meta"
)

// DefaultRESTMapper exposes mappings between the types defined in
// api.Scheme and a single server. It assumes that all types defined
// in api.Scheme map to a single RESTClient, which is invalid if
// the scheme can work with clients across multiple component plugin
// types.
//
// The resource name of a Kind is defined as the lowercase,
// English-plural version of the Kind string in v1beta3 and onwards,
// and as the camelCase version of the name in v1beta1 and v1beta2.
// When converting from resource to Kind, the singular version of the
// resource name is also accepted for convenience.
//
// TODO: Only accept plural for some operations for increased control?
// (`get pod bar` vs `get pods bar`)
type DefaultRESTMapper struct {
	mapping map[string]api.TypeMeta
	reverse map[api.TypeMeta]string
}

// NewDefaultRESTMapper initializes a mapping between Kind and APIVersion
// to a resource name and back based on the Kubernetes api.Scheme and the
// Kubernetes API conventions.
func NewDefaultRESTMapper() meta.RESTMapper {
	mapping := make(map[string]api.TypeMeta)
	reverse := make(map[api.TypeMeta]string)
	for _, version := range Versions {
		for kind := range api.Scheme.KnownTypes(version) {
			plural, singular := kindToResource(version, kind)
			meta := api.TypeMeta{APIVersion: version, Kind: kind}
			if _, ok := mapping[plural]; !ok {
				mapping[plural] = meta
				mapping[singular] = meta
				if strings.ToLower(plural) != plural {
					mapping[strings.ToLower(plural)] = meta
					mapping[strings.ToLower(singular)] = meta
				}
			}
			reverse[meta] = plural
		}
	}
	// TODO: verify name mappings work correctly when versions differ

	return &DefaultRESTMapper{
		mapping: mapping,
		reverse: reverse,
	}
}

// kindToResource converts Kind to a resource name.
func kindToResource(apiVersion, kind string) (plural, singular string) {
	if apiVersion == "v1beta1" || apiVersion == "v1beta2" {
		// Legacy support for mixed case names
		singular = strings.ToLower(kind[:1]) + kind[1:]
	} else {
		// By convention, resources should match the plural, lowercase
		// version of their name.
		singular = strings.ToLower(kind)
	}
	if !strings.HasSuffix(singular, "s") {
		plural = singular + "s"
	} else {
		plural = singular
	}
	return
}

// VersionAndKindForResource implements meta.RESTMapping
func (d *DefaultRESTMapper) VersionAndKindForResource(resource string) (defaultVersion, kind string, err error) {
	meta, ok := d.mapping[resource]
	if !ok {
		return "", "", fmt.Errorf("no resource %q has been defined", resource)
	}
	return meta.APIVersion, meta.Kind, nil
}

// RESTMapping implements meta.RESTMapping
func (d *DefaultRESTMapper) RESTMapping(version, kind string) (*meta.RESTMapping, error) {
	// Default to a version with this kind
	if len(version) == 0 {
		for _, v := range Versions {
			if _, ok := d.reverse[api.TypeMeta{APIVersion: v, Kind: kind}]; ok {
				version = v
				break
			}
		}
		if len(version) == 0 {
			return nil, fmt.Errorf("no object named %q is registered.", kind)
		}
	}

	// Ensure we have a resource mapping
	resource, ok := d.reverse[api.TypeMeta{APIVersion: version, Kind: kind}]
	if !ok {
		found := []string{}
		for _, v := range Versions {
			if _, ok := d.reverse[api.TypeMeta{APIVersion: v, Kind: kind}]; ok {
				found = append(found, v)
			}
		}
		if len(found) > 0 {
			return nil, fmt.Errorf("object with kind %q exists in versions %q, not %q", kind, strings.Join(found, ", "), version)
		}
		return nil, fmt.Errorf("the provided version %q and kind %q cannot be mapped to a supported object", version, kind)
	}

	interfaces, err := InterfacesFor(version)
	if err != nil {
		return nil, err
	}

	return &meta.RESTMapping{
		Resource:         resource,
		APIVersion:       version,
		Codec:            interfaces.Codec,
		MetadataAccessor: interfaces.MetadataAccessor,
	}, nil
}
