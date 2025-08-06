// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	vCenterHostGatewaySocket    = "/var/run/envoy-hgw/hgw-pipe"
	vCenterHostGatewaySocketEnv = "VCENTER_ENVOY_HOST_GATEWAY"
)

// InventoryPath composed of entities by Name
func InventoryPath(entities []mo.ManagedEntity) string {
	val := "/"

	for _, entity := range entities {
		// Skip root folder in building inventory path.
		if entity.Parent == nil {
			continue
		}
		val = path.Join(val, entity.Name)
	}

	return val
}

var vsanFS = []string{
	string(types.HostFileSystemVolumeFileSystemTypeVsan),
	string(types.HostFileSystemVolumeFileSystemTypeVVOL),
}

func IsDatastoreVSAN(ds mo.Datastore) bool {
	return slices.Contains(vsanFS, ds.Summary.Type)
}

func HostSystemManagementIPs(config []types.VirtualNicManagerNetConfig) []net.IP {
	var ips []net.IP

	for _, nc := range config {
		if nc.NicType != string(types.HostVirtualNicManagerNicTypeManagement) {
			continue
		}
		for ix := range nc.CandidateVnic {
			for _, selectedVnicKey := range nc.SelectedVnic {
				if nc.CandidateVnic[ix].Key != selectedVnicKey {
					continue
				}
				ip := net.ParseIP(nc.CandidateVnic[ix].Spec.Ip.IpAddress)
				if ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	return ips
}

// UsingEnvoySidecar determines if the given *vim25.Client is using vCenter's
// local Envoy sidecar (as opposed to using the HTTPS port.)
// Returns a boolean indicating whether to use the sidecar or not.
func UsingEnvoySidecar(c *vim25.Client) bool {
	envoySidecarPort := os.Getenv("GOVMOMI_ENVOY_SIDECAR_PORT")
	if envoySidecarPort == "" {
		envoySidecarPort = "1080"
	}
	envoySidecarHost := os.Getenv("GOVMOMI_ENVOY_SIDECAR_HOST")
	if envoySidecarHost == "" {
		envoySidecarHost = "localhost"
	}
	return c.URL().Hostname() == envoySidecarHost && c.URL().Scheme == "http" && c.URL().Port() == envoySidecarPort
}

// ClientWithEnvoyHostGateway clones the provided soap.Client and returns a new
// one that uses a Unix socket to leverage vCenter's local Envoy host
// gateway.
// This should be used to construct clients that talk to ESX.
// This method returns a new *vim25.Client and does not modify the original input.
// This client disables HTTP keep alives and is intended for a single round
// trip. (eg. guest file transfer, datastore file transfer)
func ClientWithEnvoyHostGateway(vc *vim25.Client) *vim25.Client {
	// Override the vim client with a new one that wraps a Unix socket transport.
	// Using HTTP here so secure means nothing.
	sc := soap.NewClient(vc.URL(), true)
	// Clone the underlying HTTP transport, only replacing the dialer logic.
	transport := sc.DefaultTransport().Clone()
	hostGatewaySocketPath := os.Getenv(vCenterHostGatewaySocketEnv)
	if hostGatewaySocketPath == "" {
		hostGatewaySocketPath = vCenterHostGatewaySocket
	}
	transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", hostGatewaySocketPath)
	}
	// We use this client for a single request, so we don't require keepalives.
	transport.DisableKeepAlives = true
	sc.Client = http.Client{
		Transport: transport,
	}
	newVC := &vim25.Client{
		Client: sc,
	}
	return newVC
}

// HostGatewayTransferURL rewrites the provided URL to be suitable for use
// with the Envoy host gateway on vCenter.
// It returns a copy of the provided URL with the host, scheme rewritten as needed.
// Receivers of such URLs must typically also use ClientWithEnvoyHostGateway to
// use the appropriate http.Transport to be able to make use of the host
// gateway.
// nil input yields an uninitialized struct.
func HostGatewayTransferURL(u *url.URL, hostMoref types.ManagedObjectReference) *url.URL {
	if u == nil {
		return &url.URL{}
	}
	// Make a copy of the provided URL.
	turl := *u
	turl.Host = "localhost"
	turl.Scheme = "http"
	oldPath := turl.Path
	turl.Path = fmt.Sprintf("/hgw/%s%s", hostMoref.Value, oldPath)
	return &turl
}

func (arg ReflectManagedMethodExecuterSoapArgument) Value() []string {
	if arg.Val == "" {
		return nil
	}

	d := xml.NewDecoder(strings.NewReader(arg.Val))
	var val []string

	for {
		t, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		if c, ok := t.(xml.CharData); ok {
			val = append(val, string(c))
		}
	}

	return val
}

func EsxcliName(name string) string {
	return strings.ReplaceAll(strings.Title(name), ".", "")
}

// OID returns a stable UUID based on input s
func OID(s string) uuid.UUID {
	return uuid.NewHash(sha256.New(), uuid.NameSpaceOID, []byte(s), 5)
}
