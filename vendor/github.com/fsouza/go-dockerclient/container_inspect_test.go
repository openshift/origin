package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestInspectContainer(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
             "AppArmorProfile": "Profile",
             "Created": "2013-05-07T14:51:42.087658+02:00",
             "Path": "date",
             "Args": [],
             "Config": {
                     "Hostname": "4fa6e0f0c678",
                     "User": "",
                     "Memory": 17179869184,
                     "MemorySwap": 34359738368,
                     "AttachStdin": false,
                     "AttachStdout": true,
                     "AttachStderr": true,
                     "PortSpecs": null,
                     "Tty": false,
                     "OpenStdin": false,
                     "StdinOnce": false,
                     "Env": null,
                     "Cmd": [
                             "date"
                     ],
                     "Image": "base",
                     "Volumes": {},
                     "VolumesFrom": "",
                     "SecurityOpt": [
                         "label:user:USER"
                      ],
                      "Ulimits": [
                          { "Name": "nofile", "Soft": 1024, "Hard": 2048 }
											],
											"Shell": [
                         "/bin/sh", "-c"
											]
             },
             "State": {
                     "Running": false,
                     "Pid": 0,
                     "ExitCode": 0,
                     "StartedAt": "2013-05-07T14:51:42.087658+02:00",
                     "Ghost": false
             },
             "Node": {
                  "ID": "4I4E:QR4I:Z733:QEZK:5X44:Q4T7:W2DD:JRDY:KB2O:PODO:Z5SR:XRB6",
                  "IP": "192.168.99.105",
                  "Addra": "192.168.99.105:2376",
                  "Name": "node-01",
                  "Cpus": 4,
                  "Memory": 1048436736,
                  "Labels": {
                      "executiondriver": "native-0.2",
                      "kernelversion": "3.18.5-tinycore64",
                      "operatingsystem": "Boot2Docker 1.5.0 (TCL 5.4); master : a66bce5 - Tue Feb 10 23:31:27 UTC 2015",
                      "provider": "virtualbox",
                      "storagedriver": "aufs"
                  }
              },
             "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "NetworkSettings": {
                     "IpAddress": "",
                     "IpPrefixLen": 0,
                     "Gateway": "",
                     "Bridge": "",
                     "PortMapping": null
             },
             "SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
             "ResolvConfPath": "/etc/resolv.conf",
             "Volumes": {},
             "HostConfig": {
               "Binds": null,
               "ContainerIDFile": "",
               "LxcConf": [],
               "Privileged": false,
               "PortBindings": {
                 "80/tcp": [
                   {
                     "HostIp": "0.0.0.0",
                     "HostPort": "49153"
                   }
                 ]
               },
               "Links": null,
               "PublishAllPorts": false,
               "CgroupParent": "/mesos",
               "Memory": 17179869184,
               "MemorySwap": 34359738368,
               "GroupAdd": ["fake", "12345"],
               "OomScoreAdj": 642
             }
}`
	var expected Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c678"
	container, err := client.InspectContainer(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*container, expected) {
		t.Errorf("InspectContainer(%q): Expected %#v. Got %#v.", id, expected, container)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/json"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestInspectContainerWithContext(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
             "AppArmorProfile": "Profile",
             "Created": "2013-05-07T14:51:42.087658+02:00",
             "Path": "date",
             "Args": [],
             "Config": {
                     "Hostname": "4fa6e0f0c678",
                     "User": "",
                     "Memory": 17179869184,
                     "MemorySwap": 34359738368,
                     "AttachStdin": false,
                     "AttachStdout": true,
                     "AttachStderr": true,
                     "PortSpecs": null,
                     "Tty": false,
                     "OpenStdin": false,
                     "StdinOnce": false,
                     "Env": null,
                     "Cmd": [
                             "date"
                     ],
                     "Image": "base",
                     "Volumes": {},
                     "VolumesFrom": "",
                     "SecurityOpt": [
                         "label:user:USER"
                      ],
                      "Ulimits": [
                          { "Name": "nofile", "Soft": 1024, "Hard": 2048 }
                      ]
             },
             "State": {
                     "Running": false,
                     "Pid": 0,
                     "ExitCode": 0,
                     "StartedAt": "2013-05-07T14:51:42.087658+02:00",
                     "Ghost": false
             },
             "Node": {
                  "ID": "4I4E:QR4I:Z733:QEZK:5X44:Q4T7:W2DD:JRDY:KB2O:PODO:Z5SR:XRB6",
                  "IP": "192.168.99.105",
                  "Addra": "192.168.99.105:2376",
                  "Name": "node-01",
                  "Cpus": 4,
                  "Memory": 1048436736,
                  "Labels": {
                      "executiondriver": "native-0.2",
                      "kernelversion": "3.18.5-tinycore64",
                      "operatingsystem": "Boot2Docker 1.5.0 (TCL 5.4); master : a66bce5 - Tue Feb 10 23:31:27 UTC 2015",
                      "provider": "virtualbox",
                      "storagedriver": "aufs"
                  }
              },
             "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "NetworkSettings": {
                     "IpAddress": "",
                     "IpPrefixLen": 0,
                     "Gateway": "",
                     "Bridge": "",
                     "PortMapping": null
             },
             "SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
             "ResolvConfPath": "/etc/resolv.conf",
             "Volumes": {},
             "HostConfig": {
               "Binds": null,
               "BlkioDeviceReadIOps": [
                   {
                       "Path": "/dev/sdb",
                       "Rate": 100
                   }
               ],
               "BlkioDeviceWriteBps": [
                   {
                       "Path": "/dev/sdb",
                       "Rate": 5000
                   }
               ],
               "ContainerIDFile": "",
               "LxcConf": [],
               "Privileged": false,
               "PortBindings": {
                 "80/tcp": [
                   {
                     "HostIp": "0.0.0.0",
                     "HostPort": "49153"
                   }
                 ]
               },
               "Links": null,
               "PublishAllPorts": false,
               "CgroupParent": "/mesos",
               "Memory": 17179869184,
               "MemorySwap": 34359738368,
               "GroupAdd": ["fake", "12345"],
               "OomScoreAdj": 642
             }
}`
	var expected Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c678"

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	inspectError := make(chan error)
	// Invoke InspectContainer in a goroutine. The response is sent to the 'inspectError'
	// channel.
	go func() {
		container, err := client.InspectContainer(id)
		if err != nil {
			inspectError <- err
			return
		}
		if !reflect.DeepEqual(*container, expected) {
			inspectError <- fmt.Errorf("inspectContainer(%q): Expected %#v. Got %#v", id, expected, container)
			return
		}
		expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/json"))
		if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
			inspectError <- fmt.Errorf("inspectContainer(%q): Wrong path in request. Want %q. Got %q", id, expectedURL.Path, gotPath)
			return
		}
		// No errors to tbe reported. Send 'nil'
		inspectError <- nil
	}()
	// Wait for either the inspect response or for the context.
	select {
	case err := <-inspectError:
		if err != nil {
			t.Fatalf("Error inspecting container with context: %v", err)
		}
	case <-ctx.Done():
		// Context was canceled unexpectedly. Report the same.
		t.Fatalf("Context canceled when waiting for inspect container response: %v", ctx.Err())
	}
}

func TestInspectContainerWithOptions(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
             "AppArmorProfile": "Profile",
             "Created": "2013-05-07T14:51:42.087658+02:00",
             "Path": "date",
             "Args": [],
             "Config": {
                     "Hostname": "4fa6e0f0c678",
                     "User": "",
                     "Memory": 17179869184,
                     "MemorySwap": 34359738368,
                     "AttachStdin": false,
                     "AttachStdout": true,
                     "AttachStderr": true,
                     "PortSpecs": null,
                     "Tty": false,
                     "OpenStdin": false,
                     "StdinOnce": false,
                     "Env": null,
                     "Cmd": [
                             "date"
                     ],
                     "Image": "base",
                     "Volumes": {},
                     "VolumesFrom": "",
                     "SecurityOpt": [
                         "label:user:USER"
                      ],
                      "Ulimits": [
                          { "Name": "nofile", "Soft": 1024, "Hard": 2048 }
											],
											"Shell": [
                         "/bin/sh", "-c"
											]
             },
             "State": {
                     "Running": false,
                     "Pid": 0,
                     "ExitCode": 0,
                     "StartedAt": "2013-05-07T14:51:42.087658+02:00",
                     "Ghost": false
             },
             "Node": {
                  "ID": "4I4E:QR4I:Z733:QEZK:5X44:Q4T7:W2DD:JRDY:KB2O:PODO:Z5SR:XRB6",
                  "IP": "192.168.99.105",
                  "Addra": "192.168.99.105:2376",
                  "Name": "node-01",
                  "Cpus": 4,
                  "Memory": 1048436736,
                  "Labels": {
                      "executiondriver": "native-0.2",
                      "kernelversion": "3.18.5-tinycore64",
                      "operatingsystem": "Boot2Docker 1.5.0 (TCL 5.4); master : a66bce5 - Tue Feb 10 23:31:27 UTC 2015",
                      "provider": "virtualbox",
                      "storagedriver": "aufs"
                  }
              },
             "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "NetworkSettings": {
                     "IpAddress": "",
                     "IpPrefixLen": 0,
                     "Gateway": "",
                     "Bridge": "",
                     "PortMapping": null
             },
             "SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
             "ResolvConfPath": "/etc/resolv.conf",
             "Volumes": {},
             "HostConfig": {
               "Binds": null,
               "ContainerIDFile": "",
               "LxcConf": [],
               "Privileged": false,
               "PortBindings": {
                 "80/tcp": [
                   {
                     "HostIp": "0.0.0.0",
                     "HostPort": "49153"
                   }
                 ]
               },
               "Links": null,
               "PublishAllPorts": false,
               "CgroupParent": "/mesos",
               "Memory": 17179869184,
               "MemorySwap": 34359738368,
               "GroupAdd": ["fake", "12345"],
               "OomScoreAdj": 642,
               "SizeRw": 3,
               "SizeRootFs": 5552693
             }
}`
	var expected Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	const id = "4fa6e0f0c678"
	container, err := client.InspectContainerWithOptions(InspectContainerOptions{
		ID:   id,
		Size: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*container, expected) {
		t.Errorf("InspectContainer(%q): Expected %#v. Got %#v.", id, expected, container)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/json?size=true"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestInspectContainerNetwork(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
            "Id": "81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c",
            "Created": "2015-11-12T14:54:04.791485659Z",
            "Path": "consul-template",
            "Args": [
                "-config=/tmp/haproxy.json",
                "-consul=192.168.99.120:8500"
            ],
            "State": {
                "Status": "running",
                "Running": true,
                "Paused": false,
                "Restarting": false,
                "OOMKilled": false,
                "Dead": false,
                "Pid": 3196,
                "ExitCode": 0,
                "Error": "",
                "StartedAt": "2015-11-12T14:54:05.026747471Z",
                "FinishedAt": "0001-01-01T00:00:00Z"
            },
            "Image": "4921c5917fc117df3dec32f4c1976635dc6c56ccd3336fe1db3477f950e78bf7",
            "ResolvConfPath": "/mnt/sda1/var/lib/docker/containers/81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c/resolv.conf",
            "HostnamePath": "/mnt/sda1/var/lib/docker/containers/81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c/hostname",
            "HostsPath": "/mnt/sda1/var/lib/docker/containers/81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c/hosts",
            "LogPath": "/mnt/sda1/var/lib/docker/containers/81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c/81e1bbe20b5508349e1c804eb08b7b6ca8366751dbea9f578b3ea0773fa66c1c-json.log",
            "Node": {
                "ID": "AUIB:LFOT:3LSF:SCFS:OYDQ:NLXD:JZNE:4INI:3DRC:ZFBB:GWCY:DWJK",
                "IP": "192.168.99.121",
                "Addr": "192.168.99.121:2376",
                "Name": "swl-demo1",
                "Cpus": 1,
                "Memory": 2099945472,
                "Labels": {
                    "executiondriver": "native-0.2",
                    "kernelversion": "4.1.12-boot2docker",
                    "operatingsystem": "Boot2Docker 1.9.0 (TCL 6.4); master : 16e4a2a - Tue Nov  3 19:49:22 UTC 2015",
                    "provider": "virtualbox",
                    "storagedriver": "aufs"
                }
            },
            "Name": "/docker-proxy.swl-demo1",
            "RestartCount": 0,
            "Driver": "aufs",
            "ExecDriver": "native-0.2",
            "MountLabel": "",
            "ProcessLabel": "",
            "AppArmorProfile": "",
            "ExecIDs": null,
            "HostConfig": {
                "Binds": null,
                "ContainerIDFile": "",
                "LxcConf": [],
                "Memory": 0,
                "MemoryReservation": 0,
                "MemorySwap": 0,
                "KernelMemory": 0,
                "CpuShares": 0,
                "CpuPeriod": 0,
                "CpusetCpus": "",
                "CpusetMems": "",
                "CpuQuota": 0,
                "BlkioWeight": 0,
                "OomKillDisable": false,
                "MemorySwappiness": -1,
                "Privileged": false,
                "PortBindings": {
                    "443/tcp": [
                        {
                            "HostIp": "",
                            "HostPort": "443"
                        }
                    ]
                },
                "Links": null,
                "PublishAllPorts": false,
                "Dns": null,
                "DnsOptions": null,
                "DnsSearch": null,
                "ExtraHosts": null,
                "VolumesFrom": null,
                "Devices": [],
                "NetworkMode": "swl-net",
                "IpcMode": "",
                "PidMode": "",
                "UTSMode": "",
                "CapAdd": null,
                "CapDrop": null,
                "GroupAdd": null,
                "RestartPolicy": {
                    "Name": "no",
                    "MaximumRetryCount": 0
                },
                "SecurityOpt": null,
                "ReadonlyRootfs": false,
                "Ulimits": null,
                "LogConfig": {
                    "Type": "json-file",
                    "Config": {}
                },
                "CgroupParent": "",
                "ConsoleSize": [
                    0,
                    0
                ],
                "VolumeDriver": ""
            },
            "GraphDriver": {
                "Name": "aufs",
                "Data": null
            },
            "Mounts": [],
            "Config": {
                "Hostname": "81e1bbe20b55",
                "Domainname": "",
                "User": "",
                "AttachStdin": false,
                "AttachStdout": false,
                "AttachStderr": false,
                "ExposedPorts": {
                    "443/tcp": {}
                },
                "Tty": false,
                "OpenStdin": false,
                "StdinOnce": false,
                "Env": [
                    "DOMAIN=local.auto",
                    "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                    "CONSUL_TEMPLATE_VERSION=0.11.1"
                ],
                "Cmd": [
                    "-consul=192.168.99.120:8500"
                ],
                "Image": "docker-proxy:latest",
                "Volumes": null,
                "WorkingDir": "",
                "Entrypoint": [
                    "consul-template",
                    "-config=/tmp/haproxy.json"
                ],
                "OnBuild": null,
                "Labels": {},
                "StopSignal": "SIGTERM"
            },
            "NetworkSettings": {
                "Bridge": "",
                "SandboxID": "c6b903dc5c1a96113a22dbc44709e30194079bd2d262eea1eb4f38d85821f6e1",
                "HairpinMode": false,
                "LinkLocalIPv6Address": "",
                "LinkLocalIPv6PrefixLen": 0,
                "Ports": {
                    "443/tcp": [
                        {
                            "HostIp": "192.168.99.121",
                            "HostPort": "443"
                        }
                    ]
                },
                "SandboxKey": "/var/run/docker/netns/c6b903dc5c1a",
                "SecondaryIPAddresses": null,
                "SecondaryIPv6Addresses": null,
                "EndpointID": "",
                "Gateway": "",
                "GlobalIPv6Address": "",
                "GlobalIPv6PrefixLen": 0,
                "IPAddress": "",
                "IPPrefixLen": 0,
                "IPv6Gateway": "",
                "MacAddress": "",
                "Networks": {
                    "swl-net": {
						"Aliases": [
							"testalias",
							"81e1bbe20b55"
						],
                        "NetworkID": "7ea29fc1412292a2d7bba362f9253545fecdfa8ce9a6e37dd10ba8bee7129812",
                        "EndpointID": "683e3092275782a53c3b0968cc7e3a10f23264022ded9cb20490902f96fc5981",
                        "Gateway": "",
                        "IPAddress": "10.0.0.3",
                        "IPPrefixLen": 24,
                        "IPv6Gateway": "",
                        "GlobalIPv6Address": "",
                        "GlobalIPv6PrefixLen": 0,
                        "MacAddress": "02:42:0a:00:00:03"
                    }
                }
            }
}`

	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "81e1bbe20b55"
	expIP := "10.0.0.3"
	expNetworkID := "7ea29fc1412292a2d7bba362f9253545fecdfa8ce9a6e37dd10ba8bee7129812"
	expectedAliases := []string{"testalias", "81e1bbe20b55"}

	container, err := client.InspectContainer(id)
	if err != nil {
		t.Fatal(err)
	}

	s := reflect.Indirect(reflect.ValueOf(container.NetworkSettings))
	networks := s.FieldByName("Networks")
	if networks.IsValid() {
		var ip string
		for _, net := range networks.MapKeys() {
			if net.Interface().(string) == container.HostConfig.NetworkMode {
				ip = networks.MapIndex(net).FieldByName("IPAddress").Interface().(string)
				t.Logf("%s %v", net, ip)
			}
		}
		if ip != expIP {
			t.Errorf("InspectContainerNetworks(%q): Expected %#v. Got %#v.", id, expIP, ip)
		}

		var networkID string
		for _, net := range networks.MapKeys() {
			if net.Interface().(string) == container.HostConfig.NetworkMode {
				networkID = networks.MapIndex(net).FieldByName("NetworkID").Interface().(string)
				t.Logf("%s %v", net, networkID)
			}
		}

		var aliases []string
		for _, net := range networks.MapKeys() {
			if net.Interface().(string) == container.HostConfig.NetworkMode {
				aliases = networks.MapIndex(net).FieldByName("Aliases").Interface().([]string)
			}
		}
		if !reflect.DeepEqual(aliases, expectedAliases) {
			t.Errorf("InspectContainerNetworks(%q): Expected Aliases %#v. Got %#v.", id, expectedAliases, aliases)
		}

		if networkID != expNetworkID {
			t.Errorf("InspectContainerNetworks(%q): Expected %#v. Got %#v.", id, expNetworkID, networkID)
		}
	} else {
		t.Errorf("InspectContainerNetworks(%q): No method Networks for NetworkSettings", id)
	}
}

func TestInspectContainerNegativeSwap(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
             "Created": "2013-05-07T14:51:42.087658+02:00",
             "Path": "date",
             "Args": [],
             "Config": {
                     "Hostname": "4fa6e0f0c678",
                     "User": "",
                     "Memory": 17179869184,
                     "MemorySwap": -1,
                     "AttachStdin": false,
                     "AttachStdout": true,
                     "AttachStderr": true,
                     "PortSpecs": null,
                     "Tty": false,
                     "OpenStdin": false,
                     "StdinOnce": false,
                     "Env": null,
                     "Cmd": [
                             "date"
                     ],
                     "Image": "base",
                     "Volumes": {},
                     "VolumesFrom": ""
             },
             "State": {
                     "Running": false,
                     "Pid": 0,
                     "ExitCode": 0,
                     "StartedAt": "2013-05-07T14:51:42.087658+02:00",
                     "Ghost": false
             },
             "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "NetworkSettings": {
                     "IpAddress": "",
                     "IpPrefixLen": 0,
                     "Gateway": "",
                     "Bridge": "",
                     "PortMapping": null
             },
             "SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
             "ResolvConfPath": "/etc/resolv.conf",
             "Volumes": {},
             "HostConfig": {
               "Binds": null,
               "ContainerIDFile": "",
               "LxcConf": [],
               "Privileged": false,
               "PortBindings": {
                 "80/tcp": [
                   {
                     "HostIp": "0.0.0.0",
                     "HostPort": "49153"
                   }
                 ]
               },
               "Links": null,
               "PublishAllPorts": false
             }
}`
	var expected Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c678"
	container, err := client.InspectContainer(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*container, expected) {
		t.Errorf("InspectContainer(%q): Expected %#v. Got %#v.", id, expected, container)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/json"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestInspectContainerFailure(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "server error", status: 500})
	expected := Error{Status: 500, Message: "server error"}
	container, err := client.InspectContainer("abe033")
	if container != nil {
		t.Errorf("InspectContainer: Expected <nil> container, got %#v", container)
	}
	if !reflect.DeepEqual(expected, *err.(*Error)) {
		t.Errorf("InspectContainer: Wrong error information. Want %#v. Got %#v.", expected, err)
	}
}

func TestInspectContainerNotFound(t *testing.T) {
	t.Parallel()
	const containerID = "abe033"
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: 404})
	container, err := client.InspectContainer(containerID)
	if container != nil {
		t.Errorf("InspectContainer: Expected <nil> container, got %#v", container)
	}
	expectNoSuchContainer(t, containerID, err)
}

func TestInspectContainerWhenContextTimesOut(t *testing.T) {
	t.Parallel()
	rt := sleepyRoudTripper{sleepDuration: 200 * time.Millisecond}

	client := newTestClient(&rt)

	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()

	_, err := client.InspectContainerWithContext("id", ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected 'DeadlineExceededError', got: %v", err)
	}
}
