/*
Copyright 2014 The Kubernetes Authors.

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

// Package testapi provides a helper for retrieving the KUBE_TEST_API environment variable.
//
// TODO(lavalamp): this package is a huge disaster at the moment. I intend to
// refactor. All code currently using this package should change:
// 1. Declare your own api.Registry.APIGroupRegistrationManager in your own test code.
// 2. Import the relevant install packages.
// 3. Register the types you need, from the announced.APIGroupAnnouncementManager.
package testapi

import (
	"fmt"
	"mime"
	"os"
	"reflect"
	"strings"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/recognizer"
)

var (
	// Groups exported on purpose
	Groups = make(map[string]TestGroup)
	// ServiceCatalog exported on purpose
	ServiceCatalog TestGroup

	serializer        runtime.SerializerInfo
	storageSerializer runtime.SerializerInfo
)

// TestGroup exported on purpose
type TestGroup struct {
	externalGroupVersion schema.GroupVersion
	internalGroupVersion schema.GroupVersion
	internalTypes        map[string]reflect.Type
	externalTypes        map[string]reflect.Type
}

func init() {
	if apiMediaType := os.Getenv("KUBE_TEST_API_TYPE"); len(apiMediaType) > 0 {
		var ok bool
		mediaType, _, err := mime.ParseMediaType(apiMediaType)
		if err != nil {
			panic(err)
		}
		serializer, ok = runtime.SerializerInfoForMediaType(api.Codecs.SupportedMediaTypes(), mediaType)
		if !ok {
			panic(fmt.Sprintf("no serializer for %s", apiMediaType))
		}
	}

	if storageMediaType := StorageMediaType(); len(storageMediaType) > 0 {
		var ok bool
		mediaType, _, err := mime.ParseMediaType(storageMediaType)
		if err != nil {
			panic(err)
		}
		storageSerializer, ok = runtime.SerializerInfoForMediaType(api.Codecs.SupportedMediaTypes(), mediaType)
		if !ok {
			panic(fmt.Sprintf("no serializer for %s", storageMediaType))
		}
	}

	kubeTestAPI := os.Getenv("KUBE_TEST_API")
	if len(kubeTestAPI) != 0 {
		// priority is "first in list preferred", so this has to run in reverse order
		testGroupVersions := strings.Split(kubeTestAPI, ",")
		for i := len(testGroupVersions) - 1; i >= 0; i-- {
			gvString := testGroupVersions[i]
			groupVersion, err := schema.ParseGroupVersion(gvString)
			if err != nil {
				panic(fmt.Sprintf("Error parsing groupversion %v: %v", gvString, err))
			}

			internalGroupVersion := schema.GroupVersion{Group: groupVersion.Group, Version: runtime.APIVersionInternal}
			Groups[groupVersion.Group] = TestGroup{
				externalGroupVersion: groupVersion,
				internalGroupVersion: internalGroupVersion,
				internalTypes:        api.Scheme.KnownTypes(internalGroupVersion),
				externalTypes:        api.Scheme.KnownTypes(groupVersion),
			}
		}
	}

	if _, ok := Groups[servicecatalog.GroupName]; !ok {
		externalGroupVersion := schema.GroupVersion{Group: servicecatalog.GroupName, Version: api.Registry.GroupOrDie(servicecatalog.GroupName).GroupVersion.Version}
		Groups[servicecatalog.GroupName] = TestGroup{
			externalGroupVersion: externalGroupVersion,
			internalGroupVersion: servicecatalog.SchemeGroupVersion,
			internalTypes:        api.Scheme.KnownTypes(servicecatalog.SchemeGroupVersion),
			externalTypes:        api.Scheme.KnownTypes(externalGroupVersion),
		}
	}

	ServiceCatalog = Groups[servicecatalog.GroupName]
}

// ContentConfig returns group, version, and codec
func (g TestGroup) ContentConfig() (string, *schema.GroupVersion, runtime.Codec) {
	return "application/json", g.GroupVersion(), g.Codec()
}

// GroupVersion returns the group,version
func (g TestGroup) GroupVersion() *schema.GroupVersion {
	copyOfGroupVersion := g.externalGroupVersion
	return &copyOfGroupVersion
}

// InternalGroupVersion returns the group,version used to identify the internal
// types for this API
func (g TestGroup) InternalGroupVersion() schema.GroupVersion {
	return g.internalGroupVersion
}

// InternalTypes returns a map of internal API types' kind names to their Go types.
func (g TestGroup) InternalTypes() map[string]reflect.Type {
	return g.internalTypes
}

// ExternalTypes returns a map of external API types' kind names to their Go types.
func (g TestGroup) ExternalTypes() map[string]reflect.Type {
	return g.externalTypes
}

// Codec returns the codec for the API version to test against, as set by the
// KUBE_TEST_API_TYPE env var.
func (g TestGroup) Codec() runtime.Codec {
	if serializer.Serializer == nil {
		return api.Codecs.LegacyCodec(g.externalGroupVersion)
	}
	return api.Codecs.CodecForVersions(serializer.Serializer, api.Codecs.UniversalDeserializer(), schema.GroupVersions{g.externalGroupVersion}, nil)
}

// NegotiatedSerializer returns the negotiated serializer for the server.
func (g TestGroup) NegotiatedSerializer() runtime.NegotiatedSerializer {
	return api.Codecs
}

// StorageMediaType returns the value of the storage type environment variable
func StorageMediaType() string {
	return os.Getenv("KUBE_TEST_API_STORAGE_TYPE")
}

// StorageCodec returns the codec for the API version to store in etcd, as set by the
// KUBE_TEST_API_STORAGE_TYPE env var.
func (g TestGroup) StorageCodec() runtime.Codec {
	s := storageSerializer.Serializer

	if s == nil {
		return api.Codecs.LegacyCodec(g.externalGroupVersion)
	}

	// etcd2 only supports string data - we must wrap any result before returning
	// TODO: remove for etcd3 / make parameterizable
	if !storageSerializer.EncodesAsText {
		s = runtime.NewBase64Serializer(s, s)
	}
	ds := recognizer.NewDecoder(s, api.Codecs.UniversalDeserializer())

	return api.Codecs.CodecForVersions(s, ds, schema.GroupVersions{g.externalGroupVersion}, nil)
}

// Converter returns the api.Scheme for the API version to test against, as set by the
// KUBE_TEST_API env var.
func (g TestGroup) Converter() runtime.ObjectConvertor {
	interfaces, err := api.Registry.GroupOrDie(g.externalGroupVersion.Group).InterfacesFor(g.externalGroupVersion)
	if err != nil {
		panic(err)
	}
	return interfaces.ObjectConvertor
}

// MetadataAccessor returns the MetadataAccessor for the API version to test against,
// as set by the KUBE_TEST_API env var.
func (g TestGroup) MetadataAccessor() meta.MetadataAccessor {
	interfaces, err := api.Registry.GroupOrDie(g.externalGroupVersion.Group).InterfacesFor(g.externalGroupVersion)
	if err != nil {
		panic(err)
	}
	return interfaces.MetadataAccessor
}

// SelfLink returns a self link that will appear to be for the version Version().
// 'resource' should be the resource path, e.g. "pods" for the Pod type. 'name' should be
// empty for lists.
func (g TestGroup) SelfLink(resource, name string) string {
	if g.externalGroupVersion.Group == servicecatalog.GroupName {
		if name == "" {
			return fmt.Sprintf("/api/%s/%s", g.externalGroupVersion.Version, resource)
		}
		return fmt.Sprintf("/api/%s/%s/%s", g.externalGroupVersion.Version, resource, name)
	}
	// TODO: will need a /apis prefix once we have proper multi-group
	// support
	if name == "" {
		return fmt.Sprintf("/apis/%s/%s/%s", g.externalGroupVersion.Group, g.externalGroupVersion.Version, resource)
	}
	return fmt.Sprintf("/apis/%s/%s/%s/%s", g.externalGroupVersion.Group, g.externalGroupVersion.Version, resource, name)
}

// ResourcePathWithPrefix returns the appropriate path for the given prefix (watch, proxy, redirect, etc), resource, namespace and name.
// For ex, this is of the form:
// /api/v1/watch/namespaces/foo/pods/pod0 for v1.
func (g TestGroup) ResourcePathWithPrefix(prefix, resource, namespace, name string) string {
	var path string
	if g.externalGroupVersion.Group == servicecatalog.GroupName {
		path = "/api/" + g.externalGroupVersion.Version
	} else {
		// TODO: switch back once we have proper multiple group support
		// path = "/apis/" + g.Group + "/" + Version(group...)
		path = "/apis/" + g.externalGroupVersion.Group + "/" + g.externalGroupVersion.Version
	}

	if prefix != "" {
		path = path + "/" + prefix
	}
	if namespace != "" {
		path = path + "/namespaces/" + namespace
	}
	// Resource names are lower case.
	resource = strings.ToLower(resource)
	if resource != "" {
		path = path + "/" + resource
	}
	if name != "" {
		path = path + "/" + name
	}
	return path
}

// ResourcePath returns the appropriate path for the given resource, namespace and name.
// For example, this is of the form:
// /api/v1/namespaces/foo/pods/pod0 for v1.
func (g TestGroup) ResourcePath(resource, namespace, name string) string {
	return g.ResourcePathWithPrefix("", resource, namespace, name)
}

// RESTMapper returns the rest mapper interface
func (g TestGroup) RESTMapper() meta.RESTMapper {
	return api.Registry.RESTMapper()
}

// ExternalGroupVersions returns all external group versions allowed for the server.
func ExternalGroupVersions() schema.GroupVersions {
	versions := []schema.GroupVersion{}
	for _, g := range Groups {
		gv := g.GroupVersion()
		versions = append(versions, *gv)
	}
	return versions
}

// GetCodecForObject gets codec based on runtime.Object
func GetCodecForObject(obj runtime.Object) (runtime.Codec, error) {
	kinds, _, err := api.Scheme.ObjectKinds(obj)
	if err != nil {
		return nil, fmt.Errorf("unexpected encoding error: %v", err)
	}
	kind := kinds[0]

	for _, group := range Groups {
		if group.GroupVersion().Group != kind.Group {
			continue
		}

		if api.Scheme.Recognizes(kind) {
			return group.Codec(), nil
		}
	}
	// Codec used for unversioned types
	if api.Scheme.Recognizes(kind) {
		serializer, ok := runtime.SerializerInfoForMediaType(api.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
		if !ok {
			return nil, fmt.Errorf("no serializer registered for json")
		}
		return serializer.Serializer, nil
	}
	return nil, fmt.Errorf("unexpected kind: %v", kind)
}

// NewTestGroup returns test group
func NewTestGroup(external, internal schema.GroupVersion, internalTypes map[string]reflect.Type, externalTypes map[string]reflect.Type) TestGroup {
	return TestGroup{external, internal, internalTypes, externalTypes}
}
