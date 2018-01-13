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

package app

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/kubernetes-incubator/cluster-capacity/cmd/genpod/app/options"
	nspod "github.com/kubernetes-incubator/cluster-capacity/pkg/client"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

func NewGenPodCommand() *cobra.Command {
	opt := options.NewGenPodOptions()
	cmd := &cobra.Command{
		Use:   "genpod --kubeconfig KUBECONFIG --namespace NAMESPACE",
		Short: "Generate pod based on namespace resource limits and node selector annotations",
		Long:  "Generate pod based on namespace resource limits and node selector annotations",
		Run: func(cmd *cobra.Command, args []string) {
			err := Validate(opt)
			if err != nil {
				fmt.Println(err)
				cmd.Help()
				return
			}
			err = Run(opt)
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	opt.AddFlags(cmd.Flags())
	return cmd
}

func Validate(opt *options.GenPodOptions) error {
	if len(opt.Namespace) == 0 {
		return fmt.Errorf("Cluster namespace missing")
	}

	if len(opt.Format) > 0 && opt.Format != "json" && opt.Format != "yaml" {
		return fmt.Errorf("Output format %v not recognized: only json and yaml are allowed", opt.Format)
	}

	return nil
}

func Run(opt *options.GenPodOptions) error {
	var cfg *restclient.Config
	_, present := os.LookupEnv("CC_INCLUSTER")
	if !present {
		master, err := utils.GetMasterFromKubeConfig(opt.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
		}

		cfg, err = clientcmd.BuildConfigFromFlags(master, opt.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Unable to build config: %v", err)
		}
	} else {
		var err error
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			return fmt.Errorf("Unable to build in cluster config: %v", err)
		}
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pod, err := nspod.RetrieveNamespacePod(client, opt.Namespace)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	return utils.PrintPod(pod, opt.Format)

}
