// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestDiskUsage(t *testing.T) {
	t.Parallel()
	duData := `
{
  "LayersSize": 17667551166,
  "Images": [
    {
      "Containers": 7,
      "Created": 1536130571,
      "Id": "sha256:056f6f1952204e38bd67cd2901c0cb2fc4cc8b640d1264814a9916b33eb34794",
      "Labels": null,
      "ParentId": "",
      "RepoDigests": [
        "fnproject/fn-test-utils@sha256:2ce83a86519d48b4f0deec062887c8aebf483708f4b87c0756a7cb108ecc98f8"
      ],
      "RepoTags": [
        "fnproject/fn-test-utils:latest"
      ],
      "SharedSize": 4413370,
      "Size": 10861179,
      "VirtualSize": 10861179
    }
  ],
  "Containers": [
    {
      "Id": "52bd62a82a72d8db8162eeef45a15dbec0a9066903631bff99a02c5e8dafcb3c",
      "Names": [
        "/0_prefork_01CP3AMNDS0000000000000001"
      ],
      "Image": "busybox",
      "ImageID": "sha256:e1ddd7948a1c31709a23cc5b7dfe96e55fc364f90e1cebcde0773a1b5a30dcda",
      "Command": "tail -f /dev/null",
      "Created": 1535562634,
      "Ports": [],
      "SizeRootFs": 1162769,
      "Labels": {},
      "State": "running",
      "Status": "Up 2 weeks",
      "HostConfig": {
        "NetworkMode": "default"
      },
      "NetworkSettings": {
        "Networks": {
          "bridge": {
            "IPAMConfig": null,
            "Links": null,
            "Aliases": null,
            "NetworkID": "2e879f7f3faba9c4970920e31b1185cadccb8a5c564a8393871c5ae114c49b39",
            "EndpointID": "853c2b7bc4e7bd47834a45d0c93465ffaecea09103fcf4caa098c88b974f4124",
            "Gateway": "172.17.0.1",
            "IPAddress": "172.17.0.5",
            "IPPrefixLen": 16,
            "IPv6Gateway": "",
            "GlobalIPv6Address": "",
            "GlobalIPv6PrefixLen": 0,
            "MacAddress": "02:42:ac:11:00:05",
            "DriverOpts": null
          }
        }
      },
      "Mounts": []
    }
  ],
  "Volumes": [
    {
      "CreatedAt": "2018-07-18T11:17:34-07:00",
      "Driver": "local",
      "Labels": null,
      "Mountpoint": "",
      "Name": "1284e17abce1d43818d7136849095c6a449a8dcfbaa859c2ff7c40abc75653eb",
      "Options": {},
      "Scope": "local",
      "UsageData": {
        "RefCount": 0,
        "Size": 0
      }
    }
  ],
  "BuilderSize": 0
}

`
	var expected *DiskUsage
	if err := json.Unmarshal([]byte(duData), &expected); err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: duData, status: http.StatusOK})
	du, err := client.DiskUsage(DiskUsageOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(du, expected) {
		t.Errorf("DiskUsage: Wrong return value. Want %#v. Got %#v.", expected, du)
	}
}
