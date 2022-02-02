package cloud_credentials

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	v1 "github.com/openshift/api/config/v1"
	awsutil "github.com/openshift/origin/test/extended/util/aws"
	"github.com/stretchr/objx"
)

var _ = g.Describe("[sig-cloud-credentials][Feature:AWSTags] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have user specified resource tags on all the users created using cloud credential requests", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(infraobj.UnstructuredContent()))

		client := dc.Resource(schema.GroupVersionResource{Group: "cloudcredential.openshift.io", Resource: "credentialsrequests", Version: "v1"}).Namespace("openshift-cloud-credential-operator")
		credreqs, err := client.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list credential requests: %v", err)
		}

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		iamClient := iam.NewFromConfig(awscfg)
		for _, item := range credreqs.Items {
			cr := objx.Map(item.UnstructuredContent())

			provisioned := cr.Get("status.provisioned").MustBool()
			if !provisioned {
				continue
			}
			user := cr.Get("status.providerStatus.user").MustStr()
			awsutil.VerifyResourceTags(awsutil.FetchResourceTags(objx.Map(infraobj.UnstructuredContent())), fetchAWSTagsForUser(iamClient, user))
		}
	})

	g.It("be able to handle updates of user specified resource tags on all the users created using cloud credential requests", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(infraobj.UnstructuredContent()))

		updatedInfra := awsutil.UpdateResourceTags(cfgClient, infraobj, v1.AWSResourceTag{Key: "cloud-credentials", Value: "verified"})

		client := dc.Resource(schema.GroupVersionResource{Group: "cloudcredential.openshift.io", Resource: "credentialsrequests", Version: "v1"}).Namespace("openshift-cloud-credential-operator")
		credreqs, err := client.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list credential requests: %v", err)
		}

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		iamClient := iam.NewFromConfig(awscfg)
		for _, item := range credreqs.Items {
			cr := objx.Map(item.UnstructuredContent())

			provisioned := cr.Get("status.provisioned").MustBool()
			if !provisioned {
				continue
			}
			user := cr.Get("status.providerStatus.user").MustStr()
			awsutil.VerifyResourceTags(awsutil.FetchResourceTags(objx.Map(updatedInfra.UnstructuredContent())), fetchAWSTagsForUser(iamClient, user))
		}
	})
})

func fetchAWSTagsForUser(iamClient *iam.Client, username string) map[string]string {
	u, err := iamClient.GetUser(context.Background(), &iam.GetUserInput{UserName: aws.String(username)})
	if err != nil {
		e2e.Failf("unable to fetch user from aws %v", err)
	}

	tagList := make(map[string]string)
	for _, tag := range u.User.Tags {
		tagList[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tagList
}
