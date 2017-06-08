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

package tpr

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	extensionsv1beta "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// this is the set of third party resources to be installed. each key is the name of the TPR to
// install, and each value is the resource to install
//
var thirdPartyResources = []v1beta1.ThirdPartyResource{
	serviceBrokerTPR,
	serviceClassTPR,
	serviceInstanceTPR,
	serviceBindingTPR,
}

// ErrTPRInstall is returned when we fail to install TPR
type ErrTPRInstall struct {
	errMsg string
}

func (e ErrTPRInstall) Error() string {
	return e.errMsg
}

// InstallTypes installs all third party resource types to the cluster
func InstallTypes(cl extensionsv1beta.ThirdPartyResourceInterface) error {
	var wg sync.WaitGroup
	errMsg := make(chan string, len(thirdPartyResources))

	for _, tpr := range thirdPartyResources {
		glog.Infof("Checking for existence of %s", tpr.Name)
		if _, err := cl.Get(tpr.Name, metav1.GetOptions{}); err == nil {
			glog.Infof("Found existing TPR %s", tpr.Name)
			continue
		}

		glog.Infof("Creating Third Party Resource Type: %s", tpr.Name)

		wg.Add(1)
		go func(tpr v1beta1.ThirdPartyResource, client extensionsv1beta.ThirdPartyResourceInterface) {
			defer wg.Done()
			if _, err := cl.Create(&tpr); err != nil {
				errMsg <- fmt.Sprintf("%s: %s", tpr.Name, err)
			} else {
				glog.Infof("Created TPR '%s'", tpr.Name)

				// There can be a delay, so poll until it's ready to go...
				err := wait.PollImmediate(1*time.Second, 1*time.Second, func() (bool, error) {
					if _, err := client.Get(tpr.Name, metav1.GetOptions{}); err == nil {
						glog.Infof("TPR %s is ready", tpr.Name)
						return true, nil
					}

					glog.Infof("TPR %s is not ready yet... waiting...", tpr.Name)
					return false, nil
				})
				if err != nil {
					glog.Infof("Error polling for TPR status:", err)
				}
			}
		}(tpr, cl)
	}

	wg.Wait()
	close(errMsg)

	var allErrMsg string
	for msg := range errMsg {
		if msg != "" {
			allErrMsg = fmt.Sprintf("%s\n%s", allErrMsg, msg)
		}
	}

	if allErrMsg != "" {
		glog.Errorf("Failed to create Third Party Resource:\n%s)", allErrMsg)
		return ErrTPRInstall{fmt.Sprintf("Failed to create Third Party Resource:%s)", allErrMsg)}
	}

	return nil
}
