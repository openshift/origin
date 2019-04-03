package storageos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/storageos/go-api/types"
)

func TestPoolList(t *testing.T) {
	poolsData := `[
		{
			"id": "0ee42bdc-e6e1-b82a-cf8e-6bf3c68dd212",
			"name": "default",
			"description": "Default storage pool",
			"default": true,
			"nodeSelector": "",
			"deviceSelector": "",
			"capacityStats": {
				"totalCapacityBytes": 40576331776,
				"availableCapacityBytes": 38560821248
			},
			"nodes": [
				{
					"id": "68ed109e-6e7b-8355-b024-93ad3df66ba9",
					"hostname": "",
					"address": "172.28.128.3",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "london"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "42cd02ed-2ffc-ab88-13e5-795b3fbdda4e",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560800768
							},
							"createdAt": "2018-04-03T14:30:12.971890842Z",
							"updatedAt": "2018-04-03T16:21:13.561891086Z"
						}
					],
					"hostID": 48073,
					"meta": {
						"modifiedRevision": 10075,
						"createdRevision": 3
					},
					"name": "storageos-1-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:12.967743135Z",
					"updatedAt": "2018-04-03T16:21:13.565125252Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T16:13:03.893795131Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "StorageOS karolis (5874fd7), built: 2018-04-03T142531Z",
					"Revision": "",
					"scheduler": true,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9129674649600,
						"availableCapacityBytes": 8676616826880
					}
				},
				{
					"id": "6a0bc24d-284a-7400-a81d-e8ebd67ae5b2",
					"hostname": "",
					"address": "172.28.128.5",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "paris"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "4f3a6607-5cf4-e575-55f6-2ef3c5acff03",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560849920
							},
							"createdAt": "2018-04-03T14:30:55.611128911Z",
							"updatedAt": "2018-04-03T16:21:13.565281979Z"
						}
					],
					"hostID": 35669,
					"meta": {
						"modifiedRevision": 10076,
						"createdRevision": 94
					},
					"name": "storageos-3-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:55.599095031Z",
					"updatedAt": "2018-04-03T16:21:13.572610769Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T16:21:04.981879955Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "StorageOS karolis (5874fd7), built: 2018-04-03T142531Z",
					"Revision": "",
					"scheduler": false,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9048521986048,
						"availableCapacityBytes": 8599498940416
					}
				},
				{
					"id": "c2c32691-1c20-e025-50ca-7b4e310689c5",
					"hostname": "",
					"address": "172.28.128.4",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "london"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "9cb3b267-d666-df0e-59c3-b45990e65b79",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560821248
							},
							"createdAt": "2018-04-03T14:30:30.236296395Z",
							"updatedAt": "2018-04-03T16:21:13.562148581Z"
						}
					],
					"hostID": 38397,
					"meta": {
						"modifiedRevision": 10074,
						"createdRevision": 41
					},
					"name": "storageos-2-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:30.215144731Z",
					"updatedAt": "2018-04-03T16:21:13.565442826Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T14:30:43.95036511Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "StorageOS karolis (5874fd7), built: 2018-04-03T142531Z",
					"Revision": "",
					"scheduler": false,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9007945654272,
						"availableCapacityBytes": 8560932982784
					}
				}
			],
			"labels": null
		}
	]`

	var expected []*types.Pool
	if err := json.Unmarshal([]byte(poolsData), &expected); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(&FakeRoundTripper{message: poolsData, status: http.StatusOK})
	pools, err := client.PoolList(types.ListOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(pools, expected) {
		t.Errorf("Pools: Wrong return value. Want %#v. Got %#v.", expected, pools)
	}
}

func TestPoolCreate(t *testing.T) {
	body := `
		{
			"id": "0ee42bdc-e6e1-b82a-cf8e-6bf3c68dd212",
			"name": "default",
			"description": "Default storage pool",
			"default": true,
			"nodeSelector": "",
			"deviceSelector": "",
			"capacityStats": {
				"totalCapacityBytes": 40576331776,
				"availableCapacityBytes": 38560821248
			},
			"nodes": [
				{
					"id": "68ed109e-6e7b-8355-b024-93ad3df66ba9",
					"hostname": "",
					"address": "172.28.128.3",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "london"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "42cd02ed-2ffc-ab88-13e5-795b3fbdda4e",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560800768
							},
							"createdAt": "2018-04-03T14:30:12.971890842Z",
							"updatedAt": "2018-04-03T16:21:13.561891086Z"
						}
					],
					"hostID": 48073,
					"meta": {
						"modifiedRevision": 10075,
						"createdRevision": 3
					},
					"name": "storageos-1-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:12.967743135Z",
					"updatedAt": "2018-04-03T16:21:13.565125252Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T16:13:03.893795131Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "StorageOS karolis (5874fd7), built: 2018-04-03T142531Z",
					"Revision": "",
					"scheduler": true,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9129674649600,
						"availableCapacityBytes": 8676616826880
					}
				},
				{
					"id": "6a0bc24d-284a-7400-a81d-e8ebd67ae5b2",
					"hostname": "",
					"address": "172.28.128.5",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "paris"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "4f3a6607-5cf4-e575-55f6-2ef3c5acff03",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560849920
							},
							"createdAt": "2018-04-03T14:30:55.611128911Z",
							"updatedAt": "2018-04-03T16:21:13.565281979Z"
						}
					],
					"hostID": 35669,
					"meta": {
						"modifiedRevision": 10076,
						"createdRevision": 94
					},
					"name": "storageos-3-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:55.599095031Z",
					"updatedAt": "2018-04-03T16:21:13.572610769Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T16:21:04.981879955Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "StorageOS karolis (5874fd7), built: 2018-04-03T142531Z",
					"Revision": "",
					"scheduler": false,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9048521986048,
						"availableCapacityBytes": 8599498940416
					}
				},
				{
					"id": "c2c32691-1c20-e025-50ca-7b4e310689c5",
					"hostname": "",
					"address": "172.28.128.4",
					"kvAddr": "",
					"apiPort": 5705,
					"natsPort": 5708,
					"natsClusterPort": 5710,
					"serfPort": 5711,
					"dfsPort": 5703,
					"kvPeerPort": 5707,
					"kvClientPort": 5706,
					"labels": {
						"location": "london"
					},
					"logLevel": "",
					"logFormat": "",
					"logFilter": "",
					"bindAddr": "",
					"deviceDir": "/var/lib/storageos/volumes",
					"clusterId": "",
					"initialCluster": "",
					"join": "",
					"kvBackend": "",
					"debug": false,
					"devices": [
						{
							"ID": "9cb3b267-d666-df0e-59c3-b45990e65b79",
							"labels": {
								"default": "true"
							},
							"status": "active",
							"identifier": "/var/lib/storageos/data",
							"class": "filesystem",
							"capacityStats": {
								"totalCapacityBytes": 40576331776,
								"availableCapacityBytes": 38560821248
							},
							"createdAt": "2018-04-03T14:30:30.236296395Z",
							"updatedAt": "2018-04-03T16:21:13.562148581Z"
						}
					],
					"hostID": 38397,
					"meta": {
						"modifiedRevision": 10074,
						"createdRevision": 41
					},
					"name": "storageos-2-54179",
					"description": "",
					"createdAt": "2018-04-03T14:30:30.215144731Z",
					"updatedAt": "2018-04-03T16:21:13.565442826Z",
					"health": "healthy",
					"healthUpdatedAt": "2018-04-03T14:30:43.95036511Z",
					"versionInfo": {
						"storageos": {
							"name": "storageos",
							"buildDate": "2018-04-03T142531Z",
							"buildRef": "",
							"revision": "5874fd7",
							"version": "karolis",
							"apiVersion": "1",
							"goVersion": "go1.9.1",
							"os": "linux",
							"arch": "amd64",
							"kernelVersion": "",
							"experimental": false
						}
					},
					"version": "x",
					"Revision": "",
					"scheduler": false,
					"unschedulable": false,
					"volumeStats": {
						"masterVolumeCount": 0,
						"replicaVolumeCount": 0,
						"virtualVolumeCount": 0
					},
					"capacityStats": {
						"totalCapacityBytes": 9007945654272,
						"availableCapacityBytes": 8560932982784
					}
				}
			],
			"labels": null
		}`
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	pool, err := client.PoolCreate(
		types.PoolOptions{
			Name:        "unit01",
			Description: "Unit test pool",
			Default:     false,
			Labels: map[string]string{
				"foo": "bar",
			},
			Context: context.Background(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if pool == nil {
		t.Fatalf("PoolCreate: Wrong return value. Wanted pool. Got %v.", pool)
	}
	if len(pool.ID) != 36 {
		t.Errorf("PoolCreate: Wrong return value. Wanted 34 character UUID. Got %d. (%s)", len(pool.ID), pool.ID)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("PoolCreate(): Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getAPIPath(PoolAPIPrefix, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("PoolCreate(): Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestPool(t *testing.T) {
	body := `{
                "active": true,
                "capacity_stats": {
                    "available_capacity_bytes": 80296787968,
                    "provisioned_capacity_bytes": 5368709120,
                    "total_capacity_bytes": 103440351232
                },                                
                "description": "Default storage pool",                
                "id": "2935b1b9-a8af-121c-9e79-a64c637f0ee9",
                "name": "default",
                "id": "b4c87d6c-2958-6283-128b-f767153938ad",
                "tags": [
                    "prod",
                    "london"
                ],
                "tenant": "",
                "type": ""
            }`
	var expected types.Pool
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	name := "default"
	pool, err := client.Pool(name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pool, &expected) {
		t.Errorf("Pool: Wrong return value. Want %#v. Got %#v.", expected, pool)
	}
	req := fakeRT.requests[0]
	expectedMethod := "GET"
	if req.Method != expectedMethod {
		t.Errorf("InspectPool(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getAPIPath(PoolAPIPrefix+"/"+name, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("PoolCreate(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestPoolDelete(t *testing.T) {
	name := "testdelete"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	if err := client.PoolDelete(types.DeleteOptions{Name: name}); err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("PoolDelete(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getAPIPath(PoolAPIPrefix+"/"+name, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("PoolDelete(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestPoolDeleteNotFound(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "no such pool", status: http.StatusNotFound})
	if err := client.PoolDelete(types.DeleteOptions{Name: "testdeletenotfound"}); err != ErrNoSuchPool {
		t.Errorf("PoolDelete: wrong error. Want %#v. Got %#v.", ErrNoSuchPool, err)
	}
}

func TestPoolDeleteInUse(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "pool in use and cannot be removed", status: http.StatusConflict})
	if err := client.PoolDelete(types.DeleteOptions{Name: "testdeletinuse"}); err != ErrPoolInUse {
		t.Errorf("PoolDelete: wrong error. Want %#v. Got %#v.", ErrNamespaceInUse, err)
	}
}

func TestPoolDeleteForce(t *testing.T) {
	name := "testdelete"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	if err := client.PoolDelete(types.DeleteOptions{Name: name, Force: true}); err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	vals := req.URL.Query()
	if len(vals) == 0 {
		t.Error("PoolDelete: query string empty. Expected force=1.")
	}
	force := vals.Get("force")
	if force != "1" {
		t.Errorf("PoolDelete(%q): Force not set. Want %q. Got %q.", name, "1", force)
	}
}
