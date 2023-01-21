package networking

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/pointer"
)

const (
	gcpSourceImage         = "/projects/centos-cloud/global/images/family/centos-stream-8"
	gcpMachineType         = "/projects/%s/zones/%s/machineTypes/n1-standard-2"
	gcpVMUser              = "cloud-user"
	gcpInstanceStartupDone = "gcp-instance-startup-done-marker"
)

var (
	gcpFirewallSourceRanges = []string{"10.0.0.0/8"}
	gcpOperationTimeout     = time.Minute * 10
)

// gcpCloudClient implements interface cloudClient for GCP.
type gcpCloudClient struct {
	oc            *exutil.CLI
	ctx           context.Context
	googleService *compute.Service
	project       string
	zone          string
	subnet        string
	network       string
}

// newGCPCloudClient initializes and returns a new gcpCloudClient.
func newGCPCloudClient(oc *exutil.CLI) (*gcpCloudClient, error) {
	client := &gcpCloudClient{oc: oc, ctx: context.Background()}
	err := client.initCloudSecret()
	if err != nil {
		return nil, err
	}

	var instance *compute.Instance
	client.project, client.zone, instance, err = client.getInstance()
	if err != nil {
		return nil, err
	}
	for _, intf := range instance.NetworkInterfaces {
		client.subnet = intf.Subnetwork
		client.network = intf.Network
	}

	return client, nil
}

// gcpReadCloudSecret reads the JSON cloud secret for GCP from Secret cloud-credentials in namespace
// openshift-cloud-network-config-controller and initializes g.googleService.
func (g *gcpCloudClient) initCloudSecret() error {
	ctx, cancel := context.WithTimeout(g.ctx, gcpOperationTimeout)
	defer cancel()

	secretField := "service_account.json"
	data, err := readCloudSecret(g.oc, secretNamespace, secretName)
	if err != nil {
		return err
	}
	credentials, ok := data[secretField]
	if !ok {
		return fmt.Errorf("cloud secret does not have expected field %s", secretField)
	}
	c, err := gcpInitCredentials(ctx, string(credentials))
	if err != nil {
		return err
	}
	g.googleService = c
	return nil
}

// createVM creates a GCP VirtualMachine according to the specs provided by the vm object. It first creates the
// instance, followed by the firewall. Next, it populates the provided vm's publicIP and privateIP addresses.
func (g *gcpCloudClient) createVM(vm *vm, requestPublicIP bool) error {
	ctx, cancel := context.WithTimeout(g.ctx, gcpOperationTimeout)
	defer cancel()

	framework.Logf("Creating VM %s", vm.name)
	loadedInstance, err := g.createInstance(ctx, vm, requestPublicIP)
	if err != nil {
		return err
	}

	framework.Logf("Creating firewall for VM %s", vm.name)
	err = g.createFirewall(vm.name, vm.ports)
	if err != nil {
		return err
	}

	framework.Logf("Extracting public and private IP info from instance %s", vm.name)
	var privateIP string
	var publicIP string
	for _, intf := range loadedInstance.NetworkInterfaces {
		privateIP = intf.NetworkIP
		for _, accessConfig := range intf.AccessConfigs {
			publicIP = accessConfig.NatIP
			break
		}
		break
	}
	vm.privateIP = net.ParseIP(privateIP)
	vm.publicIP = net.ParseIP(publicIP)
	if vm.privateIP == nil {
		return fmt.Errorf("created instance does not have a valid private IP address")
	}

	return err
}

// deleteVM deletes the GCP Virtual Machine that matches the provided vm object. It first deletes the associated
// Firewall followed by the Instance.
func (g *gcpCloudClient) deleteVM(vm *vm) error {
	ctx, cancel := context.WithTimeout(g.ctx, gcpOperationTimeout)
	defer cancel()

	framework.Logf("Deleting VM %s firewall", vm.name)
	_, err := g.googleService.Firewalls.Delete(g.project, vm.name).Do()
	if parseGCPDeleteError(err) != nil {
		return err
	}

	framework.Logf("Deleting VM %s", vm.name)
	op, err := g.googleService.Instances.Delete(g.project, g.zone, vm.name).Do()
	if parseGCPDeleteError(err) != nil {
		return err
	}
	if err == nil {
		framework.Logf("Waiting for VM %s deletion to finish", vm.name)
		err = g.waitForOperation(ctx, op)
		if parseGCPDeleteError(err) != nil {
			return err
		}
	}

	return nil
}

// Close implements the cloudClient interface method of the same name and implementes the Closer interface. In GCP,
// this is a noop.
func (g *gcpCloudClient) Close() error {
	return nil
}

// createFirewall creates a GCP firewall rule that allows <ports> traffic from global gcpFirewallSourceRanges to
// instances marked with <tag>.
func (g *gcpCloudClient) createFirewall(tag string, ports map[string]protocolPort) error {
	portsPerProtocol := make(map[string][]string)
	for _, protocolPort := range ports {
		portsPerProtocol[strings.ToLower(protocolPort.protocol)] = append(
			portsPerProtocol[protocolPort.protocol],
			strconv.Itoa(protocolPort.port))
	}
	var firewallAllowed []*compute.FirewallAllowed
	for protocol, ports := range portsPerProtocol {
		firewallAllowed = append(firewallAllowed, &compute.FirewallAllowed{
			IPProtocol: protocol,
			Ports:      ports,
		})
	}
	firewall := &compute.Firewall{
		Allowed:      firewallAllowed,
		Name:         tag,
		Network:      g.network,
		SourceRanges: gcpFirewallSourceRanges,
		TargetTags:   []string{tag},
	}
	_, err := g.googleService.Firewalls.Insert(g.project, firewall).Do()
	return err
}

// createInstance takes a context with a timeout and a VM definition and spawns a GCP VirtualMachine. It then waits
// for the operation to finish. It repeatedly reads the GCP console log until it finds string gcpInstanceStartupDone.
// Finally, it retrieves the latest state of the VirtualMachine and returns it.
func (g *gcpCloudClient) createInstance(ctx context.Context, vm *vm, requestPublicIP bool) (*compute.Instance, error) {
	framework.Logf("Creating instance %s", vm.name)
	startupScript := fmt.Sprintf("%s\necho \"%s\" > /dev/ttyS0\n",
		printp(vm.startupScript, vm.startupScriptParameters),
		gcpInstanceStartupDone,
	)
	machineType := fmt.Sprintf(gcpMachineType, g.project, g.zone)
	var accessConfigs []*compute.AccessConfig
	if requestPublicIP {
		accessConfigs = []*compute.AccessConfig{
			{
				Name: "External NAT",
				Type: "ONE_TO_ONE_NAT",
			},
		}
	}
	newInstance := &compute.Instance{
		MachineType: machineType,
		Name:        vm.name,
		Tags: &compute.Tags{
			Items: []string{
				vm.name,
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network:       g.network,
				Subnetwork:    g.subnet,
				AccessConfigs: accessConfigs,
			},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: gcpSourceImage,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "startup-script",
					Value: &startupScript,
				},
				{
					Key:   "ssh-keys",
					Value: pointer.String(fmt.Sprintf("%s:%s", gcpVMUser, vm.sshPublicKey)),
				},
			},
		},
	}
	op, err := g.googleService.Instances.Insert(g.project, g.zone, newInstance).Do()
	if err != nil {
		return nil, err
	}

	framework.Logf("Waiting for VM %s creation to finish", vm.name)
	err = g.waitForOperation(ctx, op)
	if err != nil {
		return nil, err
	}

	framework.Logf("Waiting until startup script finishes for VM %s", vm.name)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
outer:
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("startup script never finished or never posted status to console"+
				"for VM %s", vm.name)
		case <-ticker.C:
			framework.Logf("Reading serial port output of VM %s", vm.name)
			serialPortOutput, err := g.googleService.Instances.GetSerialPortOutput(g.project, g.zone, vm.name).Do()
			if err != nil {
				return nil, err
			}
			if strings.Contains(serialPortOutput.Contents, gcpInstanceStartupDone) {
				framework.Logf("Found string %s in VM %s logs", gcpInstanceStartupDone, vm.name)
				break outer
			}
			framework.Logf("Could not find string %s in VM %s logs", gcpInstanceStartupDone, vm.name)
		}
	}

	framework.Logf("Retrieving latest copy of VM %s", vm.name)
	return g.googleService.Instances.Get(g.project, g.zone, vm.name).Do()
}

// waitForOperation waits for the compute operation to finish.
// Copied from https://github.com/googleapis/google-cloud-go/issues/178.
func (g *gcpCloudClient) waitForOperation(ctx context.Context, op *compute.Operation) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for operation to complete")
		case <-ticker.C:
			result, err := g.googleService.ZoneOperations.Get(g.project, g.zone, op.Name).Do()
			if err != nil {
				return fmt.Errorf("ZoneOperations.Get: %s", err)
			}

			if result.Status == "DONE" {
				if result.Error != nil {
					var errors []string
					for _, e := range result.Error.Errors {
						errors = append(errors, e.Message)
					}
					return fmt.Errorf("operation %q failed with error(s): %s", op.Name, strings.Join(errors, ", "))
				}
				return nil
			}
		}
	}
}

// gcpGetInstance retrieves the GCP instance referred by the Node object.
// returns the project and zone name as well.
//   This is what the node's providerID looks like on GCP
//      spec:
//    providerID: gce://openshift-gce-devel-ci/us-east1-b/ci-ln-pvr3lyb-f76d1-6w8mm-master-0
//   i.e: projectID/zone/instanceName
//
// split out and return these components
func (g *gcpCloudClient) getInstance() (project string, zone string, instance *compute.Instance, err error) {
	providerID, err := getWorkerProviderID(g.oc.AsAdmin())
	if err != nil {
		return
	}
	u, err := url.Parse(providerID)
	if err != nil {
		err = fmt.Errorf("failed to parse node provider id %s: %w", providerID, err)
		return
	}
	parts := strings.SplitN(u.Path, "/", 3)
	if len(parts) != 3 {
		err = fmt.Errorf("failed to parse provider id %s: expected two path components", providerID)
		return
	}
	project = u.Host
	zone = parts[1]
	instanceStr := parts[2]

	instance, err = g.googleService.Instances.Get(project, zone, instanceStr).Do()
	if err != nil {
		return "", "", nil, err
	}
	return
}

// gcpInitCredentials takes a raw secret (in JSON format) and returns an initialized google.Service.
func gcpInitCredentials(ctx context.Context, rawSecretData string) (*compute.Service, error) {
	opts := []option.ClientOption{
		option.WithCredentialsJSON([]byte(rawSecretData)),
		option.WithUserAgent(userAgent),
	}

	return compute.NewService(ctx, opts...)
}

// parseGCPDeleteError returns the provided error unless it's an error of type *googleapi.Error with a Code of 404.
// In that case, it will return nil.
func parseGCPDeleteError(err error) error {
	if ge, ok := err.(*googleapi.Error); ok {
		if ge.Code == 404 {
			return nil
		}
	}
	return err
}
