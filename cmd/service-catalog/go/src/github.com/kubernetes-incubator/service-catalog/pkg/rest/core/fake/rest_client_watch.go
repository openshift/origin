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
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	pkgwatch "k8s.io/apimachinery/pkg/watch"
)

func doWatch(ch <-chan pkgwatch.Event, w http.ResponseWriter) {
	for evt := range ch {
		codec, err := testapi.GetCodecForObject(evt.Object)
		if err != nil {
			errStr := fmt.Sprintf("error getting codec (%s)", err)
			log.Fatal(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		objBytes, err := runtime.Encode(codec, evt.Object)
		if err != nil {
			errStr := fmt.Sprintf("error encoding item (%s)", err)
			log.Fatal(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}

		evt := metav1.WatchEvent{
			Type: fmt.Sprintf("%s", evt.Type),
			Object: runtime.RawExtension{
				Object: evt.Object,
				Raw:    objBytes,
			},
		}
		b, err := json.Marshal(&evt)
		if err != nil {
			errStr := fmt.Sprintf("error encoding JSON (%s)", err)
			log.Fatal(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json, */*")
		w.Write(b)
	}
}
