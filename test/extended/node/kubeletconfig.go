package node

import (
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
)

var _ = g.Describe("[sig-node][NodeQE] NODE kubeletconfig feature", func() {
	defer g.GinkgoRecover()
	var oc = compat_otp.NewCLI("node-"+getRandomString(), compat_otp.KubeConfigPath())

	// author: minmli@redhat.com
	g.It("NonHyperShiftHOST-Author:minmli-Medium-39142-kubeletconfig should not prompt duplicate error message", func() {
		buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
		kubeletConfigT := filepath.Join(buildPruningBaseDir, "kubeletconfig-maxpod.yaml")
		g.By("Test for case OCP-39142")

		labelKey := "custom-kubelet-" + getRandomString()
		labelValue := "maxpods-" + getRandomString()

		kubeletcfg39142 := kubeletCfgMaxpods{
			name:       "custom-kubelet-39142",
			labelkey:   labelKey,
			labelvalue: labelValue,
			maxpods:    239,
			template:   kubeletConfigT,
		}

		g.By("Create a kubeletconfig without matching machineConfigPool label")
		kubeletcfg39142.createKubeletConfigMaxpods(oc)
		defer kubeletcfg39142.deleteKubeletConfigMaxpods(oc)

		g.By("Check kubeletconfig should not prompt duplicate error message")
		keyword := "Error: could not find any MachineConfigPool set for KubeletConfig"
		err := kubeletNotPromptDupErr(oc, keyword, kubeletcfg39142.name)
		compat_otp.AssertWaitPollNoErr(err, "kubeletconfig prompt duplicate error message")
	})
})
