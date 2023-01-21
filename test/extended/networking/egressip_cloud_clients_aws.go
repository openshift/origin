package networking

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	exutil "github.com/openshift/origin/test/extended/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/pointer"
)

const (
	awsImageName                       = "RHEL-8"
	awsCustomSecGroupSourceRange       = "10.0.0.0/8"
	awsDefaultApplicationPort          = 30000
	awsInstanceStartupDone             = "aws-instance-startup-done-marker"
	awsVMUser                          = "ec2-user"
	openshiftCloudCredentialOperatorNS = "openshift-cloud-credential-operator"
	awsCredentialsRequestTemplate      = `apiVersion: cloudcredential.openshift.io/v1
kind: CredentialsRequest
metadata:
  annotations:
  name: %[2]s
  namespace: openshift-cloud-credential-operator
spec:
  providerSpec:
    apiVersion: cloudcredential.openshift.io/v1
    kind: AWSProviderSpec
    statementEntries:
    - action:
      - ec2:CreateSecurityGroup
      - ec2:DeleteSecurityGroup
      - ec2:AuthorizeSecurityGroupIngress
      - ec2:ImportKeyPair
      - ec2:DeleteKeyPair
      - ec2:CreateTags
      - ec2:DescribeAvailabilityZones
      - ec2:DescribeDhcpOptions
      - ec2:DescribeImages
      - ec2:DescribeInstances
      - ec2:DescribeInstanceTypes
      - ec2:DescribeInternetGateways
      - ec2:DescribeSecurityGroups
      - ec2:DescribeRegions
      - ec2:DescribeSubnets
      - ec2:DescribeVpcs
      - ec2:RunInstances
      - ec2:TerminateInstances
      - ec2:AllocateAddress
      - ec2:AssociateAddress
      - ec2:ReleaseAddress
      - ec2:GetConsoleOutput
      effect: Allow
      resource: '*'
  secretRef:
    name: %[2]s
    namespace: %[1]s`
)

var (
	// awsOperationTimeout is the maximum duration for select operations that take a long time such as initCloudSecret
	// and createVM.
	awsOperationTimeout = time.Minute * 10
)

// awsCloudClient implements interface cloudClient for AWS.
type awsCloudClient struct {
	oc              *exutil.CLI
	ctx             context.Context
	client          *ec2.EC2
	region          string
	instance        *ec2.Instance
	secretNamespace string
	secretName      string
}

// newAWSCloudClient initializes and returns a new awsCloudClient. It will also create a new namespace and secret
// to hold AWS cloud credentials. The name of both the namespace and the secret will be "e2e-egressip-" + random UUID.
// In case of an error, this will always return an object of type awsCloudClient and never nil. This is to avoid nil
// pointer references during teardown.
func newAWSCloudClient(oc *exutil.CLI) (*awsCloudClient, error) {
	identifier := fmt.Sprintf("e2e-egressip-%s", uuid.New())
	client := &awsCloudClient{
		oc:              oc,
		ctx:             context.Background(),
		secretNamespace: identifier,
		secretName:      identifier,
	}

	// Initialize the ec2.EC2 client client.client.
	err := client.initCloudSecret()
	if err != nil {
		return client, err
	}

	// Set client.instance to the EC2 Instance for the first worker node that we can find. It does not matter which
	// worker node we get, as we are interested in generic info such as the network/subnet or the security groups.
	// Retry on Error as we might get "AuthFailure" - AWS sometimes takes a while to accept the new credentials even
	// though all resources on the OCP side were created correctly.
	err = retry.OnError(
		wait.Backoff{
			Steps:    10,
			Duration: time.Second,
			Factor:   2.0,
			Jitter:   0.1,
		},
		// Retry on "AuthFailure" and "UnauthorizedOperation" only.
		func(err error) bool {
			ae, ok := err.(awserr.Error)
			if !ok {
				return false
			}
			if ae.Code() == "AuthFailure" {
				framework.Logf("Received auth failure, backing off and trying again")
				return true
			}
			if ae.Code() == "UnauthorizedOperation" {
				framework.Logf("Received UnauthorizedOperation failure, backing off and trying again")
				return true
			}
			return false
		},
		func() error {
			instance, err := client.getInstance()
			if err != nil {
				return err
			}
			client.instance = instance
			return nil
		})
	return client, err
}

// initCloudSecret creates a CredentialsRequest in a.secretNamespace, then reads the AWS cloud secret that was created
// and initialiazes a.client with it. initCloudSecret must finish before awsOperationTimeout is up.
func (a *awsCloudClient) initCloudSecret() error {
	ctx, cancel := context.WithTimeout(a.ctx, awsOperationTimeout)
	defer cancel()

	framework.Logf("Creating CredentialsRequest with the permissions that are needed")
	err := a.createCredentialsRequest()
	if err != nil {
		return err
	}
	framework.Logf("Reading cloud secret %s/%s", a.secretNamespace, a.secretName)
	data, err := readCloudSecret(a.oc, a.secretNamespace, a.secretName)
	if err != nil {
		return err
	}
	for _, key := range []string{"aws_access_key_id", "aws_secret_access_key"} {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("could not find required key %s in cloud secret", key)
		}
	}

	framework.Logf("Getting region")
	region, err := a.getRegion()
	if err != nil {
		return err
	}
	a.region = region

	framework.Logf("Initializing cloud credentials")
	c, err := awsInitCredentials(ctx, string(data["aws_access_key_id"]), string(data["aws_secret_access_key"]),
		a.region)
	if err != nil {
		return err
	}
	a.client = c
	return nil
}

// getRegion reads the AWS region from the Infrastructure CR named "cluster".
func (a *awsCloudClient) getRegion() (string, error) {
	infra, err := a.oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster",
		metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return infra.Status.PlatformStatus.AWS.Region, nil
}

// getInstance returns the EC2 Instance for the first worker node that it can find. It does not matter which
// worker node we get, as we are interested in generic info such as the network/subnet or the security groups.
func (a *awsCloudClient) getInstance() (*ec2.Instance, error) {
	providerID, err := getWorkerProviderID(a.oc.AsAdmin())
	if err != nil {
		return nil, err
	}
	splitProviderID := strings.Split(strings.TrimPrefix(providerID, "aws://"), "/")
	if len(splitProviderID) > 3 {
		return nil, fmt.Errorf("could not parse providerID %s", providerID)
	}
	instanceID := splitProviderID[len(splitProviderID)-1]
	if instanceID == "" {
		return nil, fmt.Errorf("could not parse providerID %s", providerID)
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}
	result, err := a.client.DescribeInstances(input)
	if err != nil {
		// Do never rewrite this error with fmt.Errorf or it will break the retry in newAWSCloudClient.
		return nil, err
	}
	instances := []*ec2.Instance{}
	for _, reservation := range result.Reservations {
		instances = append(instances, reservation.Instances...)
	}
	if len(instances) != 1 {
		return nil, fmt.Errorf("found conflicting instance replicas for %s, instances: %v", instanceID, instances)
	}
	return instances[0], nil
}

// createVM implements the cloudClient interface method of the same name. It is responsible for creating a SecurityGroup,
// Public IP, Instance and for running the startup script. createVM must finish before awsOperationTimeout is up.
func (a *awsCloudClient) createVM(vm *vm, requestPublicIP bool) error {
	ctx, cancel := context.WithTimeout(a.ctx, awsOperationTimeout)
	defer cancel()

	// Create a securityGroup if needed - port 30000 is always open for worker nodes. Therefore, in such a case, skip
	// securityGroup creation if only 30000 shall be opened.
	var err error
	var securityGroupIDs []*string
	for _, g := range a.instance.SecurityGroups {
		if g.GroupId != nil {
			securityGroupIDs = append(securityGroupIDs, g.GroupId)
		}
	}
	if len(vm.ports) > 0 {
		p, ok := vm.ports[applicationPort]
		if len(vm.ports) > 1 || !ok || p.protocol != tcp || p.port != awsDefaultApplicationPort {
			framework.Logf("Spawning security group for VM %s", vm.name)
			g, err := a.createSecurityGroup(vm, requestPublicIP)
			if err != nil {
				return err
			}
			securityGroupIDs = []*string{&g}
			vm.securityGroupID = g
		}
	}

	framework.Logf("Spawning instance for VM %s", vm.name)
	instance, err := a.createInstance(ctx, vm, securityGroupIDs)
	if err != nil {
		return err
	}
	framework.Logf("Getting the instance ID")
	if instance.InstanceId == nil || *instance.InstanceId == "" {
		return fmt.Errorf("could not get VM %s instanceID", vm.name)
	}
	vm.id = *instance.InstanceId

	framework.Logf("Getting the instance's private IP addresses")
	interfaces := instance.NetworkInterfaces
	var privateIP *string
	var interfaceID *string
	for _, intf := range interfaces {
		interfaceID = intf.NetworkInterfaceId
		privateIP = intf.PrivateIpAddress
		break
	}
	if privateIP != nil {
		vm.privateIP = net.ParseIP(*privateIP)
	}
	if vm.privateIP == nil {
		return fmt.Errorf("could not get VM %s privateIP", vm.name)
	}

	if requestPublicIP {
		framework.Logf("Creating an ElasticIP (publicIP)")
		allocateInput := &ec2.AllocateAddressInput{
			Domain: a.instance.VpcId,
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("elastic-ip"),
					Tags: []*ec2.Tag{
						{
							Key:   pointer.String("Name"),
							Value: &vm.name,
						},
					},
				},
			},
		}
		allocateOutput, err := a.client.AllocateAddress(allocateInput)
		if err != nil {
			return fmt.Errorf("could not create ElasticIP, err: %q", err)
		}
		vm.publicIPID = *allocateOutput.AllocationId

		framework.Logf("Associating ElasticIP to instance")
		associateAddressInput := &ec2.AssociateAddressInput{
			InstanceId:         instance.InstanceId,
			NetworkInterfaceId: interfaceID,
			PrivateIpAddress:   privateIP,
			AllocationId:       allocateOutput.AllocationId,
		}
		_, err = a.client.AssociateAddress(associateAddressInput)
		if err != nil {
			return fmt.Errorf("could not associate ElasticIP address, err: %q", err)
		}
		vm.publicIP = net.ParseIP(*allocateOutput.PublicIp)
	}

	return nil
}

// deleteVM implements the cloudClient interface method of the same name. It is responsible for tearing down all
// resources that were created during the createVM stage.
func (a *awsCloudClient) deleteVM(vm *vm) error {
	framework.Logf("Deleting VM %s with ID %s", vm.name, vm.id)
	if vm.id == "" {
		return fmt.Errorf("could not delete VM %q, invalid instanceId %q", vm.name, vm.id)
	}
	_, err := a.client.TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{&vm.id}})
	if err == nil {
		framework.Logf("Waiting until VM %s instance %s is terminated", vm.name, vm.id)
		descInstancesInput := &ec2.DescribeInstancesInput{InstanceIds: []*string{&vm.id}}
		err = a.client.WaitUntilInstanceTerminated(descInstancesInput)
		if err != nil {
			return fmt.Errorf("error while waiting until instance terminated, err: %q", err)
		}
	} else if parseAWSDeleteError(err) != nil {
		return err
	}

	if vm.publicIP != nil {
		framework.Logf("Releasing the ElasticIP for VM %s", vm.name)
		releaseAddressInput := &ec2.ReleaseAddressInput{
			AllocationId: &vm.publicIPID,
		}
		_, err := a.client.ReleaseAddress(releaseAddressInput)
		if parseAWSDeleteError(err) != nil {
			return fmt.Errorf("could not release ElasticIP, err: %q", err)
		}
	}

	if vm.securityGroupID != "" {
		framework.Logf("Deleting security group for VM %s", vm.name)
		deleteSecGroupInput := &ec2.DeleteSecurityGroupInput{
			GroupId: &vm.securityGroupID,
		}
		_, err = a.client.DeleteSecurityGroup(deleteSecGroupInput)
		if parseAWSDeleteError(err) != nil {
			return fmt.Errorf("could not delete security group, err: %q", err)
		}
	}

	framework.Logf("Deleting KeyPair VM %s with ID %s", vm.name, vm.keyPairID)
	deleteKeyPairInput := &ec2.DeleteKeyPairInput{
		KeyPairId: &vm.keyPairID,
	}
	_, err = a.client.DeleteKeyPair(deleteKeyPairInput)
	if parseAWSDeleteError(err) != nil {
		return fmt.Errorf("could not delete keypair, err: %q", err)
	}
	return nil
}

// Close implements the cloudClient interface and io.Closer interface method of the same name. In AWS,
// this cleans up the CloudCredentialsRequest and corresponding namespace that were created during initialization.
func (a *awsCloudClient) Close() error {
	framework.Logf("Closing awsCloudClient for %s/%s", a.secretNamespace, a.secretName)
	err := a.deleteCredentialsRequest()
	if err != nil {
		return err
	}
	return nil
}

// createCredentialsRequest creates a CredentialsRequest on AWS. The CredentialsRequest will create a cloud secret
// that we can then use to talk to AWS with the necessary permissions. Contrary to the other big cloud platforms,
// existing secrets are fairly limited in scope for AWS. Given that we need some very specific permissions to list
// images, create security groups, and so on, we must create our own CredentialsRequest. If you want to see what we
// are requesting, see constant awsCredentialsRequestTemplate.
func (a *awsCloudClient) createCredentialsRequest() error {
	framework.Logf("Creating namespace %s", a.secretNamespace)
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.secretNamespace,
		},
	}
	_, err := a.oc.AsAdmin().KubeClient().CoreV1().Namespaces().Create(a.ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	framework.Logf("Creating temporary file to store CredentialsRequest")
	credentialsRequest := fmt.Sprintf(awsCredentialsRequestTemplate, a.secretNamespace, a.secretName)
	f, err := os.CreateTemp("", a.secretName)
	if err != nil {
		return fmt.Errorf("could not create tempfile, err: %q", err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	if _, err := f.Write([]byte(credentialsRequest)); err != nil {
		return fmt.Errorf("could not write to file %s, err: %q", f.Name(), err)
	}

	framework.Logf("Applying CredentialsRequest at path %s", f.Name())
	out, err := a.oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", f.Name()).Output()
	if err != nil {
		return fmt.Errorf("could not apply CredentialsRequest, out: %q, err: %q", out, err)
	}

	framework.Logf("Waiting for credentials Secret %s/%s to be created", a.secretNamespace, a.secretName)
	return retry.OnError(
		wait.Backoff{
			Steps:    5,
			Duration: time.Second,
			Factor:   2.0,
			Jitter:   0.1,
		},
		func(err error) bool {
			if errors.IsNotFound(err) {
				framework.Logf("Secret %s/%s not yet created, retrying", a.secretNamespace, a.secretName)
				return true
			}
			return false
		},
		func() error {
			_, err := a.oc.AsAdmin().KubeClient().CoreV1().Secrets(a.secretNamespace).Get(a.ctx, a.secretName,
				metav1.GetOptions{})
			return err
		})
}

// deleteCredentialsRequest deletes the CredentialsRequest and namespace which were created during initialization.
func (a *awsCloudClient) deleteCredentialsRequest() error {
	framework.Logf("Deleting Credentials Request %q", a.secretName)
	out, err := a.oc.AsAdmin().WithoutNamespace().Run("delete").
		Args("CredentialsRequest", "-n", openshiftCloudCredentialOperatorNS, a.secretName).Output()
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("could not delete CredentialsRequest, out: %q, err: %q", out, err)
	}

	framework.Logf("Deleting namespace %s", a.secretNamespace)
	err = a.oc.AsAdmin().KubeClient().CoreV1().Namespaces().Delete(a.ctx, a.secretNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("could not delete namespace %s, err: %q", a.secretNamespace, err)
	}
	return nil
}

// createInstance creates the AWS instance for the target host. The instance will run a startup script. After the
// script terminates, we echo string awsInstanceStartupDone to /dev/ttyS0. We constantly monitor the console log for
// the presence of this string. As soon as the string was found, we know that the startup script ran to completion.
func (a *awsCloudClient) createInstance(ctx context.Context, vm *vm, securityGroupIDs []*string) (*ec2.Instance, error) {
	framework.Logf("Importing SSH keypair")
	importKeyPairInput := &ec2.ImportKeyPairInput{
		KeyName:           aws.String(vm.name),
		PublicKeyMaterial: []byte(vm.sshPublicKey),
	}
	importKeyPairOutput, err := a.client.ImportKeyPair(importKeyPairInput)
	if err != nil {
		return nil, fmt.Errorf("could not create keypair for VM %s, err: %q", vm.name, err)
	}
	vm.keyPairID = *importKeyPairOutput.KeyPairId

	framework.Logf("Getting %s image ID", awsImageName)
	imageID, err := a.getImageID()
	if err != nil {
		return nil, err
	}
	framework.Logf("Found image ID %s", imageID)

	framework.Logf("Creating instance for VM %s", vm.name)
	startupScript := fmt.Sprintf("%s\necho \"%s\" > /dev/ttyS0\n",
		printp(vm.startupScript, vm.startupScriptParameters),
		awsInstanceStartupDone,
	)
	input := &ec2.RunInstancesInput{
		ImageId:          &imageID,
		InstanceType:     a.instance.InstanceType,
		MinCount:         aws.Int64(1),
		MaxCount:         aws.Int64(1),
		SubnetId:         a.instance.SubnetId,
		SecurityGroupIds: securityGroupIDs,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   pointer.String("Name"),
						Value: &vm.name,
					},
				},
			},
		},
		UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(startupScript))),
		KeyName:  &vm.name,
	}
	framework.Logf("AWS RunInstances input: %v", *input)
	reservation, err := a.client.RunInstances(input)
	if err != nil {
		return nil, err
	}
	instance := reservation.Instances[0]

	framework.Logf("Waiting until VM %s instance %s is running", vm.name, *instance.InstanceId)
	descInstancesInput := &ec2.DescribeInstancesInput{InstanceIds: []*string{instance.InstanceId}}
	err = a.client.WaitUntilInstanceRunning(descInstancesInput)
	if err != nil {
		return instance, fmt.Errorf("error while waiting until instance running, err: %q", err)
	}

	framework.Logf("Waiting until user-data script is done running")
	counter := 0
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
outer:
	for {
		select {
		case <-ctx.Done():
			return instance, fmt.Errorf("startup script never finished or never posted status to console for VM %s",
				vm.name)
		case <-ticker.C:
			counter++
			framework.Logf("Reading serial port output of VM %s", vm.name)
			gi := &ec2.GetConsoleOutputInput{InstanceId: instance.InstanceId, Latest: aws.Bool(true)}
			output, err := a.client.GetConsoleOutput(gi)
			if err != nil {
				return instance, err
			}
			consoleOut, err := base64.StdEncoding.DecodeString(aws.StringValue(output.Output))
			if err != nil {
				framework.Logf("Could not decode VM %s console logs, err:\n%q", vm.name, err)
				continue
			}
			if strings.Contains(string(consoleOut), awsInstanceStartupDone) {
				framework.Logf("Found string %s in VM %s logs", awsInstanceStartupDone, vm.name)
				break outer
			}
			framework.Logf("Could not find string %s in VM %s console logs", awsInstanceStartupDone, vm.name)
			if counter%10 == 0 {
				framework.Logf("Console logs are:\n%s", string(consoleOut))
			}
		}
	}
	return instance, nil
}

// getImageID gets the latest image (by creation date) that matches string awsImageName.
func (a *awsCloudClient) getImageID() (string, error) {
	imageName := fmt.Sprintf("%s*", awsImageName)
	describeImagesInput := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{Name: aws.String("name"), Values: []*string{&imageName}},
			{Name: aws.String("owner-alias"), Values: []*string{aws.String("amazon")}},
		}}
	describeImagesOutput, err := a.client.DescribeImages(describeImagesInput)
	if err != nil {
		return "", fmt.Errorf("could not find %s image, err: %q", awsImageName, err)
	}
	images := describeImagesOutput.Images
	if len(images) == 0 {
		return "", fmt.Errorf("could not find any valid cloud images for %s", awsImageName)
	}
	sort.Slice(images, func(i, j int) bool {
		return *images[i].CreationDate > *images[j].CreationDate
	})

	return *images[0].ImageId, nil
}

// createSecurityGroup creates an AWS security group for the VM.
func (a *awsCloudClient) createSecurityGroup(vm *vm, requestPublicIP bool) (string, error) {
	framework.Logf("Creating security group for VM %s", vm.name)
	secGroupInput := &ec2.CreateSecurityGroupInput{
		Description: &vm.name,
		GroupName:   &vm.name,
		VpcId:       a.instance.VpcId,
	}
	secGroupOutput, err := a.client.CreateSecurityGroup(secGroupInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "InvalidVpcID.NotFound":
				return "", fmt.Errorf("unable to find VPC with ID %q", *secGroupInput.VpcId)
			case "InvalidGroup.Duplicate":
				return "", fmt.Errorf("security group %q already exists", *secGroupInput.GroupName)
			}
		}
		return "", fmt.Errorf("could not create security group %s, err: %q", vm.name, err)
	}

	framework.Logf("Creating security group ingress for security group %s for vm %s", *secGroupOutput.GroupId, vm.name)
	var perms []*ec2.IpPermission
	for portName, pp := range vm.ports {
		sourceRange := awsCustomSecGroupSourceRange
		// In the case of SSH, we open this globally to allow debugging.
		if portName == sshPort && requestPublicIP {
			sourceRange = "0.0.0.0/0"
		}
		perms = append(perms, &ec2.IpPermission{
			FromPort:   aws.Int64(int64(pp.port)),
			IpProtocol: aws.String(strings.ToLower(pp.protocol)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(sourceRange)}},
			ToPort:     aws.Int64(int64(pp.port)),
		})
	}
	_, err = a.client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       secGroupOutput.GroupId,
		IpPermissions: perms,
	})
	if err != nil {
		return "", fmt.Errorf("could not authorize security group ingress for security group %s (VM %s), err: %q",
			*secGroupOutput.GroupId, vm.name, err)
	}
	return *secGroupOutput.GroupId, nil
}

// awsInitCredentials initializes a session to AWS with the provided keys and region.
func awsInitCredentials(ctx context.Context, accessKeyID, secretAccessKey, region string) (*ec2.EC2, error) {
	creds := credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	sessionOpts := session.Options{
		Config: aws.Config{
			Region:                        &region,
			CredentialsChainVerboseErrors: pointer.Bool(true),
			Credentials:                   creds,
		},
	}
	mySession, err := session.NewSessionWithOptions(sessionOpts)
	if err != nil {
		return nil, fmt.Errorf("could not initialize AWS session: %w", err)
	}
	return ec2.New(mySession), nil
}

// parseAWSDeleteError returns nil if the provided err is of type not found. Otherwise, it returns err as is.
func parseAWSDeleteError(err error) error {
	if ae, ok := err.(awserr.Error); ok {
		if ae.Code() == "NotFound" {
			return nil
		}
	}
	return err
}
