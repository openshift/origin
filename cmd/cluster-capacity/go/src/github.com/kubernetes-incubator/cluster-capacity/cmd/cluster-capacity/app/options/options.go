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

package options

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"
	api "k8s.io/kubernetes/pkg/apis/core"
	apiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	schedapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

type ClusterCapacityConfig struct {
	//Schedulers       []*schedapp.SchedulerServer
	Pod              *v1.Pod
	KubeClient       clientset.Interface
	Options          *ClusterCapacityOptions
	DefaultScheduler *schedapp.SchedulerServer
	ResourceStore    store.ResourceStore
}

type ClusterCapacityOptions struct {
	Kubeconfig string
	//SchedulerConfigFile        []string
	DefaultSchedulerConfigFile string
	MaxLimit                   int
	Verbose                    bool
	PodSpecFile                string
	OutputFormat               string
	//ResourceSpaceMode          string
}

func NewClusterCapacityConfig(opt *ClusterCapacityOptions) *ClusterCapacityConfig {
	return &ClusterCapacityConfig{
		Options: opt,
	}
}

func NewClusterCapacityOptions() *ClusterCapacityOptions {
	return &ClusterCapacityOptions{}
}

func (s *ClusterCapacityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to the kubeconfig file to use for the analysis.")
	fs.StringVar(&s.PodSpecFile, "podspec", s.PodSpecFile, "Path to JSON or YAML file containing pod definition.")
	fs.IntVar(&s.MaxLimit, "max-limit", 0, "Number of instances of pod to be scheduled after which analysis stops. By default unlimited.")

	//TODO(jchaloup): uncomment this line once the multi-schedulers are fully implemented
	//fs.StringArrayVar(&s.SchedulerConfigFile, "config", s.SchedulerConfigFile, "Paths to files containing scheduler configuration in JSON or YAML format")

	fs.StringVar(&s.DefaultSchedulerConfigFile, "default-config", s.DefaultSchedulerConfigFile, "Path to JSON or YAML file containing scheduler configuration.")

	fs.BoolVar(&s.Verbose, "verbose", s.Verbose, "Verbose mode")
	fs.StringVarP(&s.OutputFormat, "output", "o", s.OutputFormat, "Output format. One of: json|yaml (Note: output is not versioned or guaranteed to be stable across releases).")
}

func (s *ClusterCapacityConfig) parseSchedulerConfig(path string) (*schedapp.SchedulerServer, error) {
	soptions, err := schedapp.NewOptions()
	if err != nil {
		return nil, err
	}

	err = soptions.ReallyApplyDefaults()
	if err != nil {
		return nil, err
	}

	soptions.ConfigFile = path

	err = soptions.Complete()
	if err != nil {
		return nil, err
	}

	ksConfig := soptions.GetConfig()
	ksConfig.ClientConnection.KubeConfigFile = s.Options.Kubeconfig

	if len(path) > 0 {
		filename, _ := filepath.Abs(path)
		config, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to open config file: %v", err)
		}

		decoder := yaml.NewYAMLOrJSONDecoder(config, 4096)
		decoder.Decode(ksConfig)
	}

	// In a POD, master is passed as empty string.
	var master string
	_, present := os.LookupEnv("CC_INCLUSTER")
	if !present {
		master, err = utils.GetMasterFromKubeConfig(s.Options.Kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	// TODO(avesh): need to check if this works correctly
	algorithmprovider.ApplyFeatureGates()

	var newScheduler *schedapp.SchedulerServer
	newScheduler, err = schedapp.NewSchedulerServer(ksConfig, master)
	if err != nil {
		return nil, err
	}

	newScheduler.SchedulerName = "cluster-capacity"
	return newScheduler, nil
}

// TODO (avesh): disable until support for multiple schedulers is implemented.
/*func (s *ClusterCapacityConfig) ParseAdditionalSchedulerConfigs() error {
	for _, config := range s.Options.SchedulerConfigFile {
		if config == "default-scheduler.yaml" {
			continue
		}
		newScheduler, err := s.parseSchedulerConfig(config)
		if err != nil {
			return err
		}
		s.Schedulers = append(s.Schedulers, newScheduler)
	}
	return nil
}*/

func (s *ClusterCapacityConfig) ParseAPISpec() error {
	var spec io.Reader
	var err error
	if strings.HasPrefix(s.Options.PodSpecFile, "http://") || strings.HasPrefix(s.Options.PodSpecFile, "https://") {
		response, err := http.Get(s.Options.PodSpecFile)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("unable to read URL %q, server reported %v, status code=%v", s.Options.PodSpecFile, response.Status, response.StatusCode)
		}
		spec = response.Body
	} else {
		filename, _ := filepath.Abs(s.Options.PodSpecFile)
		spec, err = os.Open(filename)
		if err != nil {
			return fmt.Errorf("Failed to open config file: %v", err)
		}
	}

	decoder := yaml.NewYAMLOrJSONDecoder(spec, 4096)
	versionedPod := &v1.Pod{}
	err = decoder.Decode(versionedPod)
	if err != nil {
		return fmt.Errorf("Failed to decode config file: %v", err)
	}

	if versionedPod.ObjectMeta.Namespace == "" {
		versionedPod.ObjectMeta.Namespace = "default"
	}

	// set pod's scheduler name to cluster-capacity
	if versionedPod.Spec.SchedulerName == "" {
		versionedPod.Spec.SchedulerName = s.DefaultScheduler.SchedulerName
	}

	// hardcoded from kube api defaults and validation
	// TODO: rewrite when object validation gets more available for non kubectl approaches in kube
	if versionedPod.Spec.DNSPolicy == "" {
		versionedPod.Spec.DNSPolicy = v1.DNSClusterFirst
	}
	if versionedPod.Spec.RestartPolicy == "" {
		versionedPod.Spec.RestartPolicy = v1.RestartPolicyAlways
	}

	for i := range versionedPod.Spec.Containers {
		if versionedPod.Spec.Containers[i].TerminationMessagePolicy == "" {
			versionedPod.Spec.Containers[i].TerminationMessagePolicy = v1.TerminationMessageFallbackToLogsOnError
		}
	}

	// TODO: client side validation seems like a long term problem for this command.
	internalPod := &api.Pod{}
	if err := apiv1.Convert_v1_Pod_To_core_Pod(versionedPod, internalPod, nil); err != nil {
		return fmt.Errorf("unable to convert to internal version: %#v", err)

	}
	if errs := validation.ValidatePod(internalPod); len(errs) > 0 {
		var errStrs []string
		for _, err := range errs {
			errStrs = append(errStrs, fmt.Sprintf("%v: %v", err.Type, err.Field))
		}
		return fmt.Errorf("Invalid pod: %#v", strings.Join(errStrs, ", "))
	}

	s.Pod = versionedPod
	return nil
}

func (s *ClusterCapacityConfig) SetDefaultScheduler() error {
	var err error
	s.DefaultScheduler, err = s.parseSchedulerConfig(s.Options.DefaultSchedulerConfigFile)
	if err != nil {
		return fmt.Errorf("Error in opening default scheduler config file: %v", err)
	}

	return nil
}
