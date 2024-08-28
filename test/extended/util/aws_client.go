package util

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
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

// InitAwsSession init session
func InitAwsSession(region string) *session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	return sess
}

type IAMClient struct {
	svc *iam.IAM
}

type STSClient struct {
	*sts.STS
}

type ECRClient struct {
	svc *ecr.ECR
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

// NewDelegatingStsClient creates an StsClient which delegates calls to methods that are not implemented by itself
// to the wrapped sts.STS client.
func NewDelegatingStsClient(wrappedClient *sts.STS) *STSClient {
	return &STSClient{
		STS: wrappedClient,
	}
}

// NewECRClient creates an ECRClient
func NewECRClient(sess *session.Session) *ECRClient {
	return &ECRClient{
		svc: ecr.New(sess),
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

// CreateContainerRepository create a container repository
func (ecrClient *ECRClient) CreateContainerRepository(repositoryName string) (string, error) {
	createRes, err := ecrClient.svc.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repositoryName),
	})
	if err != nil {
		e2e.Logf("Error creating repository %s", err.Error())
		return "", err
	}
	e2e.Logf("Repository created: %s", *createRes.Repository.RepositoryUri)
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

// CheckPermission checks if the current user has the permission to create an ECR repository
func CheckECRPermission(iamClient *IAMClient, stsClient *STSClient) (bool, error) {
	callerIdentity, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return false, err
	}
	userArn := callerIdentity.Arn
	simulateInput := &iam.SimulatePrincipalPolicyInput{
		PolicySourceArn: userArn,
		ActionNames:     []*string{aws.String("ecr:CreateRepository")},
	}
	simulateResult, err := iamClient.svc.SimulatePrincipalPolicy(simulateInput)
	if err != nil {
		return false, err
	}

	for _, result := range simulateResult.EvaluationResults {
		if *result.EvalActionName == "ecr:CreateRepository" {
			return *result.EvalDecision == string(types.PolicyEvaluationDecisionTypeAllowed), nil
		}
	}
	e2e.Logf("This account doesn't have CreateRepository permission")
	return false, nil
}
