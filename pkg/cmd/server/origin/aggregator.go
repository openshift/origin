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

// Package app does all of the work necessary to create a Kubernetes
// APIServer by binding together the API, master and APIServer infrastructure.
// It can be configured and called directly or via the hyperkube framework.
package origin

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"

	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/master/thirdparty"
)

func (c *MasterConfig) createAggregatorConfig(genericConfig genericapiserver.Config) (*aggregatorapiserver.Config, error) {
	// this is a shallow copy so let's twiddle a few things
	// the aggregator doesn't wire these up.  It just delegates them to the kubeapiserver
	genericConfig.EnableSwaggerUI = false
	genericConfig.SwaggerConfig = nil

	// This depends on aggregator types being registered into the kapi.Scheme, which is currently done in Start() to avoid concurrent scheme modification
	// install our types into the scheme so that "normal" RESTOptionsGetters can work for us
	// install.Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)

	serviceResolver := aggregatorapiserver.NewClusterIPServiceResolver(
		c.ClientGoKubeInformers.Core().V1().Services().Lister(),
	)

	var certBytes []byte
	var keyBytes []byte
	var err error
	if len(c.Options.AggregatorConfig.ProxyClientInfo.CertFile) > 0 {
		certBytes, err = ioutil.ReadFile(c.Options.AggregatorConfig.ProxyClientInfo.CertFile)
		if err != nil {
			return nil, err
		}
		keyBytes, err = ioutil.ReadFile(c.Options.AggregatorConfig.ProxyClientInfo.KeyFile)
		if err != nil {
			return nil, err
		}
	}

	aggregatorConfig := &aggregatorapiserver.Config{
		GenericConfig:     &genericConfig,
		CoreKubeInformers: c.ClientGoKubeInformers,
		ProxyClientCert:   certBytes,
		ProxyClientKey:    keyBytes,
		ServiceResolver:   serviceResolver,
	}
	return aggregatorConfig, nil
}

func createAggregatorServer(aggregatorConfig *aggregatorapiserver.Config, delegateAPIServer genericapiserver.DelegationTarget, kubeInformers informers.SharedInformerFactory, apiExtensionInformers apiextensionsinformers.SharedInformerFactory) (*aggregatorapiserver.APIAggregator, error) {
	aggregatorServer, err := aggregatorConfig.Complete().NewWithDelegate(delegateAPIServer)
	if err != nil {
		return nil, err
	}

	// create controllers for auto-registration
	apiRegistrationClient, err := apiregistrationclient.NewForConfig(aggregatorConfig.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	autoRegistrationController := autoregister.NewAutoRegisterController(aggregatorServer.APIRegistrationInformers.Apiregistration().InternalVersion().APIServices(), apiRegistrationClient)
	apiServices := apiServicesToRegister(delegateAPIServer, autoRegistrationController)
	tprRegistrationController := thirdparty.NewAutoRegistrationController(
		kubeInformers.Extensions().InternalVersion().ThirdPartyResources(),
		apiExtensionInformers.Apiextensions().InternalVersion().CustomResourceDefinitions(),
		autoRegistrationController)

	aggregatorServer.GenericAPIServer.AddPostStartHook("kube-apiserver-autoregistration", func(context genericapiserver.PostStartHookContext) error {
		go autoRegistrationController.Run(5, context.StopCh)
		go tprRegistrationController.Run(5, context.StopCh)
		return nil
	})
	aggregatorServer.GenericAPIServer.AddHealthzChecks(healthz.NamedCheck("autoregister-completion", func(r *http.Request) error {
		items, err := aggregatorServer.APIRegistrationInformers.Apiregistration().InternalVersion().APIServices().Lister().List(labels.Everything())
		if err != nil {
			return err
		}

		missing := []apiregistration.APIService{}
		for _, apiService := range apiServices {
			found := false
			for _, item := range items {
				if item.Name != apiService.Name {
					continue
				}
				if apiregistration.IsAPIServiceConditionTrue(item, apiregistration.Available) {
					found = true
					break
				}
			}

			if !found {
				missing = append(missing, *apiService)
			}
		}

		if len(missing) > 0 {
			return fmt.Errorf("missing APIService: %v", missing)
		}
		return nil
	}))

	return aggregatorServer, nil
}

func makeAPIService(gv schema.GroupVersion) *apiregistration.APIService {
	apiServicePriority, ok := apiVersionPriorities[gv]
	if !ok {
		// if we aren't found, then we shouldn't register ourselves because it could result in a CRD group version
		// being permanently stuck in the APIServices list.
		glog.Infof("Skipping APIService creation for %v", gv)
		return nil
	}
	return &apiregistration.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: gv.Version + "." + gv.Group},
		Spec: apiregistration.APIServiceSpec{
			Group:                gv.Group,
			Version:              gv.Version,
			GroupPriorityMinimum: apiServicePriority.group,
			VersionPriority:      apiServicePriority.version,
		},
	}
}

type priority struct {
	group   int32
	version int32
}

// The proper way to resolve this letting the aggregator know the desired group and version-within-group order of the underlying servers
// is to refactor the genericapiserver.DelegationTarget to include a list of priorities based on which APIs were installed.
// This requires the APIGroupInfo struct to evolve and include the concept of priorities and to avoid mistakes, the core storage map there needs to be updated.
// That ripples out every bit as far as you'd expect, so for 1.7 we'll include the list here instead of being built up during storage.
var apiVersionPriorities = map[schema.GroupVersion]priority{
	{Group: "", Version: "v1"}: {group: 18000, version: 1},
	// extensions is above the rest for CLI compatibility, though the level of unqalified resource compatibility we
	// can reasonably expect seems questionable.
	{Group: "extensions", Version: "v1beta1"}: {group: 17900, version: 1},
	// to my knowledge, nothing below here collides
	{Group: "apps", Version: "v1beta1"}:                       {group: 17800, version: 1},
	{Group: "authentication.k8s.io", Version: "v1"}:           {group: 17700, version: 15},
	{Group: "authentication.k8s.io", Version: "v1beta1"}:      {group: 17700, version: 9},
	{Group: "authorization.k8s.io", Version: "v1"}:            {group: 17600, version: 15},
	{Group: "authorization.k8s.io", Version: "v1beta1"}:       {group: 17600, version: 9},
	{Group: "autoscaling", Version: "v1"}:                     {group: 17500, version: 15},
	{Group: "autoscaling", Version: "v2alpha1"}:               {group: 17500, version: 9},
	{Group: "batch", Version: "v1"}:                           {group: 17400, version: 15},
	{Group: "batch", Version: "v2alpha1"}:                     {group: 17400, version: 9},
	{Group: "certificates.k8s.io", Version: "v1beta1"}:        {group: 17300, version: 9},
	{Group: "networking.k8s.io", Version: "v1"}:               {group: 17200, version: 15},
	{Group: "policy", Version: "v1beta1"}:                     {group: 17100, version: 9},
	{Group: "rbac.authorization.k8s.io", Version: "v1beta1"}:  {group: 17000, version: 12},
	{Group: "rbac.authorization.k8s.io", Version: "v1alpha1"}: {group: 17000, version: 9},
	{Group: "settings.k8s.io", Version: "v1alpha1"}:           {group: 16900, version: 9},
	{Group: "storage.k8s.io", Version: "v1"}:                  {group: 16800, version: 15},
	{Group: "storage.k8s.io", Version: "v1beta1"}:             {group: 16800, version: 9},
	{Group: "apiextensions.k8s.io", Version: "v1beta1"}:       {group: 16700, version: 9},

	// arbitrarily starting openshift around 10000.
	// bump authorization above RBAC
	{Group: "authorization.openshift.io", Version: "v1"}: {group: 17050, version: 15},
	{Group: "build.openshift.io", Version: "v1"}:         {group: 9900, version: 15},
	{Group: "apps.openshift.io", Version: "v1"}:          {group: 9900, version: 15},
	{Group: "image.openshift.io", Version: "v1"}:         {group: 9900, version: 15},
	{Group: "oauth.openshift.io", Version: "v1"}:         {group: 9900, version: 15},
	{Group: "project.openshift.io", Version: "v1"}:       {group: 9900, version: 15},
	{Group: "quota.openshift.io", Version: "v1"}:         {group: 9900, version: 15},
	{Group: "route.openshift.io", Version: "v1"}:         {group: 9900, version: 15},
	{Group: "network.openshift.io", Version: "v1"}:       {group: 9900, version: 15},
	{Group: "security.openshift.io", Version: "v1"}:      {group: 9900, version: 15},
	{Group: "template.openshift.io", Version: "v1"}:      {group: 9900, version: 15},
	{Group: "user.openshift.io", Version: "v1"}:          {group: 9900, version: 15},
}

func apiServicesToRegister(delegateAPIServer genericapiserver.DelegationTarget, registration autoregister.AutoAPIServiceRegistration) []*apiregistration.APIService {
	apiServices := []*apiregistration.APIService{}

	for _, curr := range delegateAPIServer.ListedPaths() {
		if curr == "/api/v1" {
			apiService := makeAPIService(schema.GroupVersion{Group: "", Version: "v1"})
			registration.AddAPIServiceToSync(apiService)
			apiServices = append(apiServices, apiService)
			continue
		}

		if !strings.HasPrefix(curr, "/apis/") {
			continue
		}
		// this comes back in a list that looks like /apis/rbac.authorization.k8s.io/v1alpha1
		tokens := strings.Split(curr, "/")
		if len(tokens) != 4 {
			continue
		}

		apiService := makeAPIService(schema.GroupVersion{Group: tokens[2], Version: tokens[3]})
		if apiService == nil {
			continue
		}
		registration.AddAPIServiceToSync(apiService)
		apiServices = append(apiServices, apiService)
	}

	return apiServices
}
