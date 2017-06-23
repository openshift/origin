package common

import (
	"reflect"

	"github.com/golang/glog"

	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kcache "k8s.io/client-go/tools/cache"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
)

type SDNInformers struct {
	KubeInformers    kinternalinformers.SharedInformerFactory
	NetworkInformers networkinformers.SharedInformerFactory
}

type InformerAddOrUpdateFunc func(interface{}, interface{}, watch.EventType)
type InformerDeleteFunc func(interface{})

func InformerFuncs(objType runtime.Object, addOrUpdateFunc InformerAddOrUpdateFunc, deleteFunc InformerDeleteFunc) kcache.ResourceEventHandlerFuncs {
	handlerFuncs := kcache.ResourceEventHandlerFuncs{}
	if addOrUpdateFunc != nil {
		handlerFuncs.AddFunc = func(obj interface{}) {
			addOrUpdateFunc(obj, nil, watch.Added)
		}
		handlerFuncs.UpdateFunc = func(old, cur interface{}) {
			addOrUpdateFunc(cur, old, watch.Modified)
		}
	}
	if deleteFunc != nil {
		handlerFuncs.DeleteFunc = func(obj interface{}) {
			if reflect.TypeOf(objType) != reflect.TypeOf(obj) {
				tombstone, ok := obj.(kcache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("Couldn't get object from tombstone: %+v", obj)
					return
				}

				obj = tombstone.Obj
				if reflect.TypeOf(objType) != reflect.TypeOf(obj) {
					glog.Errorf("Tombstone contained object, expected resource type: %v but got: %v", reflect.TypeOf(objType), reflect.TypeOf(obj))
					return
				}
			}
			deleteFunc(obj)
		}
	}
	return handlerFuncs
}
