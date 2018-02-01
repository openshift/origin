/*
Copyright 2018 The Kubernetes Authors.

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

package svcat

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/kubernetes-incubator/service-catalog/pkg/svcat/kube"
	"github.com/kubernetes-incubator/service-catalog/pkg/svcat/service-catalog"
	"k8s.io/client-go/rest"
)

// App is the underlying application behind the svcat cli.
type App struct {
	*servicecatalog.SDK
}

// NewApp creates an svcat application.
func NewApp(kubeConfig, kubeContext string) (*App, error) {
	// Initialize a service catalog client
	_, cl, err := getKubeClient(kubeConfig, kubeContext)
	if err != nil {
		return nil, err
	}

	app := &App{
		SDK: &servicecatalog.SDK{
			ServiceCatalogClient: cl,
		},
	}

	return app, nil
}

// configForContext creates a Kubernetes REST client configuration for a given kubeconfig context.
func configForContext(kubeConfig, kubeContext string) (*rest.Config, error) {
	config, err := kube.GetConfig(kubeContext, kubeConfig).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", kubeContext, err)
	}
	return config, nil
}

// getKubeClient creates a Kubernetes config and client for a given kubeconfig context.
func getKubeClient(kubeConfig, kubeContext string) (*rest.Config, *clientset.Clientset, error) {
	config, err := configForContext(kubeConfig, kubeContext)
	if err != nil {
		return nil, nil, fmt.Errorf("could not load Kubernetes configuration (%s)", err)
	}

	client, err := clientset.NewForConfig(config)
	return nil, client, err
}
