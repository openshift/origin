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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/types"
)

func TestRace(t *testing.T) {
	ctx := context.Background()

	m := VPX()

	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		spec := types.VirtualMachineConfigSpec{
			Name:    fmt.Sprintf("race-test-%d", i),
			GuestId: string(types.VirtualMachineGuestOsIdentifierOtherGuest),
			Files: &types.VirtualMachineFileInfo{
				VmPathName: "[LocalDS_0]",
			},
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := govmomi.NewClient(ctx, s.URL, true)
			if err != nil {
				t.Fatal(err)
			}

			finder := find.NewFinder(c.Client, false)
			pc := property.DefaultCollector(c.Client)
			dc, err := finder.DefaultDatacenter(ctx)
			if err != nil {
				t.Fatal(err)
			}

			finder.SetDatacenter(dc)

			f, err := dc.Folders(ctx)
			if err != nil {
				t.Fatal(err)
			}

			pool, err := finder.ResourcePool(ctx, "DC0_C0/Resources")
			if err != nil {
				t.Fatal(err)
			}

			ticker := time.NewTicker(time.Millisecond * 100)
			defer ticker.Stop()

			for j := 0; j < 2; j++ {
				cspec := spec // copy spec and give it a unique name
				cspec.Name += fmt.Sprintf("-%d", j)

				wg.Add(1)
				go func() {
					defer wg.Done()
					task, _ := f.VmFolder.CreateVM(ctx, cspec, pool, nil)
					info, terr := task.WaitForResult(ctx, nil)
					if terr != nil {
						t.Error(terr)
					}
					go func() {
						for _ = range ticker.C {
							var content []types.ObjectContent
							rerr := pc.RetrieveOne(ctx, info.Result.(types.ManagedObjectReference), nil, &content)
							if rerr != nil {
								t.Error(rerr)
							}
						}
					}()
				}()
			}

			vms, err := finder.VirtualMachineList(ctx, "*")
			if err != nil {
				t.Fatal(err)
			}

			for i := range vms {
				vm := vms[i]
				wg.Add(1)
				go func() {
					defer wg.Done()
					task, _ := vm.PowerOff(ctx)
					_ = task.Wait(ctx)
				}()
			}
		}()
	}

	wg.Wait()
}
