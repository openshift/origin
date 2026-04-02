package util

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
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

// InitAwsConfig init AWS config (AWS SDK v2)
func InitAwsConfig(region string) aws.Config {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return cfg
}

type ELBClient struct {
	svc *elasticloadbalancing.Client
}

// NewELBClient creates an ELBClient
func NewELBClient(cfg aws.Config) *ELBClient {
	return &ELBClient{
		svc: elasticloadbalancing.NewFromConfig(cfg),
	}
}

// GetLBHealthCheckPortPath get load balance health check port and path
func (elbClient *ELBClient) GetLBHealthCheckPortPath(lbName string) (string, error) {
	input := &elasticloadbalancing.DescribeLoadBalancersInput{
		LoadBalancerNames: []string{
			lbName,
		},
	}

	result, err := elbClient.svc.DescribeLoadBalancers(context.TODO(), input)
	if err != nil {
		e2e.Logf("Failed to describe load balancer: %v", err)
		return "", err
	}

	if len(result.LoadBalancerDescriptions) == 0 {
		e2e.Logf("Failed to get load balancers: %v", err)
	}

	healthCheck := result.LoadBalancerDescriptions[0].HealthCheck
	if healthCheck == nil {
		e2e.Logf("Failed to get health check: %v", err)
	}
	return *healthCheck.Target, nil
}
