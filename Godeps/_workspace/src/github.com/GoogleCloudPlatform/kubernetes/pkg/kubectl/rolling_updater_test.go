/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package kubectl

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/testapi"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

func oldRc(replicas int) *api.ReplicationController {
	return &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: "foo-v1",
			UID:  "7764ae47-9092-11e4-8393-42010af018ff",
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: replicas,
			Selector: map[string]string{"version": "v1"},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Name:   "foo-v1",
					Labels: map[string]string{"version": "v1"},
				},
			},
		},
		Status: api.ReplicationControllerStatus{
			Replicas: replicas,
		},
	}
}

func newRc(replicas int, desired int) *api.ReplicationController {
	rc := oldRc(replicas)
	rc.Spec.Template = &api.PodTemplateSpec{
		ObjectMeta: api.ObjectMeta{
			Name:   "foo-v2",
			Labels: map[string]string{"version": "v2"},
		},
	}
	rc.Spec.Selector = map[string]string{"version": "v2"}
	rc.ObjectMeta = api.ObjectMeta{
		Name: "foo-v2",
		Annotations: map[string]string{
			desiredReplicasAnnotation: fmt.Sprintf("%d", desired),
			sourceIdAnnotation:        "foo-v1:7764ae47-9092-11e4-8393-42010af018ff",
		},
	}
	return rc
}

// TestUpdate performs complex scenario testing for rolling updates. It
// provides fine grained control over the states for each update interval to
// allow the expression of as many edge cases as possible.
func TestUpdate(t *testing.T) {
	Percent := func(p int) *int {
		return &p
	}
	var NilPercent *int

	// up represents a simulated scale up event and expectation
	type up struct {
		// to is the expected replica count for a scale-up
		to int
	}
	// down represents a simulated scale down event and expectation
	type down struct {
		// oldReady is the number of oldRc replicas which will be seen
		// as ready during the scale down attempt
		oldReady int
		// newReady is the number of newRc replicas which will be seen
		// as ready during the scale up attempt
		newReady int
		// to is the expected replica count for the scale down
		to int
		// noop and to are mutually exclusive; if noop is true, that means for
		// this down event, no scaling attempt should be made (for example, if
		// by scaling down, the readiness minimum would be crossed.)
		noop bool
	}

	tests := []struct {
		name string
		// oldRc is the "from" deployment
		oldRc *api.ReplicationController
		// newRc is the "to" deployment
		newRc *api.ReplicationController
		// whether newRc existed (false means it was created)
		newRcExists bool
		// updatePercent is the % for the update config
		updatePercent *int
		// expected is the sequence of up/down events that will be simulated and
		// verified
		expected []interface{}
		// output is the expected textual output written
		output string
	}{
		{
			name:          "1/1/nil optimistic readiness",
			oldRc:         oldRc(1),
			newRc:         newRc(0, 1),
			newRcExists:   false,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{1},
				down{oldReady: 1, newReady: 1, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 1, scaling down foo-v1 from 1 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 0
`,
		}, {
			name:          "2/2/nil optimistic readiness",
			oldRc:         oldRc(2),
			newRc:         newRc(0, 2),
			newRcExists:   false,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{1},
				down{oldReady: 2, newReady: 0, to: 1},
				up{2},
				down{oldReady: 1, newReady: 1, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 2, scaling down foo-v1 from 2 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 1
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
`,
		}, {
			name:          "4/4/nil optimistic readiness, continuation",
			oldRc:         oldRc(3),
			newRc:         newRc(1, 4),
			newRcExists:   true,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{2},
				down{oldReady: 3, newReady: 1, to: 2},
				up{3},
				down{oldReady: 2, newReady: 3, to: 1},
				up{4},
				down{oldReady: 1, newReady: 3, to: 0},
			},
			output: `Continuing update with existing controller foo-v2.
Scaling up foo-v2 from 1 to 4, scaling down foo-v1 from 3 to 0 (scale up first by 1 each interval, maintain at least 2 ready)
Scaling foo-v2 up to 2
Scaling foo-v1 down to 2
Scaling foo-v2 up to 3
Scaling foo-v1 down to 1
Scaling foo-v2 up to 4
Scaling foo-v1 down to 0
`,
		}, {
			name:          "2/2/nil delayed readiness",
			oldRc:         oldRc(2),
			newRc:         newRc(0, 2),
			newRcExists:   false,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{1},
				down{oldReady: 2, newReady: 0, to: 1},
				up{2},
				// this scale-down will be a no-op because it would violate the minimum
				down{oldReady: 1, newReady: 1, noop: true},
				// this one will succeed because there are enough ready
				down{oldReady: 1, newReady: 2, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 2, scaling down foo-v1 from 2 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 1
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
`,
		}, {
			name:          "2/7/nil optimistic readiness",
			oldRc:         oldRc(2),
			newRc:         newRc(0, 7),
			newRcExists:   false,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{1},
				down{oldReady: 2, newReady: 0, to: 1},
				up{2},
				down{oldReady: 1, newReady: 2, to: 0},
				up{7},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 7, scaling down foo-v1 from 2 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 1
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
Scaling foo-v2 up to 7
`,
		}, {
			name:          "7/2/nil optimistic readiness",
			oldRc:         oldRc(7),
			newRc:         newRc(0, 2),
			newRcExists:   false,
			updatePercent: NilPercent,
			expected: []interface{}{
				up{1},
				down{oldReady: 7, newReady: 0, to: 6},
				up{2},
				down{oldReady: 6, newReady: 2, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 2, scaling down foo-v1 from 7 to 0 (scale up first by 1 each interval, maintain at least 6 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 6
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
`,
		}, {
			name:          "10/10/20 optimistic readiness",
			oldRc:         oldRc(10),
			newRc:         newRc(0, 10),
			newRcExists:   false,
			updatePercent: Percent(20),
			expected: []interface{}{
				up{2},
				down{oldReady: 10, newReady: 1, to: 8},
				up{4},
				down{oldReady: 8, newReady: 3, to: 6},
				up{6},
				down{oldReady: 6, newReady: 5, to: 4},
				up{8},
				down{oldReady: 4, newReady: 7, to: 2},
				up{10},
				down{oldReady: 2, newReady: 9, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 10, scaling down foo-v1 from 10 to 0 (scale up first by 2 each interval, maintain at least 8 ready)
Scaling foo-v2 up to 2
Scaling foo-v1 down to 8
Scaling foo-v2 up to 4
Scaling foo-v1 down to 6
Scaling foo-v2 up to 6
Scaling foo-v1 down to 4
Scaling foo-v2 up to 8
Scaling foo-v1 down to 2
Scaling foo-v2 up to 10
Scaling foo-v1 down to 0
`,
		}, {
			name:          "2/6/50 optimistic readiness",
			oldRc:         oldRc(2),
			newRc:         newRc(0, 6),
			newRcExists:   false,
			updatePercent: Percent(50),
			expected: []interface{}{
				up{3},
				down{oldReady: 2, newReady: 1, to: 0},
				up{6},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 6, scaling down foo-v1 from 2 to 0 (scale up first by 3 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 3
Scaling foo-v1 down to 0
Scaling foo-v2 up to 6
`,
		}, {name: "10/3/50 optimistic readiness",
			oldRc:         oldRc(10),
			newRc:         newRc(0, 3),
			newRcExists:   false,
			updatePercent: Percent(50),
			expected: []interface{}{
				up{2},
				down{oldReady: 10, newReady: 1, to: 8},
				up{3},
				down{oldReady: 8, newReady: 3, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 3, scaling down foo-v1 from 10 to 0 (scale up first by 2 each interval, maintain at least 5 ready)
Scaling foo-v2 up to 2
Scaling foo-v1 down to 8
Scaling foo-v2 up to 3
Scaling foo-v1 down to 0
`,
		}, {
			name:          "4/4/-50 optimistic readiness",
			oldRc:         oldRc(4),
			newRc:         newRc(0, 4),
			newRcExists:   false,
			updatePercent: Percent(-50),
			expected: []interface{}{
				down{oldReady: 4, newReady: 0, to: 2},
				up{2},
				down{oldReady: 2, newReady: 2, to: 0},
				up{4},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 4, scaling down foo-v1 from 4 to 0 (scale down first by 2 each interval, maintain at least 2 ready)
Scaling foo-v1 down to 2
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
Scaling foo-v2 up to 4
`,
		}, {
			name:          "2/4/-50 delayed readiness",
			oldRc:         oldRc(2),
			newRc:         newRc(0, 4),
			newRcExists:   false,
			updatePercent: Percent(-50),
			expected: []interface{}{
				down{oldReady: 2, newReady: 0, to: 1},
				up{1},
				// can't scale down yet, not enough ready
				down{oldReady: 1, newReady: 0, noop: true},
				// new one is ready, scale down
				down{oldReady: 1, newReady: 1, to: 0},
				up{4},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 4, scaling down foo-v1 from 2 to 0 (scale down first by 2 each interval, maintain at least 1 ready)
Scaling foo-v1 down to 1
Scaling foo-v2 up to 1
Scaling foo-v1 down to 0
Scaling foo-v2 up to 4
`,
		}, {
			name:          "4/2/-50 optimistic readiness",
			oldRc:         oldRc(4),
			newRc:         newRc(0, 2),
			newRcExists:   false,
			updatePercent: Percent(-50),
			expected: []interface{}{
				down{oldReady: 4, newReady: 0, to: 3},
				up{1},
				down{oldReady: 3, newReady: 0, to: 2},
				up{2},
				down{oldReady: 2, newReady: 2, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 2, scaling down foo-v1 from 4 to 0 (scale down first by 1 each interval, maintain at least 2 ready)
Scaling foo-v1 down to 3
Scaling foo-v2 up to 1
Scaling foo-v1 down to 2
Scaling foo-v2 up to 2
Scaling foo-v1 down to 0
`,
		}, {
			name:          "4/4/-100 optimistic readiness",
			oldRc:         oldRc(4),
			newRc:         newRc(0, 4),
			newRcExists:   false,
			updatePercent: Percent(-100),
			expected: []interface{}{
				down{oldReady: 4, newReady: 0, to: 0},
				up{4},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 4, scaling down foo-v1 from 4 to 0 (scale down first by 4 each interval, maintain at least 0 ready)
Scaling foo-v1 down to 0
Scaling foo-v2 up to 4
`,
		}, {
			name:          "1/1/-50 optimistic readiness",
			oldRc:         oldRc(1),
			newRc:         newRc(0, 1),
			newRcExists:   false,
			updatePercent: Percent(-50),
			expected: []interface{}{
				down{oldReady: 1, newReady: 0, to: 0},
				up{1},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 1, scaling down foo-v1 from 1 to 0 (scale down first by 1 each interval, maintain at least 0 ready)
Scaling foo-v1 down to 0
Scaling foo-v2 up to 1
`,
		}, {
			name:          "1/1/-50 optimistic readiness",
			oldRc:         oldRc(1),
			newRc:         newRc(0, 1),
			newRcExists:   false,
			updatePercent: Percent(-50),
			expected: []interface{}{
				down{oldReady: 1, newReady: 0, to: 0},
				up{1},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 1, scaling down foo-v1 from 1 to 0 (scale down first by 1 each interval, maintain at least 0 ready)
Scaling foo-v1 down to 0
Scaling foo-v2 up to 1
`,
		}, {
			name:          "1/1/100 optimistic readiness",
			oldRc:         oldRc(1),
			newRc:         newRc(0, 1),
			newRcExists:   false,
			updatePercent: Percent(100),
			expected: []interface{}{
				up{1},
				down{oldReady: 1, newReady: 1, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 1, scaling down foo-v1 from 1 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 0
`,
		}, {
			name:          "1/1/100 delayed readiness",
			oldRc:         oldRc(1),
			newRc:         newRc(0, 1),
			newRcExists:   false,
			updatePercent: Percent(100),
			expected: []interface{}{
				up{1},
				down{oldReady: 1, newReady: 0, noop: true},
				down{oldReady: 1, newReady: 1, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 1, scaling down foo-v1 from 1 to 0 (scale up first by 1 each interval, maintain at least 1 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 0
`,
		}, {
			name:          "4/4/-25 external downsize and recovery",
			oldRc:         oldRc(4),
			newRc:         newRc(0, 4),
			newRcExists:   false,
			updatePercent: Percent(-25),
			expected: []interface{}{
				// can't scale down because we're already under the minimum
				down{oldReady: 2, newReady: 0, noop: true},
				// old still recovering; can't scale down because doing so would bring
				// us under the minimum
				down{oldReady: 3, newReady: 0, noop: true},
				// old finally back, can now scale
				down{oldReady: 4, newReady: 0, to: 3},
				up{1},
				// new taking time to become ready, can't scale yet
				down{oldReady: 3, newReady: 0, noop: true},
				// new now ready
				down{oldReady: 3, newReady: 1, to: 2},
				up{2},
				down{oldReady: 2, newReady: 2, to: 1},
				up{3},
				// need to wait for another new to become ready
				down{oldReady: 1, newReady: 2, noop: true},
				down{oldReady: 1, newReady: 3, to: 0},
				up{4},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 4, scaling down foo-v1 from 4 to 0 (scale down first by 1 each interval, maintain at least 3 ready)
Scaling foo-v1 down to 3
Scaling foo-v2 up to 1
Scaling foo-v1 down to 2
Scaling foo-v2 up to 2
Scaling foo-v1 down to 1
Scaling foo-v2 up to 3
Scaling foo-v1 down to 0
Scaling foo-v2 up to 4
`,
		}, {
			name:          "4/4/25 external downsize and recovery",
			oldRc:         oldRc(4),
			newRc:         newRc(0, 4),
			newRcExists:   false,
			updatePercent: Percent(25),
			expected: []interface{}{
				up{1},
				// can't scale down because we're already under the minimum
				down{oldReady: 2, newReady: 0, noop: true},
				// old still recovering; but we have enough new to compensate
				down{oldReady: 3, newReady: 1, to: 3},
				up{2},
				down{oldReady: 3, newReady: 2, to: 2},
				up{3},
				down{oldReady: 2, newReady: 3, to: 1},
				up{4},
				// now the new ready satisfies the minimum
				down{oldReady: 1, newReady: 3, to: 0},
			},
			output: `Created foo-v2
Scaling up foo-v2 from 0 to 4, scaling down foo-v1 from 4 to 0 (scale up first by 1 each interval, maintain at least 3 ready)
Scaling foo-v2 up to 1
Scaling foo-v1 down to 3
Scaling foo-v2 up to 2
Scaling foo-v1 down to 2
Scaling foo-v2 up to 3
Scaling foo-v1 down to 1
Scaling foo-v2 up to 4
Scaling foo-v1 down to 0
`,
		},
	}

	for i, test := range tests {
		if i != 2 {
			continue
		}
		// Extract expectations into some makeshift FIFOs so they can be returned
		// in the correct order from the right places. This lets scale downs be
		// expressed a single event even though the data is used from multiple
		// interface calls.
		oldReady := []int{}
		newReady := []int{}
		upTo := []int{}
		downTo := []int{}
		for _, event := range test.expected {
			switch e := event.(type) {
			case down:
				oldReady = append(oldReady, e.oldReady)
				newReady = append(newReady, e.newReady)
				if !e.noop {
					downTo = append(downTo, e.to)
				}
			case up:
				upTo = append(upTo, e.to)
			}
		}

		// Make a way to get the next item from our FIFOs. Returns -1 if the array
		// is empty.
		next := func(s *[]int) int {
			slice := *s
			v := -1
			if len(slice) > 0 {
				v = slice[0]
				if len(slice) > 1 {
					*s = slice[1:]
				} else {
					*s = []int{}
				}
			}
			return v
		}
		t.Logf("running test %d (%s) (up: %v, down: %v, oldReady: %v, newReady: %v)", i, test.name, upTo, downTo, oldReady, newReady)
		updater := &RollingUpdater{
			ns: "default",
			scaleAndWait: func(rc *api.ReplicationController, retry *RetryParams, wait *RetryParams) (*api.ReplicationController, error) {
				// Return a scale up or scale down expectation depending on the rc,
				// and throw errors if there is no expectation expressed for this
				// call.
				expected := -1
				t.Logf("scaling %s -> %d", rc.Name, rc.Spec.Replicas)
				switch {
				case rc == test.newRc:
					expected = next(&upTo)
				case rc == test.oldRc:
					expected = next(&downTo)
				}
				if expected == -1 {
					t.Fatalf("unexpected scale of %s to %d", rc.Name, rc.Spec.Replicas)
				} else if e, a := expected, rc.Spec.Replicas; e != a {
					t.Fatalf("expected scale of %s to %d, got %d", rc.Name, e, a)
				}
				return rc, nil
			},
			getOrCreateTargetController: func(controller *api.ReplicationController, sourceId string) (*api.ReplicationController, bool, error) {
				// Simulate a create vs. update of an existing controller.
				return test.newRc, test.newRcExists, nil
			},
			waitForReadyPods: func(interval, timeout time.Duration, oldRc, newRc *api.ReplicationController) (int, int, int, error) {
				// Return simulated readiness, and throw an error if this call has no
				// expectations defined.
				oldReady := next(&oldReady)
				newReady := next(&newReady)
				if oldReady == -1 || newReady == -1 {
					t.Fatalf("unexpected waitForReadyPods call for oldRc %q, newRc %q", oldRc, newRc)
				}
				return oldReady, newReady, oldReady + newReady, nil
			},
			cleanup: func(oldRc, newRc *api.ReplicationController, config *RollingUpdaterConfig) error {
				// TODO: verify cleanup elsewhere.
				return nil
			},
		}
		var buffer bytes.Buffer
		config := &RollingUpdaterConfig{
			Out:           &buffer,
			OldRc:         test.oldRc,
			NewRc:         test.newRc,
			UpdatePeriod:  0,
			Interval:      time.Millisecond,
			Timeout:       time.Millisecond,
			CleanupPolicy: DeleteRollingUpdateCleanupPolicy,
			UpdatePercent: test.updatePercent,
		}
		err := updater.Update(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if buffer.String() != test.output {
			t.Errorf("Bad output. expected:\n%s\ngot:\n%s", test.output, buffer.String())
		}
	}
}

// TestRollingUpdater_cleanupWithClients ensures that the cleanup policy is
// correctly implemented.
func TestRollingUpdater_cleanupWithClients(t *testing.T) {
	rc := oldRc(2)
	rcExisting := newRc(1, 3)

	tests := []struct {
		name      string
		policy    RollingUpdaterCleanupPolicy
		responses []runtime.Object
		expected  []string
	}{
		{
			name:      "preserve",
			policy:    PreserveRollingUpdateCleanupPolicy,
			responses: []runtime.Object{rcExisting},
			expected: []string{
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"get-replicationController",
			},
		},
		{
			name:      "delete",
			policy:    DeleteRollingUpdateCleanupPolicy,
			responses: []runtime.Object{rcExisting},
			expected: []string{
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"get-replicationController",
				"delete-replicationController",
			},
		},
		{
			name:      "rename",
			policy:    RenameRollingUpdateCleanupPolicy,
			responses: []runtime.Object{rcExisting},
			expected: []string{
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"get-replicationController",
				"delete-replicationController",
				"create-replicationController",
				"delete-replicationController",
			},
		},
	}

	for _, test := range tests {
		fake := testclient.NewSimpleFake(test.responses...)
		updater := &RollingUpdater{
			ns: "default",
			c:  fake,
		}
		config := &RollingUpdaterConfig{
			Out:           ioutil.Discard,
			OldRc:         rc,
			NewRc:         rcExisting,
			UpdatePeriod:  0,
			Interval:      time.Millisecond,
			Timeout:       time.Millisecond,
			CleanupPolicy: test.policy,
		}
		err := updater.cleanupWithClients(rc, rcExisting, config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(fake.Actions) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.name, fake.Actions, test.expected)
		}
		for j, action := range fake.Actions {
			if e, a := test.expected[j], action.Action; e != a {
				t.Errorf("%s: unexpected action: expected %s, got %s", test.name, e, a)
			}
		}
	}
}

func TestFindSourceController(t *testing.T) {
	ctrl1 := api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: "foo",
			Annotations: map[string]string{
				sourceIdAnnotation: "bar:1234",
			},
		},
	}
	ctrl2 := api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: "bar",
			Annotations: map[string]string{
				sourceIdAnnotation: "foo:12345",
			},
		},
	}
	ctrl3 := api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				sourceIdAnnotation: "baz:45667",
			},
		},
	}
	tests := []struct {
		list               *api.ReplicationControllerList
		expectedController *api.ReplicationController
		err                error
		name               string
		expectError        bool
	}{
		{
			list:        &api.ReplicationControllerList{},
			expectError: true,
		},
		{
			list: &api.ReplicationControllerList{
				Items: []api.ReplicationController{ctrl1},
			},
			name:        "foo",
			expectError: true,
		},
		{
			list: &api.ReplicationControllerList{
				Items: []api.ReplicationController{ctrl1},
			},
			name:               "bar",
			expectedController: &ctrl1,
		},
		{
			list: &api.ReplicationControllerList{
				Items: []api.ReplicationController{ctrl1, ctrl2},
			},
			name:               "bar",
			expectedController: &ctrl1,
		},
		{
			list: &api.ReplicationControllerList{
				Items: []api.ReplicationController{ctrl1, ctrl2},
			},
			name:               "foo",
			expectedController: &ctrl2,
		},
		{
			list: &api.ReplicationControllerList{
				Items: []api.ReplicationController{ctrl1, ctrl2, ctrl3},
			},
			name:               "baz",
			expectedController: &ctrl3,
		},
	}
	for _, test := range tests {
		fakeClient := testclient.NewSimpleFake(test.list)
		ctrl, err := FindSourceController(fakeClient, "default", test.name)
		if test.expectError && err == nil {
			t.Errorf("unexpected non-error")
		}
		if !test.expectError && err != nil {
			t.Errorf("unexpected error")
		}
		if !reflect.DeepEqual(ctrl, test.expectedController) {
			t.Errorf("expected:\n%v\ngot:\n%v\n", test.expectedController, ctrl)
		}
	}
}

func TestUpdateExistingReplicationController(t *testing.T) {
	tests := []struct {
		rc              *api.ReplicationController
		name            string
		deploymentKey   string
		deploymentValue string

		expectedRc *api.ReplicationController
		expectErr  bool
	}{
		{
			rc: &api.ReplicationController{
				Spec: api.ReplicationControllerSpec{
					Template: &api.PodTemplateSpec{},
				},
			},
			name:            "foo",
			deploymentKey:   "dk",
			deploymentValue: "some-hash",

			expectedRc: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/next-controller-id": "foo",
					},
				},
				Spec: api.ReplicationControllerSpec{
					Selector: map[string]string{
						"dk": "some-hash",
					},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{
								"dk": "some-hash",
							},
						},
					},
				},
			},
		},
		{
			rc: &api.ReplicationController{
				Spec: api.ReplicationControllerSpec{
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{
								"dk": "some-other-hash",
							},
						},
					},
					Selector: map[string]string{
						"dk": "some-other-hash",
					},
				},
			},
			name:            "foo",
			deploymentKey:   "dk",
			deploymentValue: "some-hash",

			expectedRc: &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/next-controller-id": "foo",
					},
				},
				Spec: api.ReplicationControllerSpec{
					Selector: map[string]string{
						"dk": "some-other-hash",
					},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{
								"dk": "some-other-hash",
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		buffer := &bytes.Buffer{}
		fakeClient := testclient.NewSimpleFake(test.expectedRc)
		rc, err := UpdateExistingReplicationController(fakeClient, test.rc, "default", test.name, test.deploymentKey, test.deploymentValue, buffer)
		if !reflect.DeepEqual(rc, test.expectedRc) {
			t.Errorf("expected:\n%#v\ngot:\n%#v\n", test.expectedRc, rc)
		}
		if test.expectErr && err == nil {
			t.Errorf("unexpected non-error")
		}
		if !test.expectErr && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestUpdateWithRetries(t *testing.T) {
	codec := testapi.Codec()
	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{Name: "rc",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: api.ReplicationControllerSpec{
			Selector: map[string]string{
				"foo": "bar",
			},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: api.PodSpec{
					RestartPolicy: api.RestartPolicyAlways,
					DNSPolicy:     api.DNSClusterFirst,
				},
			},
		},
	}

	// Test end to end updating of the rc with retries. Essentially make sure the update handler
	// sees the right updates, failures in update/get are handled properly, and that the updated
	// rc with new resource version is returned to the caller. Without any of these rollingupdate
	// will fail cryptically.
	newRc := *rc
	newRc.ResourceVersion = "2"
	newRc.Spec.Selector["baz"] = "foobar"
	updates := []*http.Response{
		{StatusCode: 500, Body: objBody(codec, &api.ReplicationController{})},
		{StatusCode: 500, Body: objBody(codec, &api.ReplicationController{})},
		{StatusCode: 200, Body: objBody(codec, &newRc)},
	}
	gets := []*http.Response{
		{StatusCode: 500, Body: objBody(codec, &api.ReplicationController{})},
		{StatusCode: 200, Body: objBody(codec, rc)},
	}
	fakeClient := &client.FakeRESTClient{
		Codec: codec,
		Client: client.HTTPClientFunc(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == testapi.ResourcePath("replicationcontrollers", "default", "rc") && m == "PUT":
				update := updates[0]
				updates = updates[1:]
				// We should always get an update with a valid rc even when the get fails. The rc should always
				// contain the update.
				if c, ok := readOrDie(t, req, codec).(*api.ReplicationController); !ok || !reflect.DeepEqual(rc, c) {
					t.Errorf("Unexpected update body, got %+v expected %+v", c, rc)
				} else if sel, ok := c.Spec.Selector["baz"]; !ok || sel != "foobar" {
					t.Errorf("Expected selector label update, got %+v", c.Spec.Selector)
				} else {
					delete(c.Spec.Selector, "baz")
				}
				return update, nil
			case p == testapi.ResourcePath("replicationcontrollers", "default", "rc") && m == "GET":
				get := gets[0]
				gets = gets[1:]
				return get, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	clientConfig := &client.Config{Version: testapi.Version()}
	client := client.NewOrDie(clientConfig)
	client.Client = fakeClient.Client

	if rc, err := updateWithRetries(
		client.ReplicationControllers("default"), rc, func(c *api.ReplicationController) {
			c.Spec.Selector["baz"] = "foobar"
		}); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if sel, ok := rc.Spec.Selector["baz"]; !ok || sel != "foobar" || rc.ResourceVersion != "2" {
		t.Errorf("Expected updated rc, got %+v", rc)
	}
	if len(updates) != 0 || len(gets) != 0 {
		t.Errorf("Remaining updates %+v gets %+v", updates, gets)
	}
}

func readOrDie(t *testing.T, req *http.Request, codec runtime.Codec) runtime.Object {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Errorf("Error reading: %v", err)
		t.FailNow()
	}
	obj, err := codec.Decode(data)
	if err != nil {
		t.Errorf("error decoding: %v", err)
		t.FailNow()
	}
	return obj
}

func objBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func TestAddDeploymentHash(t *testing.T) {
	buf := &bytes.Buffer{}
	codec := testapi.Codec()
	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{Name: "rc"},
		Spec: api.ReplicationControllerSpec{
			Selector: map[string]string{
				"foo": "bar",
			},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	podList := &api.PodList{
		Items: []api.Pod{
			{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			{ObjectMeta: api.ObjectMeta{Name: "bar"}},
			{ObjectMeta: api.ObjectMeta{Name: "baz"}},
		},
	}

	seen := util.StringSet{}
	updatedRc := false
	fakeClient := &client.FakeRESTClient{
		Codec: codec,
		Client: client.HTTPClientFunc(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == testapi.ResourcePath("pods", "default", "") && m == "GET":
				if req.URL.RawQuery != "labelSelector=foo%3Dbar" {
					t.Errorf("Unexpected query string: %s", req.URL.RawQuery)
				}
				return &http.Response{StatusCode: 200, Body: objBody(codec, podList)}, nil
			case p == testapi.ResourcePath("pods", "default", "foo") && m == "PUT":
				seen.Insert("foo")
				obj := readOrDie(t, req, codec)
				podList.Items[0] = *(obj.(*api.Pod))
				return &http.Response{StatusCode: 200, Body: objBody(codec, &podList.Items[0])}, nil
			case p == testapi.ResourcePath("pods", "default", "bar") && m == "PUT":
				seen.Insert("bar")
				obj := readOrDie(t, req, codec)
				podList.Items[1] = *(obj.(*api.Pod))
				return &http.Response{StatusCode: 200, Body: objBody(codec, &podList.Items[1])}, nil
			case p == testapi.ResourcePath("pods", "default", "baz") && m == "PUT":
				seen.Insert("baz")
				obj := readOrDie(t, req, codec)
				podList.Items[2] = *(obj.(*api.Pod))
				return &http.Response{StatusCode: 200, Body: objBody(codec, &podList.Items[2])}, nil
			case p == testapi.ResourcePath("replicationcontrollers", "default", "rc") && m == "PUT":
				updatedRc = true
				return &http.Response{StatusCode: 200, Body: objBody(codec, rc)}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	clientConfig := &client.Config{Version: testapi.Version()}
	client := client.NewOrDie(clientConfig)
	client.Client = fakeClient.Client

	if _, err := AddDeploymentKeyToReplicationController(rc, client, "dk", "hash", api.NamespaceDefault, buf); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	for _, pod := range podList.Items {
		if !seen.Has(pod.Name) {
			t.Errorf("Missing update for pod: %s", pod.Name)
		}
	}
	if !updatedRc {
		t.Errorf("Failed to update replication controller with new labels")
	}
}
