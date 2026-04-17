package util

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	awsv1 "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

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
		Config: awsv1.Config{
			Region: awsv1.String(region),
		},
	}))

	return sess
}

type ELBClient struct {
	svc   *elb.Client
	svcV2 *elbv2.Client
}

// InitAwsConfig init AWS config (AWS SDK v2)
func InitAwsConfig(region string) aws.Config {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return cfg
}

// NewELBClient creates an ELBClient
func NewELBClient(cfg aws.Config) *ELBClient {
	return &ELBClient{
		svc:   elb.NewFromConfig(cfg),
		svcV2: elbv2.NewFromConfig(cfg),
	}
}

// GetLBHealthCheckPortPath get load balance health check port and path for Classic Load Balancer
func (elbClient *ELBClient) GetCLBHealthCheckPortPath(lbName string) (string, error) {
	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []string{lbName},
	}

	result, err := elbClient.svc.DescribeLoadBalancers(context.TODO(), input)
	if err != nil {
		e2e.Logf("Failed to describe classic load balancer: %v", err)
		return "", err
	}

	if len(result.LoadBalancerDescriptions) == 0 {
		e2e.Logf("Failed to get classic load balancers: %v", err)
	}

	healthCheck := result.LoadBalancerDescriptions[0].HealthCheck
	if healthCheck == nil {
		e2e.Logf("Failed to get health check: %v", err)
	}
	return *healthCheck.Target, nil
}

// GetNLBHealthCheckPortPath get load balance health check port and path for Network/Application Load Balancer
func (elbClient *ELBClient) GetNLBHealthCheckPortPath(lbName string) (string, error) {
	// Describe the load balancer
	input := &elbv2.DescribeLoadBalancersInput{
		Names: []string{lbName},
	}
	result, err := elbClient.svcV2.DescribeLoadBalancers(context.TODO(), input)
	if err != nil {
		e2e.Logf("Failed to describe NLB: %v", err)
		return "", err
	}

	if len(result.LoadBalancers) == 0 {
		e2e.Logf("No NLB found with name: %s", lbName)
		return "", nil
	}

	// Get target groups for this load balancer
	tgInput := &elbv2.DescribeTargetGroupsInput{
		LoadBalancerArn: result.LoadBalancers[0].LoadBalancerArn,
	}
	tgResult, err := elbClient.svcV2.DescribeTargetGroups(context.TODO(), tgInput)
	if err != nil {
		e2e.Logf("Failed to describe target groups: %v", err)
		return "", err
	}

	if len(tgResult.TargetGroups) == 0 {
		e2e.Logf("No target groups found for NLB: %s", lbName)
		return "", nil
	}

	// Get health check configuration from the first target group
	tg := tgResult.TargetGroups[0]
	protocol := string(tg.HealthCheckProtocol)
	path := aws.ToString(tg.HealthCheckPath)
	port := aws.ToString(tg.HealthCheckPort)

	// Format: "HTTP:10256/healthz"
	return fmt.Sprintf("%s:%s%s", protocol, port, path), nil
}
