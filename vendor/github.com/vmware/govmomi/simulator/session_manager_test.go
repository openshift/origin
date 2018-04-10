/*
Copyright (c) 2017-2018 VMware, Inc. All Rights Reserved.

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
	"strings"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

func isNotAuthenticated(err error) bool {
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).VimFault().(type) {
		case types.NotAuthenticated:
			return true
		}
	}
	return false
}

func TestSessionManagerAuth(t *testing.T) {
	ctx := context.Background()

	m := VPX()

	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	u := s.URL.User
	s.URL.User = nil // skip Login()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	session, err := c.SessionManager.UserSession(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if session != nil {
		t.Error("expected nil session")
	}

	opts := object.NewOptionManager(c.Client, *c.ServiceContent.Setting)
	var content []types.ObjectContent
	err = opts.Properties(ctx, opts.Reference(), []string{"setting"}, &content)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) != 1 {
		t.Error("expected content len=1")
	}

	if len(content[0].PropSet) != 0 {
		t.Error("non-empty PropSet")
	}

	if len(content[0].MissingSet) != 1 {
		t.Error("expected MissingSet len=1")
	}

	if _, ok := content[0].MissingSet[0].Fault.Fault.(*types.NotAuthenticated); !ok {
		t.Error("expected NotAuthenticated")
	}

	_, err = methods.GetCurrentTime(ctx, c)
	if !isNotAuthenticated(err) {
		t.Error("expected NotAuthenticated")
	}

	err = c.SessionManager.Login(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	c.UserAgent = "vcsim/x.x"

	session, err = c.SessionManager.UserSession(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if session == nil {
		t.Error("expected session")
	}

	if session.UserAgent != c.UserAgent {
		t.Errorf("UserAgent=%s", session.UserAgent)
	}

	content = nil
	err = opts.Properties(ctx, opts.Reference(), []string{"setting"}, &content)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) != 1 {
		t.Error("expected content len=1")
	}

	if len(content[0].PropSet) != 1 {
		t.Error("PropSet len=1")
	}

	if len(content[0].MissingSet) != 0 {
		t.Error("expected MissingSet len=0")
	}

	last := session.LastActiveTime

	_, err = methods.GetCurrentTime(ctx, c)
	if err != nil {
		t.Error(err)
	}

	pc, err := property.DefaultCollector(c.Client).Create(ctx)
	if err != nil {
		t.Fatal(err)
	}

	session, _ = c.SessionManager.UserSession(ctx)

	if session.LastActiveTime.Equal(last) {
		t.Error("LastActiveTime was not updated")
	}

	if !strings.Contains(pc.Reference().Value, session.Key) {
		t.Errorf("invalid ref=%s", pc.Reference())
	}

	ticket, err := c.SessionManager.AcquireCloneTicket(ctx)
	if err != nil {
		t.Fatal(err)
	}

	c, err = govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SessionManager.CloneSession(ctx, ticket)
	if err != nil {
		t.Fatal(err)
	}

	err = c.SessionManager.CloneSession(ctx, ticket)
	if err == nil {
		t.Error("expected error")
	}

	_, err = methods.GetCurrentTime(ctx, c)
	if err != nil {
		t.Error(err)
	}
}
