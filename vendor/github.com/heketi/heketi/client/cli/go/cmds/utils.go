//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmds

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func setTagsCommand(cmd *cobra.Command,
	submitTags func(id string, r *api.TagsChangeRequest) error) error {

	//ensure proper number of args
	s := cmd.Flags().Args()
	if len(s) < 1 {
		return errors.New("id required")
	}
	if len(s) < 2 {
		return errors.New("at least one tag:value pair expected")
	}

	exact, err := cmd.Flags().GetBool("exact")
	if err != nil {
		return err
	}

	id = s[0]

	newTags := map[string]string{}
	for _, t := range s[1:] {
		parts := strings.SplitN(t, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf(
				"expected colon (:) between tag name and value, got: %v",
				t)
		}
		newTags[parts[0]] = parts[1]
	}

	var req *api.TagsChangeRequest
	if exact {
		req = &api.TagsChangeRequest{Tags: newTags, Change: api.SetTags}
	} else {
		req = &api.TagsChangeRequest{Tags: newTags, Change: api.UpdateTags}
	}

	return submitTags(id, req)
}

func rmTagsCommand(cmd *cobra.Command,
	submitTags func(id string, r *api.TagsChangeRequest) error) error {

	//ensure proper number of args
	s := cmd.Flags().Args()
	if len(s) < 1 {
		return errors.New("id required")
	}

	id := s[0]

	removeAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	if len(s) < 2 && !removeAll {
		return errors.New("at least one tag expected")
	}
	if len(s) >= 2 && removeAll {
		return errors.New("--all may not be combined with named tags")
	}

	var req *api.TagsChangeRequest
	if removeAll {
		setTags := map[string]string{}
		req = &api.TagsChangeRequest{Tags: setTags, Change: api.SetTags}
	} else {
		// to keep the api simple and consistent delete takes a map
		// however the values in that map are ignored in the
		// delete case
		setTags := map[string]string{}
		for _, k := range s[1:] {
			setTags[k] = ""
		}
		req = &api.TagsChangeRequest{Tags: setTags, Change: api.DeleteTags}
	}

	return submitTags(id, req)
}

func newHeketiClient() (*client.Client, error) {
	return client.NewClientTLS(
		options.Url,
		options.User,
		options.Key,
		&client.ClientTLSOptions{
			InsecureSkipVerify: options.InsecureTLS,
			VerifyCerts:        options.TLSCerts,
		})
}
