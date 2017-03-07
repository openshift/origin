package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/golang-lru"

	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type CacheAuthorizer struct {
	authorizer kauthorizer.Authorizer

	authorizeCache *lru.Cache

	ttl time.Duration
	now func() time.Time
}

type authorizeCacheRecord struct {
	created time.Time
	allowed bool
	reason  string
	err     error
}

// NewAuthorizer returns an authorizer that caches the results of the given authorizer
func NewAuthorizer(a kauthorizer.Authorizer, ttl time.Duration, cacheSize int) (kauthorizer.Authorizer, error) {
	authorizeCache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return &CacheAuthorizer{
		authorizer:     a,
		authorizeCache: authorizeCache,
		ttl:            ttl,
		now:            time.Now,
	}, nil
}

func (c *CacheAuthorizer) Authorize(a kauthorizer.Attributes) (allowed bool, reason string, err error) {
	key, err := cacheKey(a)
	if err != nil {
		glog.V(5).Infof("could not build cache key for %#v: %v", a, err)
		return c.authorizer.Authorize(a)
	}

	if value, hit := c.authorizeCache.Get(key); hit {
		switch record := value.(type) {
		case *authorizeCacheRecord:
			if record.created.Add(c.ttl).After(c.now()) {
				return record.allowed, record.reason, record.err
			} else {
				glog.V(5).Infof("cache record expired for %s", key)
				c.authorizeCache.Remove(key)
			}
		default:
			utilruntime.HandleError(fmt.Errorf("invalid cache record type for key %s: %#v", key, record))
		}
	}

	allowed, reason, err = c.authorizer.Authorize(a)

	// Don't cache results if there was an error unrelated to authorization
	// TODO: figure out a better way to determine this
	if err == nil || kerrs.IsForbidden(err) {
		c.authorizeCache.Add(key, &authorizeCacheRecord{created: c.now(), allowed: allowed, reason: reason, err: err})
	}

	return allowed, reason, err
}

func cacheKey(a kauthorizer.Attributes) (string, error) {
	keyData := map[string]interface{}{
		"verb":            a.GetVerb(),
		"apiVersion":      a.GetAPIVersion(),
		"apiGroup":        a.GetAPIGroup(),
		"resource":        a.GetResource(),
		"subresource":     a.GetSubresource(),
		"name":            a.GetName(),
		"readOnly":        a.IsReadOnly(),
		"resourceRequest": a.IsResourceRequest(),
		"path":            a.GetPath(),
		"namespace":       a.GetNamespace(),
	}

	if user := a.GetUser(); user != nil {
		keyData["user"] = user.GetName()
		keyData["groups"] = user.GetGroups()
		keyData["scopes"] = user.GetExtra()[authorizationapi.ScopesKey]
	}

	key, err := json.Marshal(keyData)
	return string(key), err
}
