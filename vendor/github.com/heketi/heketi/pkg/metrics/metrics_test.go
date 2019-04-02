//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package metrics

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

type testApp struct {
	topologyInfo *api.TopologyInfoResponse
}

func (t *testApp) SetRoutes(router *mux.Router) error {
	return nil
}

func (t *testApp) TopologyInfo() (*api.TopologyInfoResponse, error) {
	return t.topologyInfo, nil
}

func (t *testApp) Close() {}

func (t *testApp) Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
}

func TestMetricsEndpoint(t *testing.T) {
	ta := &testApp{
		topologyInfo: &api.TopologyInfoResponse{
			ClusterList: []api.Cluster{
				{
					Id: "c1",
					Nodes: []api.NodeInfoResponse{
						{
							NodeInfo: api.NodeInfo{NodeAddRequest: api.NodeAddRequest{Hostnames: api.HostAddresses{Manage: []string{"n1"}}}},
							DevicesInfo: []api.DeviceInfoResponse{
								{
									DeviceInfo: api.DeviceInfo{
										Device: api.Device{Name: "d1"},
										Storage: api.StorageSize{
											Total: 2,
											Free:  1,
											Used:  1,
										},
									},
									Bricks: []api.BrickInfo{
										{
											Id:   "b1",
											Size: 2,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ts := httptest.NewServer(NewMetricsHandler(ta))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	match, err := regexp.Match("heketi_nodes_count{cluster=\"c1\"} 1", body)
	if !match || err != nil {
		t.Fatal("heketi_nodes_count{cluster=\"c1\"} 1 should be present in the metrics output")
	}

	match, err = regexp.Match("heketi_device_size{cluster=\"c1\",device=\"d1\",hostname=\"n1\"} 2", body)
	if !match || err != nil {
		t.Fatal("heketi_device_size{cluster=\"c1\",device=\"d1\",hostname=\"n1\"} 2 should be present in the metrics output")
	}
}
