package resourceapply

import (
	"crypto/md5"
	"fmt"
	"io"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

type cachedVersionKey struct {
	name      string
	namespace string
	kind      schema.GroupKind
}

// record of resource metadata used to determine if its safe to return early from an ApplyFoo
// resourceHash is an ms5 hash of the required in an ApplyFoo that is computed in case the input changes
// resourceVersion is the received resourceVersion from the apiserver in response to an update that is comparable to the GET
type cachedResource struct {
	resourceHash, resourceVersion string
}

type resourceCache struct {
	cache map[cachedVersionKey]cachedResource
}

type ResourceCache interface {
	UpdateCachedResourceMetadata(required runtime.Object, actual runtime.Object)
	SafeToSkipApply(required runtime.Object, existing runtime.Object) bool
}

func NewResourceCache() *resourceCache {
	return &resourceCache{
		cache: map[cachedVersionKey]cachedResource{},
	}
}

var noCache *resourceCache

func getResourceMetadata(obj runtime.Object) (schema.GroupKind, string, string, string, error) {
	if obj == nil {
		return schema.GroupKind{}, "", "", "", fmt.Errorf("nil object has no metadata")
	}
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return schema.GroupKind{}, "", "", "", err
	}
	if metadata == nil || reflect.ValueOf(metadata).IsNil() {
		return schema.GroupKind{}, "", "", "", fmt.Errorf("object has no metadata")
	}
	resourceHash := hashOfResourceStruct(obj)

	// retrieve kind, sometimes this can be done via the accesor, sometimes not (depends on the type)
	kind := schema.GroupKind{}
	gvk := obj.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) > 0 {
		kind = gvk.GroupKind()
	} else {
		if currKind := getCoreGroupKind(obj); currKind != nil {
			kind = *currKind
		}
	}
	if len(kind.Kind) == 0 {
		return schema.GroupKind{}, "", "", "", fmt.Errorf("unable to determine GroupKind of %T", obj)
	}

	return kind, metadata.GetName(), metadata.GetNamespace(), resourceHash, nil
}

func getResourceVersion(obj runtime.Object) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object has no resourceVersion")
	}
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	if metadata == nil || reflect.ValueOf(metadata).IsNil() {
		return "", fmt.Errorf("object has no metadata")
	}
	rv := metadata.GetResourceVersion()
	if len(rv) == 0 {
		return "", fmt.Errorf("missing resourceVersion")
	}

	return rv, nil
}

func (c *resourceCache) UpdateCachedResourceMetadata(required runtime.Object, actual runtime.Object) {
	if c == nil || c.cache == nil {
		return
	}
	if required == nil || actual == nil {
		return
	}
	kind, name, namespace, resourceHash, err := getResourceMetadata(required)
	if err != nil {
		return
	}
	cacheKey := cachedVersionKey{
		name:      name,
		namespace: namespace,
		kind:      kind,
	}

	resourceVersion, err := getResourceVersion(actual)
	if err != nil {
		klog.V(4).Infof("error reading resourceVersion %s:%s:%s %s", name, kind, namespace, err)
		return
	}

	c.cache[cacheKey] = cachedResource{resourceHash, resourceVersion}
	klog.V(7).Infof("updated resourceVersion of %s:%s:%s %s", name, kind, namespace, resourceVersion)
}

// in the circumstance that an ApplyFoo's 'required' is the same one which was previously
// applied for a given (name, kind, namespace) and the existing resource (if any),
// hasn't been modified since the ApplyFoo last updated that resource, then return true (we don't
// need to reapply the resource). Otherwise return false.
func (c *resourceCache) SafeToSkipApply(required runtime.Object, existing runtime.Object) bool {
	if c == nil || c.cache == nil {
		return false
	}
	if required == nil || existing == nil {
		return false
	}
	kind, name, namespace, resourceHash, err := getResourceMetadata(required)
	if err != nil {
		return false
	}
	cacheKey := cachedVersionKey{
		name:      name,
		namespace: namespace,
		kind:      kind,
	}

	resourceVersion, err := getResourceVersion(existing)
	if err != nil {
		return false
	}

	var versionMatch, hashMatch bool
	if cached, exists := c.cache[cacheKey]; exists {
		versionMatch = cached.resourceVersion == resourceVersion
		hashMatch = cached.resourceHash == resourceHash
		if versionMatch && hashMatch {
			klog.V(4).Infof("found matching resourceVersion & manifest hash")
			return true
		}
	}

	return false
}

// detect changes in a resource by caching a hash of the string representation of the resource
// note: some changes in a resource e.g. nil vs empty, will not be detected this way
func hashOfResourceStruct(o interface{}) string {
	oString := fmt.Sprintf("%v", o)
	h := md5.New()
	io.WriteString(h, oString)
	rval := fmt.Sprintf("%x", h.Sum(nil))
	return rval
}
