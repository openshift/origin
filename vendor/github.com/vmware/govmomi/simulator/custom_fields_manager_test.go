/*
Copyright (c) 2017 VMware, Inc. All Rights Reserved.

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
	"reflect"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator/vpx"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestCustomFieldsManager(t *testing.T) {
	s := New(NewServiceInstance(vpx.ServiceContent, vpx.RootFolder))

	ts := s.NewServer()
	defer ts.Close()

	ctx := context.Background()

	c, err := govmomi.NewClient(ctx, ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	fieldsManager, err := object.GetCustomFieldsManager(c.Client)
	if err != nil {
		t.Fatal(err)
	}

	field, err := fieldsManager.Add(ctx, "field_name", "VirtualMachine", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if field.Name != "field_name" && field.Type != "VirtualMachine" {
		t.Fatal("field add result mismatched with the inserted")
	}

	fields, err := fieldsManager.Field(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("expect len(fields)=1; got %d", len(fields))
	}
	if !reflect.DeepEqual(&fields[0], field) {
		t.Fatalf("expect fields[0]==field; got %+v,%+v", fields[0], field)
	}

	key, err := fieldsManager.FindKey(ctx, field.Name)
	if err != nil {
		t.Fatal(err)
	}
	if key != field.Key {
		t.Fatalf("expect key == field.Key; got %d != %d", key, field.Key)
	}

	err = fieldsManager.Rename(ctx, key, "new_field_name")
	if err != nil {
		t.Fatal(err)
	}

	fields, err = fieldsManager.Field(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("expect len(fields)=1; got %d", len(fields))
	}
	if fields[0].Name != "new_field_name" {
		t.Fatalf("expect field.name to be %s; got %s", "new_field_name", fields[0].Name)
	}

	folder := Map.content().RootFolder
	err = fieldsManager.Set(ctx, folder, 1, "value")
	if err != nil {
		t.Fatal(err)
	}

	values := Map.Get(folder.Reference()).(mo.Entity).Entity().CustomValue
	if len(values) != 1 {
		t.Fatalf("expect CustomValue has 1 item; got %d", len(values))
	}
	fkey := values[0].GetCustomFieldValue().Key
	if fkey != 1 {
		t.Fatalf("expect value.Key to be 1; got %d", fkey)
	}
	value := values[0].(*types.CustomFieldStringValue).Value
	if value != "value" {
		t.Fatalf("expect value.Value to be %q; got %q", "value", value)
	}

	err = fieldsManager.Remove(ctx, field.Key)
	if err != nil {
		t.Fatal(err)
	}

	fields, err = fieldsManager.Field(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 0 {
		t.Fatalf("expect fields to be empty; got %+v", fields)
	}
}
