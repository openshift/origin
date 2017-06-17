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
	restclient "k8s.io/client-go/rest"
)

func put(
	cl restclient.Interface,
	codec runtime.Codec,
	kind Kind,
	ns,
	name string,
	data []byte,
	out runtime.Object,
) error {
	putReq := cl.Put().AbsPath(
		"apis",
		groupName,
		tprVersion,
		"namespaces",
		ns,
		kind.URLName(),
		name,
	).Body(data)
	putRes := putReq.Do()
	if putRes.Error() != nil {
		glog.Errorf("executing PUT to %s/%s (%s)", ns, name, putRes.Error())
		return putRes.Error()
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
	if err := decode(codec, putUnknown.Raw, out); err != nil {
		glog.Errorf("decoding response (%s)", err)
		return err
	}
	return nil
}
