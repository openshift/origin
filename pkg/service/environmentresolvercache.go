package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"k8s.io/kubernetes/pkg/api"
)

// ServiceRetriever is an interface for retrieving services
type ServiceRetriever interface {
	Get(name string) (*api.Service, error)
}

type serviceEntry struct {
	host string
	port string
}

// ResolverCacheFunc is used for resolving names to services
type ResolverCacheFunc func(name string) (*api.Service, error)

// ServiceResolverCache is a cache used for resolving names to services
type ServiceResolverCache struct {
	fill  ResolverCacheFunc
	cache map[string]serviceEntry
	lock  sync.RWMutex
}

// NewServiceResolverCache returns a new ServiceResolverCache
func NewServiceResolverCache(fill ResolverCacheFunc) *ServiceResolverCache {
	return &ServiceResolverCache{
		cache: make(map[string]serviceEntry),
		fill:  fill,
	}
}

func (c *ServiceResolverCache) get(name string) (host, port string, ok bool) {
	// check
	c.lock.RLock()
	entry, found := c.cache[name]
	c.lock.RUnlock()
	if found {
		return entry.host, entry.port, true
	}

	// fill the cache
	c.lock.Lock()
	defer c.lock.Unlock()
	if entry, found := c.cache[name]; found {
		return entry.host, entry.port, true
	}
	service, err := c.fill(name)
	if err != nil {
		return
	}
	if len(service.Spec.Ports) == 0 {
		return
	}
	host, port, ok = service.Spec.ClusterIP, strconv.Itoa(service.Spec.Ports[0].Port), true
	c.cache[name] = serviceEntry{
		host: host,
		port: port,
	}
	return
}

func toServiceName(envName string) string {
	return strings.TrimSpace(strings.ToLower(strings.Replace(envName, "_", "-", -1)))
}

func recognizeVariable(name string) (service string, host bool, ok bool) {
	switch {
	case strings.HasSuffix(name, "_SERVICE_HOST"):
		service = toServiceName(strings.TrimSuffix(name, "_SERVICE_HOST"))
		host = true
	case strings.HasSuffix(name, "_SERVICE_PORT"):
		service = toServiceName(strings.TrimSuffix(name, "_SERVICE_PORT"))
	default:
		return "", false, false
	}
	if len(service) == 0 {
		return "", false, false
	}
	ok = true
	return
}

func (c *ServiceResolverCache) resolve(name string) (string, bool) {
	service, isHost, ok := recognizeVariable(name)
	if !ok {
		return "", false
	}
	host, port, ok := c.get(service)
	if !ok {
		return "", false
	}
	if isHost {
		return host, true
	}
	return port, true
}

// Defer takes a string (with optional variables) and an expansion function and returns
// a function that can be called to get the value. This method will optimize the
// expansion away in the event that no expansion is necessary.
func (c *ServiceResolverCache) Defer(env string) (func() (string, bool), error) {
	hasExpansion := false
	invalid := []string{}
	os.Expand(env, func(name string) string {
		hasExpansion = true
		if _, _, ok := recognizeVariable(name); !ok {
			invalid = append(invalid, name)
		}
		return ""
	})
	if len(invalid) != 0 {
		return nil, fmt.Errorf("invalid variable name(s): %s", strings.Join(invalid, ", "))
	}
	if !hasExpansion {
		return func() (string, bool) { return env, true }, nil
	}

	// only load the value once
	lock := sync.Mutex{}
	loaded := false
	return func() (string, bool) {
		lock.Lock()
		defer lock.Unlock()
		if loaded {
			return env, true
		}
		resolved := true
		expand := os.Expand(env, func(s string) string {
			s, ok := c.resolve(s)
			resolved = resolved && ok
			return s
		})
		if !resolved {
			return "", false
		}
		loaded = true
		env = expand
		return env, true
	}, nil
}
