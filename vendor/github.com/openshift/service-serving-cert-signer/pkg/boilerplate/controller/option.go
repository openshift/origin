package controller

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Option func(*controller)

type InformerGetter interface {
	Informer() cache.SharedIndexInformer
}

func WithMaxRetries(maxRetries int) Option {
	return func(c *controller) {
		c.maxRetries = maxRetries
	}
}

func WithRateLimiter(limiter workqueue.RateLimiter) Option {
	return func(c *controller) {
		c.queue = workqueue.NewNamedRateLimitingQueue(limiter, c.name)
	}
}

func WithInformerSynced(getter InformerGetter) Option {
	informer := getter.Informer() // immediately signal that we intend to use this informer in case it is lazily initialized
	return toRunOpt(func(c *controller) {
		c.cacheSyncs = append(c.cacheSyncs, informer.GetController().HasSynced)
	})
}

func WithInformer(getter InformerGetter, filter ParentFilter) Option {
	informer := getter.Informer() // immediately signal that we intend to use this informer in case it is lazily initialized
	return toRunOpt(func(c *controller) {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				object := metaOrDie(obj)
				if filter.Add(object) {
					glog.V(4).Infof("%s: handling add %s/%s", c.name, object.GetNamespace(), object.GetName())
					c.add(filter, object)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldObject := metaOrDie(oldObj)
				newObject := metaOrDie(newObj)
				if filter.Update(oldObject, newObject) {
					glog.V(4).Infof("%s: handling update %s/%s", c.name, newObject.GetNamespace(), newObject.GetName())
					c.add(filter, newObject)
				}
			},
			DeleteFunc: func(obj interface{}) {
				accessor, err := meta.Accessor(obj)
				if err != nil {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						utilruntime.HandleError(fmt.Errorf("could not get object from tombstone: %+v", obj))
						return
					}
					accessor, err = meta.Accessor(tombstone.Obj)
					if err != nil {
						utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not an accessor: %+v", obj))
						return
					}
				}
				if filter.Delete(accessor) {
					glog.V(4).Infof("%s: handling delete %s/%s", c.name, accessor.GetNamespace(), accessor.GetName())
					c.add(filter, accessor)
				}
			},
		})
		WithInformerSynced(getter)(c)
	})
}

func toRunOpt(opt Option) Option {
	return toOnceOpt(func(c *controller) {
		if c.run {
			opt(c)
			return
		}
		c.runOpts = append(c.runOpts, opt)
	})
}

func toOnceOpt(opt Option) Option {
	var once sync.Once
	return func(c *controller) {
		once.Do(func() {
			opt(c)
		})
	}
}

func metaOrDie(obj interface{}) v1.Object {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		panic(err) // this should never happen
	}
	return accessor
}
