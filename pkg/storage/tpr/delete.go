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
	"k8s.io/apiserver/pkg/storage"
	restclient "k8s.io/client-go/rest"
)

func delete(cl restclient.Interface, kind Kind, key, ns, name string, expectedCode int) error {
	req := cl.Delete().AbsPath(
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
		glog.Errorf("executing DELETE for %s/%s (%s)", ns, name, res.Error())
	}
	var statusCode int
	res.StatusCode(&statusCode)
	if statusCode == http.StatusNotFound {
		return storage.NewKeyNotFoundError(key, 0)
	}
	if statusCode != expectedCode {
		return fmt.Errorf(
			"executing DELETE for %s/%s, received response code %d",
			ns,
			name,
			statusCode,
		)
	}
	return nil
}
