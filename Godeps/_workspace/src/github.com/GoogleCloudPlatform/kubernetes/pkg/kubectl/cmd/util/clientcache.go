/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package util

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

// KubernetesClientCache is a cache of Kubernetes clients.
type KubernetesClientCache struct {
	*ClientCache
}

func NewKubernetesClientCache(loader clientcmd.ClientConfig, matchVersion bool, defaultFn ConfigDefaultsFunc) KubernetesClientCache {
	newFn := func(config *client.Config) (interface{}, error) { return client.New(config) }
	return KubernetesClientCache{NewClientCache(loader, matchVersion, defaultFn, newFn)}
}

func (c KubernetesClientCache) ClientForVersion(version string) (*client.Client, error) {
	cli, err := c.ClientCache.ClientForVersion(version)
	if err != nil {
		return nil, err
	}
	return cli.(*client.Client), nil
}

// ClientCache caches previously loaded client objects for reuse, and ensures MatchServerVersion
// is invoked only once
type ClientCache struct {
	loader        clientcmd.ClientConfig
	clients       map[string]interface{}
	defaultConfig *client.Config
	matchVersion  bool
	setDefaultsFn ConfigDefaultsFunc
	newClientFn   NewClientFunc
}

type ConfigDefaultsFunc func(*client.Config)
type NewClientFunc func(*client.Config) (interface{}, error)

func NewClientCache(loader clientcmd.ClientConfig, matchVersion bool, defaultFn ConfigDefaultsFunc, newFn NewClientFunc) *ClientCache {
	return &ClientCache{
		loader:        loader,
		clients:       make(map[string]interface{}),
		matchVersion:  matchVersion,
		setDefaultsFn: defaultFn,
		newClientFn:   newFn,
	}
}

// ClientConfigForVersion returns the correct config for a server
func (c *ClientCache) ClientConfigForVersion(version string) (*client.Config, error) {
	if c.defaultConfig == nil {
		config, err := c.loader.ClientConfig()
		if err != nil {
			return nil, err
		}
		c.defaultConfig = config
		if c.matchVersion {
			if err := client.MatchesServerVersion(config); err != nil {
				return nil, err
			}
		}
	}
	// TODO: have a better config copy method
	config := *c.defaultConfig
	if len(version) != 0 {
		config.Version = version
	}
	if c.setDefaultsFn != nil {
		c.setDefaultsFn(&config)
	}

	return &config, nil
}

// ClientForVersion initializes or reuses a client for the specified version, or returns an
// error if that is not possible. It deals with an opaque interface so that other client
// libraries can be reused.
func (c *ClientCache) ClientForVersion(version string) (interface{}, error) {
	config, err := c.ClientConfigForVersion(version)
	if err != nil {
		return nil, err
	}

	if client, ok := c.clients[config.Version]; ok {
		return client, nil
	}

	client, err := c.newClientFn(config)
	if err != nil {
		return nil, err
	}

	c.clients[config.Version] = client
	return client, nil
}
