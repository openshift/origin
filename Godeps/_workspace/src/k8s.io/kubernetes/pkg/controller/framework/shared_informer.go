/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package framework

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

// if you use this, there is one behavior change.  When you receive a notification, the cache will be AT LEAST as
// current as the notification, but it MAY be more current.  So if there was a create, followed by a delete, the cache
// would NOT have your item.  All controllers should already be deduping on their side (if you forget to brush your teeth
// at bedtime, you don't do it twice in the morning), but I doubt many people have considered the problem
// This has advantages over the broadcaster since it allows us to share a common cache across many controllers.
// Extending the broadcaster would have required us keep duplicate caches for each watch.
type SharedInformer interface {
	AddEventHandler(handler ResourceEventHandler) error
	GetStore() cache.Store
	// GetController gives back a synthetic interface that "votes" to start the informer
	GetController() ControllerInterface
	Run(stopCh <-chan struct{})
	HasSynced() bool
}

type SharedIndexInformer interface {
	SharedInformer

	AddIndexer(indexer cache.Indexer) error
	GetIndexer() cache.Indexer
}

func NewSharedInformer(lw cache.ListerWatcher, objType runtime.Object, resyncPeriod time.Duration) SharedInformer {
	sharedInformer := &ShareableInformer{
		processor: &shareableProcessor{},
		store:     cache.NewStore(DeletionHandlingMetaNamespaceKeyFunc),
	}

	// This will hold incoming changes. Note how we pass store in as a
	// KeyLister, that way resync operations will result in the correct set
	// of update/delete deltas.
	fifo := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, nil, sharedInformer.store)

	cfg := &Config{
		Queue:            fifo,
		ListerWatcher:    lw,
		ObjectType:       objType,
		FullResyncPeriod: resyncPeriod,
		RetryOnError:     false,

		Process: sharedInformer.HandleDeltas,
	}
	sharedInformer.controller = New(cfg)

	return sharedInformer
}

type ShareableInformer struct {
	store      cache.Store
	controller *Controller

	processor *shareableProcessor

	runVotes    sync.WaitGroup
	started     bool
	startedLock sync.Mutex
}

type votingController struct {
	informer *ShareableInformer
}

func (v *votingController) Run(stopCh <-chan struct{}) {
	v.informer.runVotes.Done()
}

func (v *votingController) HasSynced() bool {
	return v.informer.HasSynced()
}

type shareableProcessor struct {
	listeners []*processorListener
}

const (
	channelDepth = 1000
)

type processorListener struct {
	notifications chan interface{}

	handler ResourceEventHandler
}

type updateNotification struct {
	oldObj interface{}
	newObj interface{}
}

type addNotification struct {
	newObj interface{}
}

type deleteNotification struct {
	oldObj interface{}
}

func NewProcessListener(handler ResourceEventHandler) *processorListener {
	return &processorListener{
		notifications: make(chan interface{}, channelDepth),
		handler:       handler,
	}
}

func (s *ShareableInformer) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()
		s.started = true
	}()

	s.runVotes.Wait()

	s.processor.Run(stopCh)
	s.controller.Run(stopCh)
}

func (s *ShareableInformer) isStarted() bool {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()
	return s.started
}

func (s *ShareableInformer) HasSynced() bool {
	return s.controller.HasSynced()
}

func (s *ShareableInformer) GetStore() cache.Store {
	return s.store
}

func (s *ShareableInformer) GetController() ControllerInterface {
	return &votingController{informer: s}
}

func (s *ShareableInformer) AddEventHandler(handler ResourceEventHandler) error {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

	if s.started {
		return fmt.Errorf("informer has already started")
	}

	// after adding your event handler, you'll need to call `Run` on your ControllerInterface to count your vote
	s.runVotes.Add(1)

	listener := NewProcessListener(handler)
	s.processor.listeners = append(s.processor.listeners, listener)
	return nil
}

func (s *ShareableInformer) HandleDeltas(obj interface{}) error {
	// from oldest to newest
	for _, d := range obj.(cache.Deltas) {
		switch d.Type {
		case cache.Sync, cache.Added, cache.Updated:
			if old, exists, err := s.store.Get(d.Object); err == nil && exists {
				if err := s.store.Update(d.Object); err != nil {
					return err
				}
				s.processor.distribute(updateNotification{oldObj: old, newObj: d.Object})
			} else {
				if err := s.store.Add(d.Object); err != nil {
					return err
				}
				s.processor.distribute(addNotification{newObj: d.Object})
			}
		case cache.Deleted:
			if err := s.store.Delete(d.Object); err != nil {
				return err
			}
			s.processor.distribute(deleteNotification{oldObj: d.Object})
		}
	}
	return nil
}

// TODO get more sophisticated about handling stuck controllers, but prove that this works for now.
func (p *shareableProcessor) distribute(obj interface{}) {
	for _, listener := range p.listeners {
		listener.notifications <- obj
	}
}

func (p *shareableProcessor) Run(stopCh <-chan struct{}) {
	for _, listener := range p.listeners {
		go listener.Run(stopCh)
	}
}

func (p *processorListener) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for {
		select {
		case <-stopCh:

		case obj := <-p.notifications:
			switch notification := obj.(type) {
			case updateNotification:
				p.handler.OnUpdate(notification.oldObj, notification.newObj)
			case addNotification:
				p.handler.OnAdd(notification.newObj)
			case deleteNotification:
				p.handler.OnDelete(notification.oldObj)

			default:
				utilruntime.HandleError(fmt.Errorf("unrecognized notification: %#v", obj))
			}
		}
	}
}
