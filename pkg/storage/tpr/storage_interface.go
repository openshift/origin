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
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	restclient "k8s.io/client-go/rest"
)

var (
	errNotImplemented = errors.New("not implemented for third party resources")
)

type store struct {
	hasNamespace     bool
	codec            runtime.Codec
	defaultNamespace string
	cl               restclient.Interface
	singularKind     Kind
	singularShell    func(string, string) runtime.Object
	listKind         Kind
	listShell        func() runtime.Object
	checkObject      func(runtime.Object) error
	decodeKey        func(string) (string, string, error)
	versioner        storage.Versioner
}

// NewStorage creates a new TPR-based storage.Interface implementation
func NewStorage(opts Options) (storage.Interface, factory.DestroyFunc) {
	return &store{
		hasNamespace:     opts.HasNamespace,
		codec:            opts.RESTOptions.StorageConfig.Codec,
		defaultNamespace: opts.DefaultNamespace,
		cl:               opts.RESTClient,
		singularKind:     opts.SingularKind,
		singularShell:    opts.NewSingularFunc,
		listKind:         opts.ListKind,
		listShell:        opts.NewListFunc,
		checkObject:      opts.CheckObjectFunc,
		decodeKey:        opts.Keyer.NamespaceAndNameFromKey,
		versioner:        etcd.APIObjectVersioner{},
	}, opts.DestroyFunc
}

// Versioned returns the versioned associated with this interface
func (t *store) Versioner() storage.Versioner {
	return t.versioner
}

// Create adds a new object at a key unless it already exists. 'ttl' is time-to-live
// in seconds (0 means forever). If no error is returned and out is not nil, out will be
// set to the read value from database.
func (t *store) Create(
	ctx context.Context,
	key string,
	obj,
	out runtime.Object,
	ttl uint64,
) error {

	ns, name, err := t.decodeKey(key)
	if err != nil {
		glog.Errorf("decoding key %s (%s)", key, err)
		return err
	}

	data, err := runtime.Encode(t.codec, obj)
	if err != nil {
		return err
	}
	req := t.cl.Post().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		t.singularKind.URLName(),
	).Body(data)

	res := req.Do()
	if res.Error() != nil {
		glog.Errorf("executing POST for %s/%s (%s)", ns, name, res.Error())
	}
	var statusCode int
	res.StatusCode(&statusCode)
	if statusCode == http.StatusConflict {
		return storage.NewKeyExistsError(key, 0)
	}
	if statusCode != http.StatusCreated {
		return fmt.Errorf(
			"executing POST for %s/%s, received response code %d",
			ns,
			name,
			statusCode,
		)
	}

	var unknown runtime.Unknown
	if err := res.Into(&unknown); err != nil {
		glog.Errorf("decoding response (%s)", err)
		return err
	}

	if err := decode(t.codec, unknown.Raw, out); err != nil {
		return err
	}
	return nil
}

// Delete removes the specified key and returns the value that existed at that spot.
// If key didn't exist, it will return NotFound storage error.
//
// In this implementation, Delete will not write the deleted object back to out
func (t *store) Delete(
	ctx context.Context,
	key string,
	out runtime.Object,
	preconditions *storage.Preconditions,
) error {
	ns, name, err := t.decodeKey(key)
	if err != nil {
		glog.Errorf("decoding key %s (%s)", key, err)
		return err
	}

	req := t.cl.Delete().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		t.singularKind.URLName(),
		name,
	)

	res := req.Do()
	if res.Error() != nil {
		glog.Errorf("executing DELETE for %s/%s (%s)", ns, name, res.Error())
	}
	var statusCode int
	res.StatusCode(&statusCode)
	if statusCode == http.StatusNotFound {
		return storage.NewKeyNotFoundError(key, 0)
	}
	if statusCode != http.StatusAccepted {
		return fmt.Errorf(
			"executing DELETE for %s/%s, received response code %d",
			ns,
			name,
			statusCode,
		)
	}

	return nil
}

// Watch begins watching the specified key. Events are decoded into API objects,
// and any items selected by 'p' are sent down to returned watch.Interface.
// resourceVersion may be used to specify what version to begin watching,
// which should be the current resourceVersion, and no longer rv+1
// (e.g. reconnecting without missing any updates).
// If resource version is "0", this interface will get current object at given key
// and send it in an "ADDED" event, before watch starts.
func (t *store) Watch(
	ctx context.Context,
	key string,
	resourceVersion string,
	p storage.SelectionPredicate,
) (watch.Interface, error) {
	ns, name, err := t.decodeKey(key)
	if err != nil {
		return nil, err
	}

	req := t.cl.Get().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"watch",
		"namespaces",
		ns,
		t.singularKind.URLName(),
		name,
	).Param("resourceVersion", resourceVersion)
	watchIface, err := req.Watch()
	if err != nil {
		glog.Errorf("initiating the raw watch (%s)", err)
		return nil, err
	}
	filteredIFace := watch.Filter(watchIface, watchFilterer(t, ns, false))
	return filteredIFace, nil
}

// watchFilterer returns a function that can be used as an argument to watch.Filter
func watchFilterer(t *store, ns string, list bool) func(watch.Event) (watch.Event, bool) {
	return func(in watch.Event) (watch.Event, bool) {
		encodedBytes, err := runtime.Encode(t.codec, in.Object)
		if err != nil {
			glog.Errorf("couldn't encode watch event object (%s)", err)
			return watch.Event{}, false
		}
		if list {
			// if we're watching a list, extract to a list object
			finalObj := t.listShell()
			if err := decode(t.codec, encodedBytes, finalObj); err != nil {
				glog.Errorf("couldn't decode watch event bytes (%s)", err)
				return watch.Event{}, false
			}
			if !t.hasNamespace {
				// if we're watching a list and not supposed to have a namespace, strip namespaces
				objs, err := meta.ExtractList(finalObj)
				if err != nil {
					glog.Errorf("couldn't extract a list from %#v (%s)", finalObj, err)
					return watch.Event{}, false
				}
				objList := make([]runtime.Object, len(objs))
				for i, obj := range objs {
					if err := removeNamespace(obj); err != nil {
						glog.Errorf("couldn't remove namespace from %#v (%s)", obj, err)
						return watch.Event{}, false
					}
					objList[i] = obj
				}
				if err := meta.SetList(finalObj, objList); err != nil {
					glog.Errorf("setting list items (%s)", err)
					return watch.Event{}, false
				}
				return watch.Event{
					Type:   in.Type,
					Object: finalObj,
				}, true
			}
			return watch.Event{
				Type:   in.Type,
				Object: finalObj,
			}, true
		}
		finalObj := t.singularShell("", "")
		if err := decode(t.codec, encodedBytes, finalObj); err != nil {
			glog.Errorf("couldn't decode watch event bytes (%s)", err)
			return watch.Event{}, false
		}
		if !t.hasNamespace {
			if err := removeNamespace(finalObj); err != nil {
				glog.Errorf("couldn't remove namespace from %#v (%s)", finalObj, err)
				return watch.Event{}, false
			}
		}
		return watch.Event{
			Type:   in.Type,
			Object: finalObj,
		}, true
	}

}

// WatchList begins watching the specified key's items. Items are decoded into API
// objects and any item selected by 'p' are sent down to returned watch.Interface.
// resourceVersion may be used to specify what version to begin watching,
// which should be the current resourceVersion, and no longer rv+1
// (e.g. reconnecting without missing any updates).
// If resource version is "0", this interface will list current objects directory defined by key
// and send them in "ADDED" events, before watch starts.
func (t *store) WatchList(
	ctx context.Context,
	key string,
	resourceVersion string,
	p storage.SelectionPredicate,
) (watch.Interface, error) {
	ns, _, err := t.decodeKey(key)
	if err != nil {
		return nil, err
	}

	req := t.cl.Get().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"watch",
		"namespaces",
		ns,
		t.singularKind.URLName(),
	).Param("resourceVersion", resourceVersion)

	watchIface, err := req.Watch()
	if err != nil {
		glog.Errorf("initiating the raw watch (%s)", err)
		return nil, err
	}
	return watch.Filter(watchIface, watchFilterer(t, ns, true)), nil
}

// Get unmarshals json found at key into objPtr. On a not found error, will either
// return a zero object of the requested type, or an error, depending on ignoreNotFound.
// Treats empty responses and nil response nodes exactly like a not found error.
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (t *store) Get(
	ctx context.Context,
	key string,
	resourceVersion string,
	objPtr runtime.Object,
	ignoreNotFound bool,
) error {
	ns, name, err := t.decodeKey(key)
	if err != nil {
		glog.Errorf("decoding key %s (%s)", key, err)
		return err
	}
	req := t.cl.Get().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		t.singularKind.URLName(),
		name,
	)

	res := req.Do()
	if res.Error() != nil {
		glog.Errorf("executing GET for %s/%s (%s)", ns, name, res.Error())
	}
	var statusCode int
	res.StatusCode(&statusCode)
	if statusCode == http.StatusNotFound {
		if ignoreNotFound {
			return runtime.SetZeroValue(objPtr)
		}
		glog.Errorf("executing GET for %s/%s: not found", ns, name)
		return storage.NewKeyNotFoundError(key, 0)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf(
			"executing GET for %s/%s, received response code %d",
			ns,
			name,
			statusCode,
		)
	}

	var unknown runtime.Unknown
	if res.Into(&unknown); err != nil {
		glog.Errorf("decoding response (%s)", err)
		return err
	}

	if err := decode(t.codec, unknown.Raw, objPtr); err != nil {
		return nil
	}
	if !t.hasNamespace {
		if err := removeNamespace(objPtr); err != nil {
			glog.Errorf("removing namespace from %#v (%s)", objPtr, err)
			return err
		}
	}
	return nil
}

// GetToList unmarshals json found at key and opaque it into *List api object
// (an object that satisfies the runtime.IsList definition).
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (t *store) GetToList(
	ctx context.Context,
	key string,
	resourceVersion string,
	p storage.SelectionPredicate,
	listObj runtime.Object,
) error {
	return t.List(ctx, key, resourceVersion, p, listObj)
}

// List unmarshalls jsons found at directory defined by key and opaque them
// into *List api object (an object that satisfies runtime.IsList definition).
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (t *store) List(
	ctx context.Context,
	key string,
	resourceVersion string,
	p storage.SelectionPredicate,
	listObj runtime.Object,
) error {
	ns, _, err := t.decodeKey(key)
	if err != nil {
		glog.Errorf("decoding %s (%s)", key, err)
		return err
	}

	if t.hasNamespace && ns == t.defaultNamespace {
		// if the resource is supposed to have a namespace, and the given one is the default,
		// then assume that '--all-namespaces' was given on the kubectl command line.
		// this assumption means that a kubectl command that specifies a namespace equal to
		// the default namespace (i.e. '-n default-ns'), we will still list all resources.
		//
		// to list all resources, we get all namespaces, list all resources in each namespace,
		// and then collect all resources into the single listObj
		allNamespaces, err := getAllNamespaces(t.cl)
		if err != nil {
			glog.Errorf("listing all namespaces (%s)", err)
			return err
		}
		var objList []runtime.Object
		for _, ns := range allNamespaces.Items {
			allObjs, err := listResource(t.cl, ns.Name, t.singularKind, listObj, t.codec)
			if err != nil {
				glog.Errorf("error listing resources (%s)", err)
				return err
			}
			objList = append(objList, allObjs...)
		}
		if err := meta.SetList(listObj, objList); err != nil {
			glog.Errorf("setting list items (%s)", err)
			return err
		}
		return nil
	}

	// otherwise, list all the resources in the given namespace. if the resource is not supposed
	// to be namespaced, then ns will be the default namespace
	objs, err := listResource(t.cl, ns, t.singularKind, listObj, t.codec)
	if err != nil {
		glog.Errorf("listing resources (%s)", err)
		return err
	}
	if !t.hasNamespace {
		for i, obj := range objs {
			if err := removeNamespace(obj); err != nil {
				glog.Errorf("removing namespace from obj %d (%s)", i, err)
				return err
			}
		}
	}
	if err := meta.SetList(listObj, objs); err != nil {
		glog.Errorf("setting list items (%s)", err)
		return err
	}
	return nil
}

// GuaranteedUpdate implements storage.Interface.GuaranteedUpdate.
func (t *store) GuaranteedUpdate(
	ctx context.Context,
	key string,
	out runtime.Object,
	ignoreNotFound bool,
	precondtions *storage.Preconditions,
	userUpdate storage.UpdateFunc,
	suggestion ...runtime.Object,
) error {
	// If a suggestion was passed, use that as the initial object, otherwise
	// use Get() to retrieve it
	var initObj runtime.Object
	if len(suggestion) == 1 && suggestion[0] != nil {
		initObj = suggestion[0]
	} else {
		initObj = t.singularShell("", "")
		if err := t.Get(ctx, key, "", initObj, ignoreNotFound); err != nil {
			glog.Errorf("getting initial object (%s)", err)
			return err
		}
	}
	// In either case, extract current state from the initial object
	curState, err := t.getStateFromObject(initObj)
	if err != nil {
		glog.Errorf("getting state from initial object (%s)", err)
		return err
	}
	// Loop until update succeeds or we get an error
	for {
		if err := checkPreconditions(key, precondtions, curState.obj); err != nil {
			glog.Errorf("checking preconditions (%s)", err)
			return err
		}
		// Create a candidate for the new object by applying the userUpdate func
		candidate, _, err := userUpdate(curState.obj, *curState.meta)
		if err != nil {
			glog.Errorf("applying user update: (%s)", err)
			return err
		}
		// Get bytes from the candidate
		candidateData, err := runtime.Encode(t.codec, candidate)
		if err != nil {
			glog.Errorf("encoding candidate obj (%s)", err)
			return err
		}
		// If the candidate matches what we already have, then all we need to do is
		// decode into the out object
		if bytes.Equal(candidateData, curState.data) {
			err := decode(t.codec, candidateData, out)
			if err != nil {
				glog.Errorf("decoding to output object (%s)", err)
			}
			return err
		}
		// Otherwise, get an up-to-date copy of the resource we're trying to update
		// (because it may have changed if we're looping and in a race)
		newCurObj := t.singularShell("", "")
		if err := t.Get(ctx, key, "", newCurObj, ignoreNotFound); err != nil {
			glog.Errorf("getting new current object (%s)", err)
			return err
		}
		newCurState, err := t.getStateFromObject(newCurObj)
		if err != nil {
			glog.Errorf("getting state from new current object (%s)", err)
			return err
		}
		// If the new current version of the object is the same as the old current
		// then proceed with trying to PUT the candidate to the core apiserver
		if newCurState.rev == curState.rev {
			ns, name, err := t.decodeKey(key)
			if err != nil {
				glog.Errorf("decoding key %s (%s)", key, err)
				return err
			}
			putReq := t.cl.Put().AbsPath(
				"apis",
				groupName,
				tprVersion,
				"namespaces",
				ns,
				t.singularKind.URLName(),
				name,
			).Body(candidateData)
			putRes := putReq.Do()
			if putRes.Error() != nil {
				glog.Errorf("executing PUT to %s/%s (%s)", ns, name, putRes.Error())
				return err
			}
			var statusCode int
			putRes.StatusCode(&statusCode)
			if statusCode != http.StatusOK {
				return fmt.Errorf(
					"executing PUT for %s/%s, received response code %d",
					ns,
					name,
					statusCode,
				)
			}
			var putUnknown runtime.Unknown
			if err := putRes.Into(&putUnknown); err != nil {
				glog.Errorf("reading response (%s)", err)
				return err
			}
			if err := decode(t.codec, putUnknown.Raw, out); err != nil {
				glog.Errorf("decoding response (%s)", err)
				return err
			}
		} else {
			glog.V(4).Infof(
				"GuaranteedUpdate of %s failed because of a conflict, going to retry",
				key,
			)
			curState = newCurState
			continue
		}
		return nil
	}
}

func decode(
	codec runtime.Codec,
	value []byte,
	objPtr runtime.Object,
) error {
	if _, err := conversion.EnforcePtr(objPtr); err != nil {
		panic("unable to convert output object to pointer")
	}
	_, _, err := codec.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	return nil
}

func removeNamespace(obj runtime.Object) error {
	if err := accessor.SetNamespace(obj, ""); err != nil {
		glog.Errorf("removing namespace from %#v (%s)", obj, err)
		return err
	}
	return nil
}

func checkPreconditions(
	key string,
	preconditions *storage.Preconditions,
	out runtime.Object,
) error {
	if preconditions == nil {
		return nil
	}
	objMeta, err := v1.ObjectMetaFor(out)
	if err != nil {
		return storage.NewInternalErrorf(
			"can't enforce preconditions %v on un-introspectable object %v, got error: %v",
			*preconditions, out, err,
		)
	}
	if preconditions.UID != nil && *preconditions.UID != objMeta.UID {
		errMsg := fmt.Sprintf(
			"Precondition failed: UID in precondition: %v, UID in object meta: %v",
			*preconditions.UID, objMeta.UID,
		)
		return storage.NewInvalidObjError(key, errMsg)
	}
	return nil
}
