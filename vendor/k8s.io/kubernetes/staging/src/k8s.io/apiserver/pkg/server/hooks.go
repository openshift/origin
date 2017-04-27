/*
Copyright 2014 The Kubernetes Authors.

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

package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/healthz"
	restclient "k8s.io/client-go/rest"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
)

// PostStartHookFunc is a function that is called after the server has started.
// It must properly handle cases like:
//  1. asynchronous start in multiple API server processes
//  2. conflicts between the different processes all trying to perform the same action
//  3. partially complete work (API server crashes while running your hook)
//  4. API server access **BEFORE** your hook has completed
// Think of it like a mini-controller that is super privileged and gets to run in-process
// If you use this feature, tag @deads2k on github who has promised to review code for anyone's PostStartHook
// until it becomes easier to use.
type PostStartHookFunc func(context PostStartHookContext) error

// PostStartHookContext provides information about this API server to a PostStartHookFunc
type PostStartHookContext struct {
	// LoopbackClientConfig is a config for a privileged loopback connection to the API server
	LoopbackClientConfig *restclient.Config
}

// PostStartHookProvider is an interface in addition to provide a post start hook for the api server
type PostStartHookProvider interface {
	PostStartHook() (string, PostStartHookFunc, error)
}

type postStartHookEntry struct {
	hook PostStartHookFunc

	// done will be closed when the postHook is finished
	done chan struct{}
}

// AddPostStartHook allows you to add a PostStartHook.
func (s *GenericAPIServer) AddPostStartHook(name string, hook PostStartHookFunc) error {
	if len(name) == 0 {
		return fmt.Errorf("missing name")
	}
	if hook == nil {
		return nil
	}
	if s.disabledPostStartHooks.Has(name) {
		return nil
	}

	s.postStartHookLock.Lock()
	defer s.postStartHookLock.Unlock()

	if s.postStartHooksCalled {
		return fmt.Errorf("unable to add %q because PostStartHooks have already been called", name)
	}
	if _, exists := s.postStartHooks[name]; exists {
		return fmt.Errorf("unable to add %q because it is already registered", name)
	}

	// done is closed when the poststarthook is finished.  This is used by the health check to be able to indicate
	// that the poststarthook is finished
	done := make(chan struct{})
	s.AddHealthzChecks(postStartHookHealthz{name: "poststarthook/" + name, done: done})
	s.postStartHooks[name] = postStartHookEntry{hook: hook, done: done}

	return nil
}

// RunPostStartHooks runs the PostStartHooks for the server
func (s *GenericAPIServer) RunPostStartHooks() {
	s.postStartHookLock.Lock()
	defer s.postStartHookLock.Unlock()
	s.postStartHooksCalled = true

	context := PostStartHookContext{LoopbackClientConfig: s.LoopbackClientConfig}

	// only wait for namespace in a real kube master setup. In unit tests, we don't have the namespace resource
	// and therefore the wait loop would fail.
	isKubeMaster := false
	for _, ws := range s.HandlerContainer.RegisteredWebServices() {
		if ws.RootPath() == "/api/v1" {
			isKubeMaster = true
		}
	}
	if isKubeMaster {
		// we need to wait until our loopback client has enough rights to actually loopback
		// the core APIs cannot be disabled, so we'll use that as a proxy for the information instead
		// of attempting a SAR (which can be disabled, though that would be a very bad idea)
		// TODO remove once openshift starts using the kube loopback connection.
		err := wait.PollImmediate(30*time.Millisecond, 30*time.Second, func() (bool, error) {
			client := coreclient.NewForConfigOrDie(s.LoopbackClientConfig)
			// we can have other errors in our level, like etcd not being up. Treat them all the same
			if _, err := client.Namespaces().List(metav1.ListOptions{}); err != nil {
				fmt.Printf("loopback error: %v\n", err)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			glog.Fatalf("LoopbackClient was unable to list namespaces: %v", err)
		}
	}

	for hookName, hookEntry := range s.postStartHooks {
		go runPostStartHook(hookName, hookEntry, context)
	}
}

func runPostStartHook(name string, entry postStartHookEntry, context PostStartHookContext) {
	var err error
	func() {
		// don't let the hook *accidentally* panic and kill the server
		defer utilruntime.HandleCrash()
		err = entry.hook(context)
	}()
	// if the hook intentionally wants to kill server, let it.
	if err != nil {
		glog.Fatalf("PostStartHook %q failed: %v", name, err)
	}

	close(entry.done)
}

// postStartHookHealthz implements a healthz check for poststarthooks.  It will return a "hookNotFinished"
// error until the poststarthook is finished.
type postStartHookHealthz struct {
	name string

	// done will be closed when the postStartHook is finished
	done chan struct{}
}

var _ healthz.HealthzChecker = postStartHookHealthz{}

func (h postStartHookHealthz) Name() string {
	return h.name
}

var hookNotFinished = errors.New("not finished")

func (h postStartHookHealthz) Check(req *http.Request) error {
	select {
	case <-h.done:
		return nil
	default:
		return hookNotFinished
	}
}
