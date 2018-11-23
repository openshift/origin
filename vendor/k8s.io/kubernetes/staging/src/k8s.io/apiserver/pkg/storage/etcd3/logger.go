/*
Copyright 2014 The Kubernetes Authors.

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

package etcd3

import (
	"fmt"

	"github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
)

func init() {
	clientv3.SetLogger(glogWrapper{})
}

type glogWrapper struct{}

const glogWrapperDepth = 4

func (glogWrapper) Info(args ...interface{}) {
	glog.InfoDepth(glogWrapperDepth, args...)
}

func (glogWrapper) Infoln(args ...interface{}) {
	glog.InfoDepth(glogWrapperDepth, fmt.Sprintln(args...))
}

func (glogWrapper) Infof(format string, args ...interface{}) {
	glog.InfoDepth(glogWrapperDepth, fmt.Sprintf(format, args...))
}

func (glogWrapper) Warning(args ...interface{}) {
	glog.WarningDepth(glogWrapperDepth, args...)
}

func (glogWrapper) Warningln(args ...interface{}) {
	glog.WarningDepth(glogWrapperDepth, fmt.Sprintln(args...))
}

func (glogWrapper) Warningf(format string, args ...interface{}) {
	glog.WarningDepth(glogWrapperDepth, fmt.Sprintf(format, args...))
}

func (glogWrapper) Error(args ...interface{}) {
	glog.ErrorDepth(glogWrapperDepth, args...)
}

func (glogWrapper) Errorln(args ...interface{}) {
	glog.ErrorDepth(glogWrapperDepth, fmt.Sprintln(args...))
}

func (glogWrapper) Errorf(format string, args ...interface{}) {
	glog.ErrorDepth(glogWrapperDepth, fmt.Sprintf(format, args...))
}

func (glogWrapper) Fatal(args ...interface{}) {
	glog.FatalDepth(glogWrapperDepth, args...)
}

func (glogWrapper) Fatalln(args ...interface{}) {
	glog.FatalDepth(glogWrapperDepth, fmt.Sprintln(args...))
}

func (glogWrapper) Fatalf(format string, args ...interface{}) {
	glog.FatalDepth(glogWrapperDepth, fmt.Sprintf(format, args...))
}

func (glogWrapper) V(l int) bool {
	return bool(glog.V(glog.Level(l)))
}
