package storage

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	awsutil "github.com/openshift/origin/test/extended/util/aws"
	"github.com/stretchr/objx"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-storage][Feature:AWSTags][Serial] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have user specified resource tags on all the EBS volumes created using EBS Provisioner", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(infraobj.UnstructuredContent()))

		scClient := dc.Resource(schema.GroupVersionResource{Group: "storage.k8s.io", Resource: "storageclasses", Version: "v1"})
		scList, err := scClient.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to check for storageclasses: %v", err)
		}
		o.Expect(scList.Items).ShouldNot(o.BeEmpty())

		g.By("verifying storage class with EBS provisioner")
		var scname string
		for _, sc := range scList.Items {
			obj := objx.Map(sc.UnstructuredContent())
			e2e.Logf("%v", obj)
			if strings.EqualFold(obj.Get("provisioner").MustStr(), "ebs.csi.aws.com") {
				scname = obj.Get("metadata.name").MustStr()
				break
			}
		}
		if len(scname) == 0 {
			e2e.Failf("storageclass with ebs provisioner is missing")
		}

		g.By("creating a pvc and verifying volume tags")
		ns, err := c.CoreV1().Namespaces().
			Create(context.Background(), &k8sv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ebs-csi"},
			}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pvc := createPVC(c, "app-1-pvc", ns.Name, scname)

		createPodWithPVC(c, "app-1", ns.Name, pvc.Name)

		var vol string
		g.By("Waiting for PVC to be bound to Volume")
		_ = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
			obj, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.Background(), pvc.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			vol = obj.Spec.VolumeName
			return obj.Status.Phase == k8sv1.ClaimBound, nil
		})

		pv, err := c.CoreV1().PersistentVolumes().Get(context.Background(), vol, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		zone := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		ec2client := ec2.NewFromConfig(awscfg)
		awsutil.VerifyResourceTags(
			awsutil.FetchResourceTags(objx.Map(infraobj.UnstructuredContent())),
			fetchAWSTagsForVolume(ec2client, pv.Spec.CSI.VolumeHandle, zone))
	})

	g.It("be able to handle updates of user specified resource tags on all the EBS volumes created using EBS Provisioner", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking if platform type is AWS in infrastructures config")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to check for infrastructures: %v", err)
		}
		awsutil.SkipUnlessPlatformAWS(objx.Map(infraobj.UnstructuredContent()))

		updatedInfra := awsutil.UpdateResourceTags(cfgClient, infraobj, v1.AWSResourceTag{Key: "ebs-csi-driver", Value: "verified"})

		scClient := dc.Resource(schema.GroupVersionResource{Group: "storage.k8s.io", Resource: "storageclasses", Version: "v1"})
		scList, err := scClient.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to check for storageclasses: %v", err)
		}
		o.Expect(scList.Items).ShouldNot(o.BeEmpty())

		g.By("verifying storage class with EBS provisioner")
		var scname string
		for _, sc := range scList.Items {
			obj := objx.Map(sc.UnstructuredContent())
			e2e.Logf("%v", obj)
			if strings.EqualFold(obj.Get("provisioner").MustStr(), "ebs.csi.aws.com") {
				scname = obj.Get("metadata.name").MustStr()
				break
			}
		}
		if len(scname) == 0 {
			e2e.Failf("storageclass with ebs provisioner is missing")
		}

		g.By("creating a pvc and verifying volume tags")
		ns, err := c.CoreV1().Namespaces().
			Create(context.Background(), &k8sv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ebs-csi-2"},
			}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pvc := createPVC(c, "app-2-pvc", ns.Name, scname)

		createPodWithPVC(c, "app-2", ns.Name, pvc.Name)

		var vol string
		g.By("Waiting for PVC to be bound to Volume")
		_ = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
			obj, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.Background(), pvc.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			vol = obj.Spec.VolumeName
			return obj.Status.Phase == k8sv1.ClaimBound, nil
		})

		pv, err := c.CoreV1().PersistentVolumes().Get(context.Background(), vol, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		zone := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]

		awscfg, err := awsutil.GetClient(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		ec2client := ec2.NewFromConfig(awscfg)
		awsutil.VerifyResourceTags(
			awsutil.FetchResourceTags(objx.Map(updatedInfra.UnstructuredContent())),
			fetchAWSTagsForVolume(ec2client, pv.Spec.CSI.VolumeHandle, zone))
	})
})

func createPVC(c *clientset.Clientset, name, namespace, scname string) *k8sv1.PersistentVolumeClaim {
	block := k8sv1.PersistentVolumeBlock
	pvcTmpl := &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: k8sv1.PersistentVolumeClaimSpec{
			AccessModes: []k8sv1.PersistentVolumeAccessMode{
				k8sv1.ReadWriteOnce,
			},
			StorageClassName: &scname,
			VolumeMode:       &block,
			Resources: k8sv1.ResourceRequirements{
				Requests: k8sv1.ResourceList{
					k8sv1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	pvc, err := c.CoreV1().PersistentVolumeClaims(namespace).
		Create(context.Background(), pvcTmpl, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("PVC %v", pvc)
	return pvc
}

func createPodWithPVC(c *clientset.Clientset, name, namespace, pvcname string) {
	pTmpl := &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: namespace,
		},
		Spec: k8sv1.PodSpec{
			Containers: []k8sv1.Container{
				{
					Name:    "app",
					Image:   "busybox",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"tail -f /dev/null"},
					VolumeDevices: []k8sv1.VolumeDevice{
						{
							Name:       "data",
							DevicePath: "/dev/xvda",
						},
					},
				},
			},
			Volumes: []k8sv1.Volume{
				{
					Name: "data",
					VolumeSource: k8sv1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcname,
						},
					},
				},
			},
		},
	}

	_, err := c.CoreV1().Pods(namespace).Create(context.Background(), pTmpl, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func fetchAWSTagsForVolume(ec2client *ec2.Client, volID, zone string) map[string]string {
	vol, err := ec2client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
		VolumeIds: []string{volID},
		Filters: []types.Filter{
			{
				Name:   aws.String("availability-zone"),
				Values: []string{zone},
			},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(vol.Volumes).Should(o.HaveLen(1))

	tagList := make(map[string]string)
	for _, tag := range vol.Volumes[0].Tags {
		tagList[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tagList
}
