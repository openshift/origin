// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"context"
	"sync"

	"github.com/golang/glog"
)

// RunMigration migrates data from a number of CT logs to Trillian. Each log's
// migration is coordinated by the corresponding Controller. This function
// terminates when all Controllers are done (possibly with an erorr, or as a
// result of canceling the passed in context).
//
// TODO(pavelkalinnikov):
// - Surface status of each goroutine in monitoring.
// - Handle Controller failures, e.g. restart them (depends on error type).
// - Introduce a MultiController type.
// - Start Controllers with random delays to prevent one instance (e.g. the one
//   that has started first) capturing mastership over all logs at once. This
//   is particularly important for mirroring.
// - Add voluntary mastership resignations.
func RunMigration(ctx context.Context, ctrls []*Controller) {
	var wg sync.WaitGroup
	for _, ctrl := range ctrls {
		ctrl := ctrl
		uri := ctrl.ctClient.BaseURI()
		treeID := ctrl.plClient.tree.TreeId

		wg.Add(1)
		go func() {
			defer wg.Done()
			glog.Infof("Starting migration Controller (%d<-%q)", treeID, uri)
			if err := ctrl.RunWhenMaster(ctx); err != nil {
				glog.Errorf("Controller.RunWhenMaster(%d<-%q): %v", treeID, uri, err)
				return
			}
			glog.Infof("Controller stopped (%d<-%q)", treeID, uri)
		}()
	}

	wg.Wait()
}
