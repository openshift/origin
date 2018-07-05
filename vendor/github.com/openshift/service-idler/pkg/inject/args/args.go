/*
Copyright 2018 Red Hat, Inc.

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

package args

import (
	"time"

	"github.com/kubernetes-sigs/kubebuilder/pkg/inject/args"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale"

	"github.com/openshift/service-idler/pkg/client/clientset/versioned"
	"github.com/openshift/service-idler/pkg/client/informers/externalversions"
)

// InjectArgs are the arguments need to initialize controllers
type InjectArgs struct {
	args.InjectArgs

	Clientset   *versioned.Clientset
	Informers   externalversions.SharedInformerFactory
	ScaleClient scale.ScalesGetter
}

// makeScaleClient constructs a new scale client using the given
// rest config, and some sane defaults for rest mappers
func makeScaleClient(config *rest.Config) (scale.ScalesGetter, error) {
	// TODO: we need something like deferred discovery REST mapper that calls invalidate
	// on cache misses.
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	cachedDiscovery := discocache.NewMemCacheClient(discoveryClient)
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(cachedDiscovery, apimeta.InterfacesForUnstructured)
	restMapper.Reset()
	// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
	// so we want to re-fetch every time when we actually ask for it
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)
	scaleClient, err := scale.NewForConfig(config, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return nil, err
	}
	return scaleClient, err
}

// CreateInjectArgs returns new controller args
func CreateInjectArgs(config *rest.Config) InjectArgs {
	cs := versioned.NewForConfigOrDie(config)

	// This is where I'd put my NewForConfigOrDie... IF I HAD ONE!
	scaleClient, err := makeScaleClient(config)
	if err != nil {
		panic(err)
	}
	return InjectArgs{
		InjectArgs:  args.CreateInjectArgs(config),
		Clientset:   cs,
		ScaleClient: scaleClient,
		Informers:   externalversions.NewSharedInformerFactory(cs, 2*time.Minute),
	}
}
