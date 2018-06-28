/*
Copyright (c) 2018 VMware, Inc. All Rights Reserved.

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

package simulator

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/vim25/types"
)

func TestEventManagerVPX(t *testing.T) {
	logEvents = testing.Verbose()
	ctx := context.Background()

	m := VPX()
	m.Datacenter = 2

	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	e := event.NewManager(c.Client)
	count := m.Count()

	root := c.ServiceContent.RootFolder
	vm := Map.Any("VirtualMachine").(*VirtualMachine)
	host := Map.Get(vm.Runtime.Host.Reference()).(*HostSystem)

	vmEvents := 6 // BeingCreated + InstanceUuid + Uuid + Created + Starting + PoweredOn
	tests := []struct {
		obj    types.ManagedObjectReference
		expect int
		ids    []string
	}{
		{root, -1 * count.Machine, nil},
		{root, 1, []string{"SessionEvent"}}, // UserLoginSessionEvent
		{vm.Reference(), 0, []string{"SessionEvent"}},
		{root, count.Machine, []string{"VmCreatedEvent"}},     // concrete type
		{root, count.Machine * vmEvents, []string{"VmEvent"}}, // base type
		{vm.Reference(), 1, []string{"VmCreatedEvent"}},
		{vm.Reference(), vmEvents, nil},
		{host.Reference(), len(host.Vm), []string{"VmCreatedEvent"}},
		{host.Reference(), len(host.Vm) * vmEvents, nil},
	}

	for i, test := range tests {
		n := 0
		filter := types.EventFilterSpec{
			Entity: &types.EventFilterSpecByEntity{
				Entity:    test.obj,
				Recursion: types.EventFilterSpecRecursionOptionAll,
			},
			EventTypeId: test.ids,
			MaxCount:    100,
		}

		f := func(obj types.ManagedObjectReference, events []types.BaseEvent) error {
			n += len(events)

			qevents, qerr := e.QueryEvents(ctx, filter)
			if qerr != nil {
				t.Fatal(qerr)
			}

			if n != len(qevents) {
				t.Errorf("%d vs %d", n, len(qevents))
			}

			return nil
		}

		err = e.Events(ctx, []types.ManagedObjectReference{test.obj}, filter.MaxCount, false, false, f, test.ids...)
		if err != nil {
			t.Fatalf("%d: %s", i, err)
		}

		if test.expect < 0 {
			expect := test.expect * -1
			if n < expect {
				t.Errorf("%d: expected at least %d events, got: %d", i, expect, n)
			}
			continue
		}

		if test.expect != n {
			t.Errorf("%d: expected %d events, got: %d", i, test.expect, n)
		}
	}
}
