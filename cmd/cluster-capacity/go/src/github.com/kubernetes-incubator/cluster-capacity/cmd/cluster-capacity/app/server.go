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
	"os"

	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"

	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/kubernetes-incubator/cluster-capacity/cmd/cluster-capacity/app/options"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

var (
	clusterCapacityLong = dedent.Dedent(`
		Cluster-capacity simulates an API server with initial state copied from the Kubernetes enviroment
		with its configuration specified in KUBECONFIG. The simulated API server tries to schedule the number of
		pods specified by --max-limits flag. If the --max-limits flag is not specified, pods are scheduled until
		the simulated API server runs out of resources.
	`)
)

func NewClusterCapacityCommand() *cobra.Command {
	opt := options.NewClusterCapacityOptions()
	cmd := &cobra.Command{
		Use:   "cluster-capacity --kubeconfig KUBECONFIG --podspec PODSPEC",
		Short: "Cluster-capacity is used for simulating scheduling of one or multiple pods",
		Long:  clusterCapacityLong,
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

func Validate(opt *options.ClusterCapacityOptions) error {
	if len(opt.PodSpecFile) == 0 {
		return fmt.Errorf("Pod spec file is missing")
	}

	_, present := os.LookupEnv("CC_INCLUSTER")
	if !present {
		if len(opt.Kubeconfig) == 0 {
			return fmt.Errorf("kubeconfig is missing")
		}
	}
	return nil
}

func Run(opt *options.ClusterCapacityOptions) error {
	conf := options.NewClusterCapacityConfig(opt)

	err := conf.SetDefaultScheduler()
	if err != nil {
		return fmt.Errorf("Failed to create default scheduler server: %v ", err)
	}

	// TODO (avesh): Enable when support for multiple schedulers is implemented.
	/*err = conf.ParseAdditionalSchedulerConfigs()
	if err != nil {
		return fmt.Errorf("Failed to parse config file: %v ", err)
	}*/

	err = conf.ParseAPISpec()
	if err != nil {
		return fmt.Errorf("Failed to parse pod spec file: %v ", err)
	}

	var cfg *restclient.Config
	if len(conf.Options.Kubeconfig) != 0 {
		master, err := utils.GetMasterFromKubeConfig(conf.Options.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
		}

		cfg, err = clientcmd.BuildConfigFromFlags(master, conf.Options.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Unable to build config: %v", err)
		}

	} else {
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			return fmt.Errorf("Unable to build in cluster config: %v", err)
		}
	}

	conf.KubeClient, err = clientset.NewForConfig(cfg)

	if err != nil {
		return err
	}

	report, err := runSimulator(conf, true)
	if err != nil {
		return err
	}
	if err := framework.ClusterCapacityReviewPrint(report, conf.Options.Verbose, conf.Options.OutputFormat); err != nil {
		return fmt.Errorf("Error while printing: %v", err)
	}
	return nil
}

func runSimulator(s *options.ClusterCapacityConfig, syncWithClient bool) (*framework.ClusterCapacityReview, error) {
	cc, err := framework.New(s.DefaultScheduler, s.Pod, s.Options.MaxLimit)
	if err != nil {
		return nil, err
	}

	// TODO (avesh): Enable when support for multiple schedulers is implemented.
	/*for i := 0; i < len(s.Schedulers); i++ {
		if err = cc.AddScheduler(s.Schedulers[i]); err != nil {
			return nil, err
		}
	}*/

	if syncWithClient {
		err = cc.SyncWithClient(s.KubeClient)
	} else {
		err = cc.SyncWithStore(s.ResourceStore)
	}
	if err != nil {
		return nil, err
	}

	err = cc.Run()
	if err != nil {
		return nil, err
	}

	report := cc.Report()
	return report, nil
}
