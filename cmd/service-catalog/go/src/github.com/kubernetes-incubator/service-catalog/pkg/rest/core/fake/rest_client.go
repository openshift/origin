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

package fake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	fakerestclient "k8s.io/client-go/rest/fake"
)

var (
	accessor = meta.NewAccessor()
)

// ObjStorage is a map of object names to objects
type ObjStorage map[string]runtime.Object

// TypedStorage is a map of types to ObjStorage
type TypedStorage map[string]ObjStorage

// NamespacedStorage is a map of namespaces to TypedStorage
type NamespacedStorage map[string]TypedStorage

// NewTypedStorage returns a new TypedStorage
func NewTypedStorage() TypedStorage { return map[string]ObjStorage{} }

// Set adds an object to storage, given a namespace, type, and name
func (s NamespacedStorage) Set(ns, tipe, name string, obj runtime.Object) {
	if _, ok := s[ns]; !ok {
		s[ns] = make(TypedStorage)
	}
	if _, ok := s[ns][tipe]; !ok {
		s[ns][tipe] = make(ObjStorage)
	}
	s[ns][tipe][name] = obj
}

// GetList returns a list of objects from storage, given a namespace and a type
func (s NamespacedStorage) GetList(ns, tipe string) []runtime.Object {
	itemMap, ok := s[ns][tipe]
	if !ok {
		return []runtime.Object{}
	}
	items := make([]runtime.Object, 0, len(itemMap))
	for _, item := range itemMap {
		items = append(items, item)
	}
	return items
}

// Get returns an object from storage, given a namespace, type, and name
func (s NamespacedStorage) Get(ns, tipe, name string) runtime.Object {
	item, ok := s[ns][tipe][name]
	if !ok {
		return nil
	}
	return item
}

// Delete removes an object from storage, given a namepace, type, and name
func (s NamespacedStorage) Delete(ns, tipe, name string) {
	delete(s[ns][tipe], name)
}

// RESTClient is a fake implementation of rest.Interface used to facilitate
// testing. It short-circuits all HTTP requests that would ordinarily go
// upstream to a core apiserver. It muxes those requests in-process, uses
// in-memory storage, and responds just as a core apiserver would.
type RESTClient struct {
	Storage  NamespacedStorage
	Watcher  *Watcher
	accessor meta.MetadataAccessor
	*fakerestclient.RESTClient
}

// NewRESTClient returns a new FakeCoreRESTClient
func NewRESTClient(newEmptyObj func() runtime.Object) *RESTClient {
	storage := make(NamespacedStorage)
	watcher := NewWatcher()

	coreCl := &fakerestclient.RESTClient{
		Client: fakerestclient.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			r := getRouter(storage, watcher, newEmptyObj)
			rw := newResponseWriter()
			r.ServeHTTP(rw, request)
			return rw.getResponse(), nil
		}),
		NegotiatedSerializer: serializer.DirectCodecFactory{
			CodecFactory: Codecs,
		},
		APIRegistry: Registry,
	}
	return &RESTClient{
		Storage:    storage,
		Watcher:    watcher,
		accessor:   meta.NewAccessor(),
		RESTClient: coreCl,
	}
}

type responseWriter struct {
	header    http.Header
	headerSet bool
	body      []byte
}

func newResponseWriter() *responseWriter {
	return &responseWriter{
		header: make(http.Header),
	}
}

func (rw *responseWriter) Header() http.Header {
	return rw.header
}

func (rw *responseWriter) Write(bytes []byte) (int, error) {
	if !rw.headerSet {
		rw.WriteHeader(http.StatusOK)
	}
	rw.body = append(rw.body, bytes...)
	return len(bytes), nil
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.headerSet = true
	rw.header.Set("status", strconv.Itoa(status))
}

func (rw *responseWriter) getResponse() *http.Response {
	status, err := strconv.ParseInt(rw.header.Get("status"), 10, 16)
	if err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: int(status),
		Header:     rw.header,
		Body:       ioutil.NopCloser(bytes.NewBuffer(rw.body)),
	}
}

func getRouter(
	storage NamespacedStorage,
	watcher *Watcher,
	newEmptyObj func() runtime.Object,
) http.Handler {
	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/namespaces/{namespace}/{type}",
		getItems(storage),
	).Methods("GET")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/namespaces/{namespace}/{type}",
		createItem(storage, newEmptyObj),
	).Methods("POST")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/namespaces/{namespace}/{type}/{name}",
		getItem(storage),
	).Methods("GET")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/namespaces/{namespace}/{type}/{name}",
		updateItem(storage, newEmptyObj),
	).Methods("PUT")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/namespaces/{namespace}/{type}/{name}",
		deleteItem(storage),
	).Methods("DELETE")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/watch/namespaces/{namespace}/{type}/{name}",
		watchItem(watcher),
	).Methods("GET")
	r.HandleFunc(
		"/apis/servicecatalog.k8s.io/v1alpha1/watch/namespaces/{namespace}/{type}",
		watchList(watcher),
	).Methods("GET")
	r.HandleFunc(
		"/api/v1/namespaces",
		listNamespaces(storage),
	).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	return r
}

func watchItem(watcher *Watcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ch := watcher.ReceiveChan()
		doWatch(ch, w)
	}
}

func watchList(watcher *Watcher) func(http.ResponseWriter, *http.Request) {
	const timeout = 1 * time.Second
	return func(w http.ResponseWriter, r *http.Request) {
		ch := watcher.ReceiveChan()
		doWatch(ch, w)
	}
}

func getItems(storage NamespacedStorage) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		ns := mux.Vars(r)["namespace"]
		tipe := mux.Vars(r)["type"]
		objs := storage.GetList(ns, tipe)
		items := make([]runtime.Object, 0, len(objs))
		for _, obj := range objs {
			// We need to strip away typemeta, but we don't want to tamper with what's
			// in memory, so we're going to make a deep copy first.
			objCopy, err := conversion.NewCloner().DeepCopy(obj)
			if err != nil {
				errStr := fmt.Sprintf("error performing deep copy: %s", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			item, ok := objCopy.(runtime.Object)
			if !ok {
				errStr := fmt.Sprintf("error performing type assertion: %s", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			items = append(items, item)
		}
		var list runtime.Object
		var codec runtime.Codec
		var err error
		switch tipe {
		case "servicebrokers":
			list = &sc.ServiceBrokerList{TypeMeta: newTypeMeta("broker-list")}
			if err := meta.SetList(list, items); err != nil {
				errStr := fmt.Sprintf("Error setting list items (%s)", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			codec, err = testapi.GetCodecForObject(&sc.ServiceBrokerList{})
		case "serviceclasses":
			list = &sc.ServiceClassList{TypeMeta: newTypeMeta("service-class-list")}
			if err := meta.SetList(list, items); err != nil {
				errStr := fmt.Sprintf("Error setting list items (%s)", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			codec, err = testapi.GetCodecForObject(&sc.ServiceClassList{})
		case "serviceinstances":
			list = &sc.ServiceInstanceList{TypeMeta: newTypeMeta("instance-list")}
			if err := meta.SetList(list, items); err != nil {
				errStr := fmt.Sprintf("Error setting list items (%s)", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			codec, err = testapi.GetCodecForObject(&sc.ServiceInstanceList{})
		case "serviceinstancecredentials":
			list = &sc.ServiceInstanceCredentialList{TypeMeta: newTypeMeta("binding-list")}
			if err := meta.SetList(list, items); err != nil {
				errStr := fmt.Sprintf("Error setting list items (%s)", err)
				http.Error(rw, errStr, http.StatusInternalServerError)
				return
			}
			codec, err = testapi.GetCodecForObject(&sc.ServiceInstanceCredentialList{})
		default:
			errStr := fmt.Sprintf("unrecognized resource type: %s", tipe)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		if err != nil {
			errStr := fmt.Sprintf("error getting codec: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		listBytes, err := runtime.Encode(codec, list)
		if err != nil {
			errStr := fmt.Sprintf("error encoding list: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		rw.Write(listBytes)
	}
}

func createItem(storage NamespacedStorage, newEmptyObj func() runtime.Object) func(rw http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		ns := mux.Vars(r)["namespace"]
		tipe := mux.Vars(r)["type"]
		codec, err := testapi.GetCodecForObject(newEmptyObj())
		if err != nil {
			errStr := fmt.Sprintf("error getting a codec for %#v (%s)", newEmptyObj(), err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errStr := fmt.Sprintf("error getting body bytes: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		item, err := runtime.Decode(codec, bodyBytes)
		if err != nil {
			errStr := fmt.Sprintf("error decoding body bytes: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		name, err := accessor.Name(item)
		if err != nil {
			errStr := fmt.Sprintf("couldn't get object name: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		if storage.Get(ns, tipe, name) != nil {
			rw.WriteHeader(http.StatusConflict)
			return
		}
		accessor.SetResourceVersion(item, "1")
		storage.Set(ns, tipe, name, item)
		setContentType(rw)
		rw.WriteHeader(http.StatusCreated)
		bytes, err := runtime.Encode(codec, item)
		if err != nil {
			errStr := fmt.Sprintf("error encoding item (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		rw.Write(bytes)
	}
}

func getItem(storage NamespacedStorage) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		ns := mux.Vars(r)["namespace"]
		tipe := mux.Vars(r)["type"]
		name := mux.Vars(r)["name"]
		item := storage.Get(ns, tipe, name)
		if item == nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		codec, err := testapi.GetCodecForObject(item)
		if err != nil {
			errStr := fmt.Sprintf("error getting codec (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		bytes, err := runtime.Encode(codec, item)
		if err != nil {
			errStr := fmt.Sprintf("error encoding item (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		setContentType(rw)
		rw.Write(bytes)
	}
}

func updateItem(storage NamespacedStorage, newEmptyObj func() runtime.Object) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		ns := mux.Vars(r)["namespace"]
		tipe := mux.Vars(r)["type"]
		name := mux.Vars(r)["name"]
		origItem := storage.Get(ns, tipe, name)
		if origItem == nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		codec, err := testapi.GetCodecForObject(newEmptyObj())
		if err != nil {
			errStr := fmt.Sprintf("error getting codec: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errStr := fmt.Sprintf("error getting body bytes: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		item, err := runtime.Decode(codec, bodyBytes)
		if err != nil {
			errStr := fmt.Sprintf("error decoding body bytes: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		origResourceVersionStr, err := accessor.ResourceVersion(item)
		if err != nil {
			errStr := fmt.Sprintf("error getting resource version")
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		resourceVersionStr, err := accessor.ResourceVersion(item)
		if err != nil {
			errStr := fmt.Sprintf("error getting resource version")
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		// As with the actual core apiserver, "0" is a special resource version that
		// forces an update as if the current / most up-to-date resource version had
		// been passed in.
		if resourceVersionStr != "0" && resourceVersionStr != origResourceVersionStr {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		resourceVersion, err := strconv.Atoi(origResourceVersionStr)
		resourceVersion++
		accessor.SetResourceVersion(item, strconv.Itoa(resourceVersion))

		// if the deletion timestamp is set and there are 0 finalizers, then we should delete
		finalizers, err := scmeta.GetFinalizers(item)
		if err != nil {
			errStr := fmt.Sprintf("error getting finalizers (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		oldDT, err := scmeta.GetDeletionTimestamp(origItem)
		if err != nil && err != scmeta.ErrNoDeletionTimestamp {
			errStr := fmt.Sprintf("error getting deletion timestamp on existing obj (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		newDT, err := scmeta.GetDeletionTimestamp(item)
		if err != nil && err != scmeta.ErrNoDeletionTimestamp {
			errStr := fmt.Sprintf("error getting deletion timestamp on new obj (%s)", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		if newDT != nil && oldDT != nil {
			// only measure the deletion timestamp based on seconds (.Unix()) rather than nanos
			// (.UnixNano()), because nanoseconds are stored in memory but do not come over
			// the wire. Thus, the new deletion timestamp will not be equal to the old
			// because the new will have nanos missing
			if newDT.Unix() != oldDT.Unix() {
				errStr := fmt.Sprintf(
					"you cannot update the deletion timestamp (old: %#v, new: %#v)",
					oldDT.String(),
					newDT.String(),
				)
				http.Error(rw, errStr, http.StatusBadRequest)
				return
			}
		}

		if len(finalizers) == 0 && newDT != nil {
			// if there are no finalizers and the deletion timestamp is set, delete
			storage.Delete(ns, tipe, name)
		} else {
			// otherwise, just update as normal
			storage.Set(ns, tipe, name, item)
		}
		bytes, err := runtime.Encode(codec, item)
		if err != nil {
			errStr := fmt.Sprintf("error encoding item: %s", err)
			http.Error(rw, errStr, http.StatusInternalServerError)
			return
		}
		setContentType(rw)
		rw.Write(bytes)
	}
}

func deleteItem(storage NamespacedStorage) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		ns := mux.Vars(r)["namespace"]
		tipe := mux.Vars(r)["type"]
		name := mux.Vars(r)["name"]
		item := storage.Get(ns, tipe, name)
		if item == nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		finalizers, err := scmeta.GetFinalizers(item)
		if err != nil {
			http.Error(
				rw,
				fmt.Sprintf("error getting finalizers (%s)", err),
				http.StatusInternalServerError,
			)
			return
		}
		if len(finalizers) == 0 {
			// delete if there are no finalizers
			storage.Delete(ns, tipe, name)
		} else {
			// set a deletion timestamp on the item if there are finalizers
			if err := scmeta.SetDeletionTimestamp(item, time.Now()); err != nil {
				http.Error(
					rw,
					fmt.Sprintf("error setting deletion timestamp (%s)", err),
					http.StatusInternalServerError,
				)
				return
			}
			storage.Set(ns, tipe, name, item)
		}
		rw.WriteHeader(http.StatusOK)
	}
}

func listNamespaces(storage NamespacedStorage) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		nsList := apiv1.NamespaceList{}
		for ns := range storage {
			nsList.Items = append(nsList.Items, apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})
		}
		setContentType(rw)
		if err := json.NewEncoder(rw).Encode(&nsList); err != nil {
			log.Printf("Error encoding namespace list (%s)", err)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func notFoundHandler(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusNotFound)
}

func newTypeMeta(kind string) metav1.TypeMeta {
	return metav1.TypeMeta{Kind: kind, APIVersion: sc.GroupName + "/v1alpha1'"}
}
