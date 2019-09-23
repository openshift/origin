// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package client_tls_test

import (
	"testing"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/heketi/tests"
)

var (
	URL      = "https://localhost:8080"
	User     = "abc"
	Key      = "xyz"
	CertPath = "../heketi.crt"
)

func TestCreateClusterTLSCert(t *testing.T) {
	heketiServer := NewServerCtlFromEnv("..")
	err := heketiServer.Start()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer heketiServer.Stop()

	heketi, err := client.NewClientTLS(URL, User, Key, &client.ClientTLSOptions{
		VerifyCerts: []string{CertPath},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, heketi != nil, "expected heketi != nil, got:", heketi)

	testClientActions(t, heketi)
}

func TestCreateClusterTLSNoVerify(t *testing.T) {
	heketiServer := NewServerCtlFromEnv("..")
	err := heketiServer.Start()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer heketiServer.Stop()

	heketi, err := client.NewClientTLS(URL, User, Key, &client.ClientTLSOptions{
		InsecureSkipVerify: true,
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, heketi != nil, "expected heketi != nil, got:", heketi)

	testClientActions(t, heketi)
}

// This test checks that the client fails when the server is using
// a self signed cert and none of the options needed for it
// are provided.
func TestClientFailUnknownAuthority(t *testing.T) {
	heketiServer := NewServerCtlFromEnv("..")
	err := heketiServer.Start()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer heketiServer.Stop()

	heketi, err := client.NewClientTLS(URL, User, Key, &client.ClientTLSOptions{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, heketi != nil, "expected heketi != nil, got:", heketi)

	clusterReq := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	_, err = heketi.ClusterCreate(clusterReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func testClientActions(t *testing.T, heketi *client.Client) {
	clusterReq := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := heketi.ClusterCreate(clusterReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, cluster.Id != "", `expected cluster.Id != "", got ""`)

	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"foo"}
	nodeReq.Hostnames.Storage = []string{"foo"}
	nodeReq.Zone = 1
	node, err := heketi.NodeAdd(nodeReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, node.Id != "", `expected node.Id != "", got ""`)

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)
	tests.Assert(t, len(clusters.Clusters) == 1,
		"expected len(clusters.Clusters) == 1, got:", len(clusters.Clusters))

	clusterInfo, err := heketi.ClusterInfo(clusters.Clusters[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(clusterInfo.Nodes) == 1,
		"expected len(clusterInfo.Nodes) == 1, got:", len(clusterInfo.Nodes))
	tests.Assert(t, clusterInfo.Nodes[0] == node.Id,
		"expected clusterInfo.Nodes[0] == node.Id, got:",
		clusterInfo.Nodes[0] == node.Id)
}
