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
	"net/http"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
	restclient "k8s.io/client-go/rest"
)

func get(
	cl restclient.Interface,
	codec runtime.Codec,
	kind Kind,
	key,
	ns,
	name string,
	out runtime.Object,
	hasNamespace,
	ignoreNotFound bool,
) error {
	req := cl.Get().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		kind.URLName(),
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
			return runtime.SetZeroValue(out)
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
	if err := res.Into(&unknown); err != nil {
		glog.Errorf("decoding response (%s)", err)
		return err
	}

	if err := decode(codec, unknown.Raw, out); err != nil {
		return nil
	}
	if !hasNamespace {
		if err := removeNamespace(out); err != nil {
			glog.Errorf("removing namespace from %#v (%s)", out, err)
			return err
		}
	}
	return nil
}
