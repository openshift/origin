package util

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/iam"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// AWSInstanceNotFound custom error for not found instances
type AWSInstanceNotFound struct{ InstanceName string }

// Error implements the error interface
func (nfe *AWSInstanceNotFound) Error() string {
	return fmt.Sprintf("No instance found in current cluster with name %s", nfe.InstanceName)
}

// GetAwsCredentialFromCluster get aws credential from cluster
func GetAwsCredentialFromCluster(oc *CLI) {
	credential, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/aws-creds", "-n", "kube-system", "-o", "json").Output()
	// Skip for sts and c2s clusters.
	if err != nil {
		g.Skip("Did not get credential to access aws, skip the testing.")
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyIDBase64, secureKeyBase64 := gjson.Get(credential, `data.aws_access_key_id`).String(), gjson.Get(credential, `data.aws_secret_access_key`).String()
	accessKeyID, err1 := base64.StdEncoding.DecodeString(accessKeyIDBase64)
	o.Expect(err1).NotTo(o.HaveOccurred())
	secureKey, err2 := base64.StdEncoding.DecodeString(secureKeyBase64)
	o.Expect(err2).NotTo(o.HaveOccurred())
	clusterRegion, err3 := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	os.Setenv("AWS_ACCESS_KEY_ID", string(accessKeyID))
	os.Setenv("AWS_SECRET_ACCESS_KEY", string(secureKey))
	os.Setenv("AWS_REGION", clusterRegion)
}

// AwsClient struct
type AwsClient struct {
	svc *ec2.EC2
}

// InitAwsSession init session
func InitAwsSession(region string) *session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	return sess
}

// IAMClient struct for IAM operations
type IAMClient struct {
	svc *iam.IAM
}

// NewIAMClient constructor to create IAM client with default credential and config
// Should use GetAwsCredentialFromCluster(oc) to set ENV first before using it
func NewIAMClient(sess *session.Session) *IAMClient {
	return &IAMClient{
		svc: iam.New(
			sess,
			aws.NewConfig(),
		),
	}
}

// Attach role policy
func (iamClient *IAMClient) AttachRolePolicy(roleName, policyArn string) error {
	_, err := iamClient.svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyArn),
	})

	if err != nil {
		e2e.Logf("Failed to AttachRolePolicy for roleName: %s policyArn %s error %s", roleName, policyArn, err.Error())
	}

	return err
}

// Detach role policy
func (iamClient *IAMClient) DetachRolePolicy(roleName, policyArn string) error {
	_, err := iamClient.svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyArn),
	})

	if err != nil {
		e2e.Logf("Failed to DetachRolePolicy for roleName: %s policyArn %s error %s", roleName, policyArn, err.Error())
	}

	return err
}

// getIAMUserName get IAM user name
func (iamClient *IAMClient) GetIAMUserName() (string, error) {
	result, err := iamClient.svc.GetUser(nil)
	if err != nil {
		return "", err
	}

	return *result.User.UserName, nil
}

// AttachPolicyToUser attach policy to IAM user
func (iamClient *IAMClient) AttachUserPolicy(userName, policyArn string) error {
	_, err := iamClient.svc.AttachUserPolicy(&iam.AttachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyArn),
	})

	if err != nil {
		e2e.Logf("Failed to AttachPolicyToUser for userName: %s policyArn %s error %s", userName, policyArn, err.Error())
	}
	e2e.Logf("Attached policy to user")
	return err
}

// DetachPolicyFromUser detach user policy
func (iamClient *IAMClient) DetachUserPolicy(userName, policyArn string) error {
	_, err := iamClient.svc.DetachUserPolicy(&iam.DetachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyArn),
	})

	if err != nil {
		e2e.Logf("Failed to DetachPolicyToUser for userName: %s policyArn %s error %s", userName, policyArn, err.Error())
	}

	return err
}

// ECRClient
type ECRClient struct {
	svc *ecr.ECR
}

// NewECRClient creates an ECRClient
func NewECRClient(sess *session.Session) *ECRClient {
	return &ECRClient{
		svc: ecr.New(sess),
	}
}

// CreateContainerRepository create a container repository
func (ecrClient *ECRClient) CreateContainerRepository(repositoryName string) (string, error) {
	createRes, err := ecrClient.svc.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repositoryName),
	})
	if err != nil {
		e2e.Logf("Error creating repository %s", err.Error())
		return "", err
	}
	e2e.Logf("Repository created:", *createRes.Repository.RepositoryUri)
	return *createRes.Repository.RepositoryUri, nil
}

// DeleteContainerRepository delete container repository
func (ecrClient *ECRClient) DeleteContainerRepository(repositoryName string) error {
	_, err := ecrClient.svc.DeleteRepository(&ecr.DeleteRepositoryInput{
		RepositoryName: aws.String(repositoryName),
		Force:          aws.Bool(true),
	})
	return err
}

// GetAuthorizationToken get container repository credential
func (ecrClient *ECRClient) GetAuthorizationToken() (string, error) {
	loginRes, err := ecrClient.svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		e2e.Logf("Error getting authorization token: %s", err.Error())
		return "", err
	}
	authData := loginRes.AuthorizationData[0]
	password := aws.StringValue(authData.AuthorizationToken)
	return password, nil
}
