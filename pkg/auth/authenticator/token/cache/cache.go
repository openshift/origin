package cache

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/golang-lru"

	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	"github.com/openshift/origin/pkg/auth/authenticator"
)

type CacheAuthenticator struct {
	authenticator authenticator.Token

	cache *lru.Cache

	ttl time.Duration
	now func() time.Time
}

type cacheRecord struct {
	created time.Time
	user    user.Info
	ok      bool
	err     error
}

// NewAuthenticator returns an authenticator that caches the results of the given authenticator
func NewAuthenticator(a authenticator.Token, ttl time.Duration, maxCount int) (authenticator.Token, error) {
	cache, err := lru.New(maxCount)
	if err != nil {
		return nil, err
	}
	return &CacheAuthenticator{
		authenticator: a,
		cache:         cache,
		ttl:           ttl,
		now:           time.Now,
	}, nil
}

func (c *CacheAuthenticator) AuthenticateToken(token string) (user.Info, bool, error) {
	if value, hit := c.cache.Get(token); hit {
		switch record := value.(type) {
		case *cacheRecord:
			if record.created.Add(c.ttl).After(c.now()) {
				glog.V(5).Infof("cache record found: %#v", record)
				return record.user, record.ok, record.err
			} else {
				glog.V(5).Infof("cache record expired: %#v", record)
				c.cache.Remove(token)
			}
		default:
			utilruntime.HandleError(fmt.Errorf("invalid cache record type: %#v", record))
		}
	}

	u, ok, err := c.authenticator.AuthenticateToken(token)

	// Don't cache results if there was an error unrelated to authentication
	// TODO: figure out a better way to determine this
	if err == nil || kerrs.IsUnauthorized(err) {
		c.cache.Add(token, &cacheRecord{created: c.now(), user: u, ok: ok, err: err})
	}

	return u, ok, err
}
