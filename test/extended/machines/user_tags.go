package operators

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsutil "github.com/openshift/origin/test/extended/util/aws"
	"github.com/stretchr/objx"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:AWSTags] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have user specified resource tags on machines", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		obj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(obj.UnstructuredContent()))

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		ec2client := ec2.NewFromConfig(awscfg)

		// list all machines
		items, err := listMachines(dc, "")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verifying all resource tags in infrastructure are propogated to machine tags and aws tags")
		verifyEC2Tags(ec2client, awsutil.FetchResourceTags(obj.UnstructuredContent()), items)
	})

	g.It("update the machine tags when infrastructure is updated", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(infraobj.UnstructuredContent()))

		updatedTag := v1.AWSResourceTag{Key: "ec2-machines", Value: "verified"}
		updatedInfra := awsutil.UpdateResourceTags(cfgClient, infraobj, updatedTag)

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		ec2client := ec2.NewFromConfig(awscfg)

		g.By("verifying all resource tags in infrastructure are propogated to tags on all machines and aws tags")
		// list all machines
		items, err := listMachines(dc, "")
		o.Expect(err).NotTo(o.HaveOccurred())

		resourceTags := awsutil.FetchResourceTags(updatedInfra.UnstructuredContent())
		o.Expect(resourceTags).Should(o.HaveKeyWithValue(updatedTag.Key, updatedTag.Value))

		verifyEC2Tags(ec2client, resourceTags, items)
	})
})

func verifyEC2Tags(client *ec2.Client, resourceTags objx.Map, machines []objx.Map) {
	for _, machine := range machines {
		tags := objects(machine.Get("spec.providerSpec.value.tags"))
		machineName := machine.Get("metadata.name").String()

		machineTags := make(map[string]string)
		for _, i := range tags {
			machineTags[i["name"].(string)] = i["value"].(string)
		}

		awsutil.VerifyResourceTags(resourceTags, machineTags)

		tagList := fetchAWSTags(client, machineName)
		for k, v := range resourceTags {
			o.Expect(tagList).Should(o.HaveKeyWithValue(k, v))
		}
	}
}

func fetchAWSTags(ec2client *ec2.Client, name string) map[string]string {
	op, err := ec2client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:Name"), Values: []string{name}},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	tagList := make(map[string]string)
	for idx := range op.Reservations {
		for _, inst := range op.Reservations[idx].Instances {
			for _, tag := range inst.Tags {
				tagList[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}
		}
	}
	return tagList
}
