package images

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v1 "github.com/openshift/api/config/v1"
	awsutil "github.com/openshift/origin/test/extended/util/aws"
	"github.com/stretchr/objx"
)

var _ = g.Describe("[sig-imageregistry][Feature:AWSTags] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have user specified resource tags on the image registry's storage backend (s3 bucket)", func() {
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

		depClient := dc.Resource(schema.GroupVersionResource{Group: "apps", Resource: "deployments", Version: "v1"}).Namespace("openshift-image-registry")
		regObj, err := depClient.Get(context.Background(), "image-registry", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for image registry: %v", err)
		}
		var dep k8sv1.Deployment
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(regObj.UnstructuredContent(), &dep)
		o.Expect(err).NotTo(o.HaveOccurred())

		var bucketID string
		for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
			if e.Name == "REGISTRY_STORAGE_S3_BUCKET" {
				bucketID = e.Value
				break
			}
		}

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		s3client := s3.NewFromConfig(awscfg)

		awsutil.VerifyResourceTags(awsutil.FetchResourceTags(objx.Map(infraobj.UnstructuredContent())), fetchAWSTagsForS3(s3client, bucketID))
	})

	g.It("be able to handle updates of user specified resource tags on the image registry's storage backend (s3 bucket)", func() {
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

		updatedInfra := awsutil.UpdateResourceTags(cfgClient, infraobj, v1.AWSResourceTag{Key: "image-registry", Value: "verified"})

		depClient := dc.Resource(schema.GroupVersionResource{Group: "apps", Resource: "deployments", Version: "v1"}).Namespace("openshift-image-registry")
		regObj, err := depClient.Get(context.Background(), "image-registry", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for image registry: %v", err)
		}

		var dep k8sv1.Deployment
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(regObj.UnstructuredContent(), &dep)
		o.Expect(err).NotTo(o.HaveOccurred())

		var bucketID string
		for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
			if e.Name == "REGISTRY_STORAGE_S3_BUCKET" {
				bucketID = e.Value
				break
			}
		}

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		s3client := s3.NewFromConfig(awscfg)

		awsutil.VerifyResourceTags(awsutil.FetchResourceTags(objx.Map(updatedInfra.UnstructuredContent())), fetchAWSTagsForS3(s3client, bucketID))
	})
})

func fetchAWSTagsForS3(s3client *s3.Client, bucketID string) map[string]string {
	tags, err := s3client.GetBucketTagging(context.Background(), &s3.GetBucketTaggingInput{
		Bucket: aws.String(bucketID),
	})
	if err != nil {
		e2e.Failf("unable to fetch tags from aws for s3 resource: %v", err)
	}

	tagList := make(map[string]string)
	for _, tag := range tags.TagSet {
		tagList[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tagList
}
