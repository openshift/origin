package cli

import (
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cli] oc version", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-version")
	ocAdmin := oc.AsAdmin()

	g.It("check kubernetes version matches the io.openshift.build.versions label", func() {
		imageName, err := ocAdmin.Run("get").Args("pods", "-l", "app=kube-controller-manager", "-n", "openshift-kube-controller-manager", "-o=jsonpath={..spec.containers[0].image}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		masterNode, err := ocAdmin.WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		imageInfo, err := oc.AsAdmin().Run("debug").Args("node/"+masterName[0], "--", "chroot", "/host", "oc", "image", "info", "--registry-config=/var/lib/kubelet/config.json", strings.Fields(imageName)[0]).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		re1 := regexp.MustCompile(`io.openshift.build.versions=kubernetes=[1-9]{1}.[1-9]{1,2}.[1-9]{1,2}`)
		kversion := re1.FindAllString(imageInfo, -1)
		if len(kversion) == 0 {
			e2e.Failf("Failed to find matches for io.openshift.build.versions=kubernetes in kversion %s", kversion)
		}
		kversion = strings.Split(kversion[0], "=")
		kversionNum := kversion[len(kversion)-1]
		version, err := ocAdmin.Run("version").Output()
		o.Expect(version).To(o.ContainSubstring(kversionNum))
		if err != nil {
			e2e.Failf("Failed to match output from oc version %v to io.openshift.build.versions label %v", version, kversionNum)
		}
		re := regexp.MustCompile(`Kubernetes Version: v[1-9]{1}.[1-9]{1,2}.[1-9]{1,2}`)
		if re == nil {
			e2e.Failf("Failed to match regex, please check kubernetes version to see if it is one release candidate")
		}
		result := re.FindAllString(version, -1)
		if len(result) == 0 {
			e2e.Failf("Failed to find match for kubernetes version in result %s", result)
		}
		if match, _ := regexp.MatchString(kversionNum, result[0]); !match {
			e2e.Failf("Failed to match Kuberntes version %v & oc Version %v", kversionNum, result[0])
		}

		kubeletVersion, err := ocAdmin.WithoutNamespace().Run("get").Args("node", "-o", "custom-columns=VERSION:.status.nodeInfo.kubeletVersion").Output()
		nodekubeletVersion := strings.Fields(kubeletVersion)

		for _, value := range nodekubeletVersion[1:] {
			if err != nil {
				e2e.Failf("Fail to get the kubelet version")
			}
			re := regexp.MustCompile(`v[1-9]{1}.[1-9]{1,2}.[1-9]{1,2}`)
			if re == nil {
				e2e.Failf("kubeletVersion regexp err!")
			}
			kubeletResult := re.FindAllString(value, -1)
			if len(kubeletResult) == 0 {
				e2e.Failf("Failed to match regex, please check kubeletversion of the nodes to see if it is not one release candidate")
			}
			if match, _ := regexp.MatchString(kversionNum, kubeletResult[0]); !match {
				e2e.Failf("kubernetes version %v and kubelet version %v in oc get nodes ouput differ", kversionNum, kubeletResult)
			}
			o.Expect(value).To(o.ContainSubstring(kversionNum))
		}

	})
})
