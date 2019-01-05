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

package category

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/vmware/govmomi/govc/cli"
	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
)

type ls struct {
	*flags.ClientFlag
	*flags.OutputFlag
}

func init() {
	cli.Register("tags.category.ls", &ls{})
}

func (cmd *ls) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *ls) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

func (cmd *ls) Description() string {
	return `List all categories.

Examples:
  govc tags.category.ls
  govc tags.category.ls -json | jq .`
}

func withClient(ctx context.Context, cmd *flags.ClientFlag, f func(*rest.Client) error) error {
	vc, err := cmd.Client()
	if err != nil {
		return err
	}
	tagsURL := vc.URL()
	tagsURL.User = cmd.Userinfo()

	c := rest.NewClient(vc)
	if err != nil {
		return err
	}

	if err = c.Login(ctx, tagsURL.User); err != nil {
		return err
	}
	defer c.Logout(ctx)

	return f(c)
}

type lsResult []tags.Category

func (r lsResult) Write(w io.Writer) error {
	for _, c := range r {
		fmt.Fprintln(w, c.Name)
	}
	return nil
}

func (cmd *ls) Run(ctx context.Context, f *flag.FlagSet) error {
	return withClient(ctx, cmd.ClientFlag, func(c *rest.Client) error {
		l, err := tags.NewManager(c).GetCategories(ctx)
		if err != nil {
			return err
		}

		return cmd.WriteResult(lsResult(l))
	})
}
