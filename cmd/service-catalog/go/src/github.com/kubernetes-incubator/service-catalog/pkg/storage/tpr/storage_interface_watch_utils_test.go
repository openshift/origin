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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kubernetes-incubator/service-catalog/pkg/rest/core/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
)

const watchTimeout = 1 * time.Second

func runWatchTest(keyer Keyer, fakeCl *fake.RESTClient, iface storage.Interface, obj runtime.Object) error {
	key, err := keyer.Key(request.NewContext(), name)
	if err != nil {
		return fmt.Errorf("error creating new key from Keyer (%s)", err)
	}
	resourceVsn := "1234"
	predicate := storage.SelectionPredicate{}

	sendObjErrCh := make(chan error)
	go func() {
		if err := fakeCl.Watcher.SendObject(watch.Added, obj, 1*time.Second); err != nil {
			sendObjErrCh <- err
			return
		}
		fakeCl.Watcher.Close()
	}()

	watchIface, err := iface.Watch(context.Background(), key, resourceVsn, predicate)
	if err != nil {
		return err
	}
	if watchIface == nil {
		return err
	}
	defer watchIface.Stop()
	ch := watchIface.ResultChan()
	select {
	case err := <-sendObjErrCh:
		return fmt.Errorf("error sending object (%s)", err)
	case evt, ok := <-ch:
		if !ok {
			return errors.New("watch channel was closed")
		}
		if evt.Type != watch.Added {
			return errors.New("event type was not ADDED")
		}
		if err := deepCompare("expected", obj, "actual", evt.Object); err != nil {
			return fmt.Errorf("received objects aren't the same (%s)", err)
		}
	case <-time.After(watchTimeout):
		return fmt.Errorf("didn't receive an event within %s", watchTimeout)
	}
	select {
	case _, ok := <-ch:
		if ok {
			return errors.New("watch channel was not closed")
		}
	case <-time.After(watchTimeout):
		return fmt.Errorf("watch channel didn't receive after %s", watchTimeout)
	}
	return nil
}

func runWatchListTest(keyer Keyer, fakeCl *fake.RESTClient, iface storage.Interface, obj runtime.Object) error {
	const timeout = 1 * time.Second
	reqCtx := request.NewContext()
	reqCtx = request.WithNamespace(reqCtx, namespace)
	key := keyer.KeyRoot(reqCtx)
	resourceVsn := "1234"
	predicate := storage.SelectionPredicate{}
	sendObjErrCh := make(chan error)
	go func() {
		defer fakeCl.Watcher.Close()
		if err := fakeCl.Watcher.SendObject(watch.Added, obj, 1*time.Second); err != nil {
			sendObjErrCh <- err
			return
		}
	}()
	watchIface, err := iface.WatchList(context.Background(), key, resourceVsn, predicate)
	if err != nil {
		return err
	}
	if watchIface == nil {
		return errors.New("expected non-nil watch interface")
	}
	defer watchIface.Stop()
	ch := watchIface.ResultChan()
	select {
	case err := <-sendObjErrCh:
		return fmt.Errorf("error sending object (%s)", err)
	case evt, ok := <-ch:
		if !ok {
			return errors.New("watch channel was closed")
		}
		if evt.Type != watch.Added {
			return errors.New("event type was not ADDED")
		}
		if err := deepCompare("expected", obj, "actual", evt.Object); err != nil {
			return fmt.Errorf("received objects aren't the same (%s)", err)
		}
	case <-time.After(watchTimeout):
		return fmt.Errorf("didn't receive after %s", watchTimeout)
	}
	select {
	case _, ok := <-ch:
		if ok {
			return errors.New("watch channel was not closed")
		}
	case <-time.After(watchTimeout):
		return fmt.Errorf("watch channel didn't receive after %s", watchTimeout)
	}
	return nil
}
